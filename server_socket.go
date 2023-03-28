package sio

import (
	"fmt"
	"reflect"
	"time"

	"github.com/tomruk/socket.io-go/internal/sync"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/tomruk/socket.io-go/adapter"
	eio "github.com/tomruk/socket.io-go/engine.io"
	"github.com/tomruk/socket.io-go/parser"
)

type serverSocket struct {
	id        SocketID
	pid       adapter.PrivateSessionID
	recovered bool

	// `connected` means "did we receive the CONNECT packet?"
	connected   bool
	connectedMu sync.RWMutex

	server  *Server
	conn    *serverConn
	nsp     *Namespace
	adapter adapter.Adapter

	parser parser.Parser

	acks   map[uint64]*ackHandler
	acksMu sync.Mutex

	middlewareFuncs   []reflect.Value
	middlewareFuncsMu sync.RWMutex

	join   func(room ...Room)
	joinMu sync.Mutex

	closeOnce sync.Once
	debug     Debugger

	eventHandlers         *eventHandlerStore
	errorHandlers         *handlerStore[*ServerSocketErrorFunc]
	disconnectingHandlers *handlerStore[*ServerSocketDisconnectingFunc]
	disconnectHandlers    *handlerStore[*ServerSocketDisconnectFunc]
}

// previousSession can be nil
func newServerSocket(
	server *Server,
	c *serverConn,
	nsp *Namespace,
	parser parser.Parser,
	previousSession *adapter.SessionToPersist,
) (*serverSocket, error) {
	_adapter := nsp.Adapter()
	s := &serverSocket{
		server:  server,
		conn:    c,
		nsp:     nsp,
		adapter: _adapter,
		parser:  parser,
		acks:    make(map[uint64]*ackHandler),

		eventHandlers:         newEventHandlerStore(),
		errorHandlers:         newHandlerStore[*ServerSocketErrorFunc](),
		disconnectingHandlers: newHandlerStore[*ServerSocketDisconnectingFunc](),
		disconnectHandlers:    newHandlerStore[*ServerSocketDisconnectFunc](),
	}

	s.join = func(room ...Room) {
		s.debug.Log("Joining room(s)", room)
		_adapter.AddAll(s.ID(), room)
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
			s.pid = adapter.PrivateSessionID(id)
		}
	}
	nsp.debug.Log("New socket! ID", s.id)
	s.debug = server.debug.WithContext("[sio/server] Socket (nsp: `" + nsp.Name() + "`)")
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

func (s *serverSocket) onPacket(header *parser.PacketHeader, eventName string, decode parser.Decode) error {
	switch header.Type {
	case parser.PacketTypeEvent, parser.PacketTypeBinaryEvent:
		var (
			hasAckFunc bool // This doesn't need mutex

			mu   sync.Mutex
			sent bool
		)

		sendAck := func(ackID uint64, values []reflect.Value) {
			mu.Lock()
			if sent {
				mu.Unlock()
				return
			}
			sent = true
			mu.Unlock()

			s.debug.Log("Sending ack with ID", ackID)
			s.sendAckPacket(ackID, values)
		}

		for _, handler := range s.eventHandlers.getAll(eventName) {
			_hasAckFunc := s.onEvent(handler, header, decode, sendAck)
			if _hasAckFunc {
				hasAckFunc = true
			}
		}
		if header.ID != nil {
			mu.Lock()
			send := !hasAckFunc
			if sent {
				mu.Unlock()
				return nil
			}
			sent = true
			mu.Unlock()

			// If there is no acknowledgement function
			// and there is no response already sent,
			// then send an empty acknowledgement.
			if send {
				s.debug.Log("Sending ack with ID", *header.ID)
				s.sendAckPacket(*header.ID, nil)
			}
		}
	case parser.PacketTypeAck, parser.PacketTypeBinaryAck:
		s.onAck(header, decode)

	case parser.PacketTypeDisconnect:
		s.onDisconnect()
	default:
		return wrapInternalError(fmt.Errorf("invalid packet type: %d", header.Type))
	}

	return nil
}

func (s *serverSocket) onDisconnect() {
	s.debug.Log("Got disconnect packet")
	s.onClose(ReasonClientNamespaceDisconnect)
}

func (s *serverSocket) onEvent(
	handler *eventHandler,
	header *parser.PacketHeader,
	decode parser.Decode,
	sendAck ackSendFunc,
) (hasAckFunc bool) {
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
		s.debug.Log("ignore packet received after disconnection")
		return
	}

	ack, _ := handler.ack()
	if header.ID != nil && ack {
		hasAckFunc = true

		// We already know that the last value of the handler is an ack function
		// and it doesn't have a return value. So dismantle it, and create it with reflect.MakeFunc.
		f := values[len(values)-1]
		in, variadic := dismantleAckFunc(f.Type())
		rt := reflect.FuncOf(in, nil, variadic)

		f = reflect.MakeFunc(rt, func(args []reflect.Value) (results []reflect.Value) {
			sendAck(*header.ID, args)
			return nil
		})
		values[len(values)-1] = f
	}

	_, err = handler.call(values...)
	if err != nil {
		s.onError(wrapInternalError(err))
		return
	}
	return
}

func (s *serverSocket) onAck(header *parser.PacketHeader, decode parser.Decode) {
	if header.ID == nil {
		s.onError(wrapInternalError(fmt.Errorf("header.ID is nil")))
		return
	}

	s.debug.Log("Calling ack with ID", *header.ID)

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

	err = ack.call(values...)
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
	s.debug.Log("Leaving room", room)
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
	return adapter.NewBroadcastOperator(s.nsp.Name(), s.adapter, IsEventReservedForServer).Except(Room(s.ID()))
}

type sidInfo struct {
	SID string `json:"sid"`
	PID string `json:"pid,omitempty"`
}

func (s *serverSocket) onConnect() error {
	s.debug.Log("Socket connected. Locking mutex and writing packet")
	s.connectedMu.Lock()
	defer s.connectedMu.Unlock()

	// Socket ID is the default room a socket joins to.
	s.Join(Room(s.ID()))

	header := &parser.PacketHeader{
		Type:      parser.PacketTypeConnect,
		Namespace: s.nsp.Name(),
	}

	c := sidInfo{
		SID: string(s.ID()),
		PID: string(s.pid),
	}

	buffers, err := s.parser.Encode(header, &c)
	if err != nil {
		return wrapInternalError(err)
	}

	s.conn.sendBuffers(buffers...)
	s.connected = true
	return nil
}

func (s *serverSocket) onError(err error) {
	s.errorHandlers.forEach(func(handler *ServerSocketErrorFunc) { (*handler)(err) }, true)
}

func (s *serverSocket) onClose(reason Reason) {
	s.debug.Log("Going to close the socket if it is not already closed. Reason", reason)

	// Server socket is one-time, it cannot be reconnected.
	// We don't want it to close more than once,
	// so we use sync.Once to avoid running onClose more than once.
	s.closeOnce.Do(func() {
		s.debug.Log("Going to close the socket. It is not already closed. Reason", reason)
		if !s.Connected() {
			return
		}
		s.connectedMu.Lock()
		s.connected = false
		s.connectedMu.Unlock()

		s.disconnectingHandlers.forEach(func(handler *ServerSocketDisconnectingFunc) { (*handler)(reason) }, true)

		if s.server.connectionStateRecovery.Enabled && recoverableDisconnectReasons.Contains(reason) {
			s.debug.Log("Connection state recovery is enabled")
			rooms, ok := s.adapter.SocketRooms(s.ID())
			if !ok {
				rooms = mapset.NewThreadUnsafeSet[Room]()
			}
			s.adapter.PersistSession(&adapter.SessionToPersist{
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
		s.conn.remove(s)

		s.disconnectHandlers.forEach(func(handler *ServerSocketDisconnectFunc) { (*handler)(reason) }, true)
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

func (s *serverSocket) emit(
	eventName string,
	timeout time.Duration,
	volatile, fromQueue bool,
	_v ...any) {
	header := &parser.PacketHeader{
		Type:      parser.PacketTypeEvent,
		Namespace: s.nsp.Name(),
	}

	if IsEventReservedForServer(eventName) {
		panic(fmt.Errorf("sio: Emit: attempted to emit a reserved event: `%s`", eventName))
	}

	// One extra space for eventName,
	// the other for ID (see the Broadcast method of sessionAwareAdapter)
	v := make([]any, 0, len(_v)+2)
	v = append(v, eventName)
	v = append(v, _v...)

	f := v[len(v)-1]
	rt := reflect.TypeOf(f)

	if f != nil && rt.Kind() == reflect.Func {
		ackID := s.registerAckHandler(f, timeout)
		header.ID = &ackID
		v = v[:len(v)-1]
	}

	if s.server.connectionStateRecovery.Enabled {
		opts := adapter.NewBroadcastOptions()
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
	s.debug.Log("Registering ack with ID", id)
	if timeout == 0 {
		s.acksMu.Lock()
		h, err := newAckHandler(f, false)
		if err != nil {
			panic(err)
		}
		s.acks[id] = h
		s.acksMu.Unlock()
		return
	}

	h, err := newAckHandlerWithTimeout(f, timeout, func() {
		s.debug.Log("Timeout occured for ack with ID", id, "timeout", timeout)
		s.acksMu.Lock()
		delete(s.acks, id)
		s.acksMu.Unlock()
	})
	if err != nil {
		panic(err)
	}

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
		s.conn.disconnectAll()
		s.conn.close()
	} else {
		s.sendControlPacket(parser.PacketTypeDisconnect)
		s.onClose(ReasonServerNamespaceDisconnect)
	}
}
