package sio

import (
	"fmt"
	"reflect"
	"sync"
	"time"

	mapset "github.com/deckarep/golang-set/v2"
	eio "github.com/tomruk/socket.io-go/engine.io"
	"github.com/tomruk/socket.io-go/parser"
)

type serverSocket struct {
	id        SocketID
	pid       PrivateSessionID
	recovered bool

	connected   bool
	connectedMu sync.RWMutex

	server  *Server
	conn    *serverConn
	nsp     *Namespace
	adapter Adapter

	parser parser.Parser

	acks   map[uint64]*ackHandler
	acksMu sync.Mutex

	middlewareFuncs   []reflect.Value
	middlewareFuncsMu sync.RWMutex

	join   func(room ...Room)
	joinMu sync.Mutex

	closeOnce sync.Once
	debug     DebugFunc

	eventHandlers         *eventHandlerStore
	errorHandlers         *handlerStore[*ServerSocketErrorFunc]
	disconnectingHandlers *handlerStore[*ServerSocketDisconnectingFunc]
	disconnectHandlers    *handlerStore[*ServerSocketDisconnectFunc]
}

// previousSession can be nil
func newServerSocket(server *Server, c *serverConn, nsp *Namespace, parser parser.Parser, previousSession *SessionToPersist) (*serverSocket, error) {
	adapter := nsp.Adapter()
	s := &serverSocket{
		server:  server,
		conn:    c,
		nsp:     nsp,
		adapter: adapter,
		parser:  parser,
		debug:   server.debug,

		eventHandlers:         newEventHandlerStore(),
		errorHandlers:         newHandlerStore[*ServerSocketErrorFunc](),
		disconnectingHandlers: newHandlerStore[*ServerSocketDisconnectingFunc](),
		disconnectHandlers:    newHandlerStore[*ServerSocketDisconnectFunc](),
	}

	s.join = func(room ...Room) {
		s.debug("Joining room(s)", room)
		adapter.AddAll(s.ID(), room)
	}

	if previousSession != nil {
		s.id = previousSession.SID
		s.pid = previousSession.PID
		s.recovered = true
		s.Join(previousSession.Rooms...)
		for _, missedPacket := range previousSession.MissedPackets {
			buffers, err := s.parser.Encode(missedPacket.Header, missedPacket.Data)
			if err != nil {
				return nil, err
			}
			s.conn.sendBuffers(buffers...)
		}
	} else {
		id, err := eio.GenerateBase64ID(eio.Base64IDSize)
		if err != nil {
			return nil, err
		}
		s.id = SocketID(id)
		s.recovered = false

		if server.connectionStateRecovery.Enabled {
			id, err := eio.GenerateBase64ID(eio.Base64IDSize)
			if err != nil {
				return nil, err
			}
			s.pid = PrivateSessionID(id)
		}
	}
	return s, nil
}

func (s *serverSocket) Server() *Server { return s.server }

func (s *serverSocket) Namespace() *Namespace { return s.nsp }

func (s *serverSocket) Recovered() bool { return s.recovered }

func (s *serverSocket) Connected() bool {
	s.connectedMu.RLock()
	defer s.connectedMu.RUnlock()
	return s.connected
}

var _emptyError error
var reflectError = reflect.TypeOf(&_emptyError).Elem()

func (s *serverSocket) Use(f any) {
	s.middlewareFuncsMu.Lock()
	defer s.middlewareFuncsMu.Unlock()
	rv := reflect.ValueOf(f)
	if rv.Kind() != reflect.Func {
		panic("sio: function expected")
	}
	rt := rv.Type()
	if rt.NumIn() != 2 {
		panic("sio: function signature: func(eventName string, v ...any) error")
	}
	if rt.In(0).Kind() != reflect.String {
		panic("sio: function signature: func(eventName string, v ...any) error")
	}
	if rt.In(1).Kind() != reflect.Slice || rt.In(1).Elem().Kind() != reflect.Interface {
		panic("sio: function signature: func(eventName string, v ...any) error")
	}
	if rt.NumOut() != 1 {
		panic("sio: function signature: func(eventName string, v ...any) error")
	}
	if rt.Out(0).Kind() != reflect.Interface || !rt.Out(0).Implements(reflectError) {
		panic("sio: function signature: func(eventName string, v ...any) error")
	}
	s.middlewareFuncs = append(s.middlewareFuncs, rv)
}

func (s *serverSocket) onPacket(header *parser.PacketHeader, eventName string, decode parser.Decode) error {
	switch header.Type {
	case parser.PacketTypeEvent, parser.PacketTypeBinaryEvent:
		handlers := s.eventHandlers.GetAll(eventName)

		go func() {
			ackSent := false
			ackSentMu := new(sync.Mutex)

			sendAck := func(id uint64, ret []reflect.Value) {
				ackSentMu.Lock()
				if ackSent {
					ackSentMu.Unlock()
					return
				}
				ackSent = true
				ackSentMu.Unlock()
				s.debug("Sending ack with ID", id)
				s.sendAckPacket(id, ret)
			}

			for _, handler := range handlers {
				s.onEvent(handler, header, decode, sendAck)
			}
		}()
	case parser.PacketTypeAck, parser.PacketTypeBinaryAck:
		go s.onAck(header, decode)

	case parser.PacketTypeDisconnect:
		s.onDisconnect()
	default:
		return wrapInternalError(fmt.Errorf("invalid packet type: %d", header.Type))
	}

	return nil
}

func (s *serverSocket) onDisconnect() {
	s.debug("Got disconnect packet")
	s.onClose("client namespace disconnect")
}

func (s *serverSocket) onEvent(handler *eventHandler, header *parser.PacketHeader, decode parser.Decode, sendAck sendAckFunc) {
	values, err := decode(handler.inputArgs...)
	if err != nil {
		s.onError(wrapInternalError(err))
		return
	}

	if len(values) == len(handler.inputArgs) {
		for i, v := range values {
			if handler.inputArgs[i].Kind() != reflect.Ptr && v.Kind() == reflect.Ptr {
				values[i] = v.Elem()
			}
		}
	} else {
		s.onError(fmt.Errorf("sio: onEvent: invalid number of arguments"))
		return
	}

	err = s.callMiddlewares(values)
	if err != nil {
		s.onError(err)
		return
	}

	if !s.Connected() {
		s.debug("ignore packet received after disconnection")
		return
	}

	ret, err := handler.Call(values...)
	if err != nil {
		s.onError(wrapInternalError(err))
		return
	}

	if header.ID != nil {
		sendAck(*header.ID, ret)
	}
}

func (s *serverSocket) callMiddlewares(values []reflect.Value) error {
	s.middlewareFuncsMu.RLock()
	defer s.middlewareFuncsMu.RUnlock()

	for _, f := range s.middlewareFuncs {
		err := s.callMiddlewareFunc(f, values)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *serverSocket) callMiddlewareFunc(rv reflect.Value, values []reflect.Value) (err error) {
	defer func() {
		if r := recover(); r != nil {
			var ok bool
			err, ok = r.(error)
			if !ok {
				err = fmt.Errorf("sio: middleware error: %v", r)
			}
		}
	}()
	rets := rv.Call(values)
	ret := rets[0]
	if ret.IsNil() {
		return nil
	}
	err = ret.Interface().(error)
	return
}

func (s *serverSocket) onAck(header *parser.PacketHeader, decode parser.Decode) {
	if header.ID == nil {
		s.onError(wrapInternalError(fmt.Errorf("header.ID is nil")))
		return
	}

	s.debug("Calling ack with ID", *header.ID)

	s.acksMu.Lock()
	ack, ok := s.acks[*header.ID]
	if ok {
		delete(s.acks, *header.ID)
	}
	s.acksMu.Unlock()

	if !ok {
		s.onError(wrapInternalError(fmt.Errorf("ACK with ID %d not found", *header.ID)))
		return
	}

	values, err := decode(ack.inputArgs...)
	if err != nil {
		s.onError(wrapInternalError(err))
		return
	}

	if len(values) == len(ack.inputArgs) {
		for i, v := range values {
			if ack.inputArgs[i].Kind() != reflect.Ptr && v.Kind() == reflect.Ptr {
				values[i] = v.Elem()
			}
		}
	} else {
		s.onError(fmt.Errorf("sio: onEvent: invalid number of arguments"))
		return
	}

	err = ack.Call(values...)
	if err != nil {
		s.onError(wrapInternalError(err))
		return
	}
}

func (s *serverSocket) Join(room ...Room) {
	s.joinMu.Lock()
	join := s.join
	s.joinMu.Unlock()
	join(room...)
}

func (s *serverSocket) Leave(room Room) {
	s.debug("Leaving room", room)
	s.adapter.Delete(s.ID(), room)
}

func (s *serverSocket) Rooms() mapset.Set[Room] {
	rooms, ok := s.adapter.SocketRooms(s.ID())
	if !ok {
		return mapset.NewSet[Room]()
	}
	return rooms
}

func (s *serverSocket) To(room ...Room) *BroadcastOperator {
	return s.newBroadcastOperator().To(room...)
}

func (s *serverSocket) In(room ...Room) *BroadcastOperator {
	return s.To(room...)
}

func (s *serverSocket) Except(room ...Room) *BroadcastOperator {
	return s.newBroadcastOperator().Except(room...)
}

func (s *serverSocket) Local() *BroadcastOperator {
	return s.newBroadcastOperator().Local()
}

func (s *serverSocket) Broadcast() *BroadcastOperator {
	return s.newBroadcastOperator()
}

func (s *serverSocket) newBroadcastOperator() *BroadcastOperator {
	return newBroadcastOperator(s.nsp.Name(), s.adapter, s.parser).Except(Room(s.ID()))
}

type sidInfo struct {
	SID string  `json:"sid"`
	PID *string `json:"pid"`
}

func (s *serverSocket) onConnect() error {
	s.debug("Socket connected. Locking mutex and writing packet")
	s.connectedMu.Lock()
	defer s.connectedMu.Unlock()

	// Socket ID is the default room a socket joins to.
	s.Join(Room(s.ID()))

	header := &parser.PacketHeader{
		Type:      parser.PacketTypeConnect,
		Namespace: s.nsp.Name(),
	}

	pid := string(s.pid)
	c := &sidInfo{
		SID: string(s.ID()),
		PID: &pid,
	}

	buffers, err := s.parser.Encode(header, c)
	if err != nil {
		return wrapInternalError(err)
	}

	s.conn.sendBuffers(buffers...)
	s.connected = true
	return nil
}

func (s *serverSocket) onError(err error) {
	handlers := s.errorHandlers.GetAll()
	if len(handlers) > 0 {
		go func() {
			for _, handler := range handlers {
				(*handler)(err)
			}
		}()
	}
}

func (s *serverSocket) onClose(reason Reason) {
	s.debug("Going to close the socket if it is not already closed")

	// Server socket is one-time, it cannot be reconnected.
	// We don't want it to close more than once,
	// so we use sync.Once to avoid running onClose more than once.
	s.closeOnce.Do(func() {
		s.debug("Going to close the socket. It is not already closed")
		if !s.Connected() {
			return
		}
		for _, handler := range s.disconnectingHandlers.GetAll() {
			(*handler)(reason)
		}

		if s.server.connectionStateRecovery.Enabled && recoverableDisconnectReasons.Contains(reason) {
			s.debug("Connection state recovery is enabled")
			rooms, ok := s.adapter.SocketRooms(s.ID())
			if !ok {
				rooms = mapset.NewThreadUnsafeSet[Room]()
			}
			s.adapter.PersistSession(&SessionToPersist{
				SID:   s.ID(),
				PID:   s.pid,
				Rooms: rooms.ToSlice(),
			})
		}

		s.joinMu.Lock()
		s.join = func(room ...Room) {}
		s.joinMu.Unlock()
		s.leaveAll()

		s.nsp.remove(s)
		s.conn.Remove(s)

		s.connectedMu.Lock()
		s.connected = false
		s.connectedMu.Unlock()
		for _, handler := range s.disconnectHandlers.GetAll() {
			(*handler)(reason)
		}
	})
}

func (s *serverSocket) leaveAll() {
	s.adapter.DeleteAll(s.ID())
}

func (s *serverSocket) ID() SocketID {
	return s.id
}

func (s *serverSocket) Emit(eventName string, v ...any) {
	s.emit(eventName, 0, false, false, v...)
}

func (s *serverSocket) emit(eventName string, timeout time.Duration, volatile, fromQueue bool, _v ...any) {
	header := &parser.PacketHeader{
		Type:      parser.PacketTypeEvent,
		Namespace: s.nsp.Name(),
	}

	if IsEventReservedForServer(eventName) {
		panic("sio: Emit: attempted to emit a reserved event: `" + eventName + "`")
	}

	// One extra space for eventName,
	// the other for ID (see the Broadcast method of sessionAwareAdapter)
	v := make([]any, 0, len(_v)+2)
	v = append(v, eventName)
	v = append(v, _v...)

	f := v[len(v)-1]
	rt := reflect.TypeOf(f)

	if rt.Kind() == reflect.Func {
		ackID := s.registerAckHandler(f, timeout)
		header.ID = &ackID
		v = v[:len(v)-1]
	}

	if s.server.connectionStateRecovery.Enabled {
		opts := NewBroadcastOptions()
		opts.Rooms.Add(Room(s.id))
		s.adapter.Broadcast(header, v, opts)
	} else {
		buffers, err := s.parser.Encode(header, &v)
		if err != nil {
			s.onError(wrapInternalError(err))
			return
		}
		s.conn.sendBuffers(buffers...)
	}
}

// 0 as the timeout argument means there is no timeout.
func (s *serverSocket) registerAckHandler(f any, timeout time.Duration) (id uint64) {
	id = s.nsp.nextAckID()
	s.debug("Registering ack with ID", id)
	if timeout == 0 {
		s.acksMu.Lock()
		s.acks[id] = newAckHandler(f, false)
		s.acksMu.Unlock()
		return
	}

	err := doesAckHandlerHasAnError(f)
	if err != nil {
		panic(err)
	}

	h := newAckHandlerWithTimeout(f, timeout, func() {
		s.debug("Ack with ID", id, "timed out")
		s.acksMu.Lock()
		delete(s.acks, id)
		s.acksMu.Unlock()
	})

	s.acksMu.Lock()
	s.acks[id] = h
	s.acksMu.Unlock()
	return
}

func (s *serverSocket) Timeout(timeout time.Duration) Emitter {
	return Emitter{
		socket:  s,
		timeout: timeout,
	}
}

func (s *serverSocket) sendControlPacket(typ parser.PacketType, v ...any) {
	header := parser.PacketHeader{
		Type:      typ,
		Namespace: s.nsp.Name(),
	}

	buffers, err := s.parser.Encode(&header, &v)
	if err != nil {
		s.onError(wrapInternalError(err))
		return
	}

	s.conn.sendBuffers(buffers...)
}

func (s *serverSocket) sendAckPacket(id uint64, values []reflect.Value) {
	header := parser.PacketHeader{
		Type:      parser.PacketTypeAck,
		Namespace: s.nsp.Name(),
		ID:        &id,
	}

	v := make([]any, len(values))

	for i := range values {
		if values[i].CanInterface() {
			v[i] = values[i].Interface()
		} else {
			s.onError(fmt.Errorf("sio: sendAck: CanInterface must be true"))
			return
		}
	}

	buffers, err := s.parser.Encode(&header, &v)
	if err != nil {
		s.onError(wrapInternalError(err))
		return
	}

	s.conn.sendBuffers(buffers...)
}

func (s *serverSocket) Disconnect(close bool) {
	if !s.Connected() {
		return
	}
	if close {
		s.conn.DisconnectAll()
		s.conn.Close()
	} else {
		s.sendControlPacket(parser.PacketTypeDisconnect)
		s.onClose("server namespace disconnect")
	}
}
