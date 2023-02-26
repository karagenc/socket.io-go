package sio

import (
	"fmt"
	"reflect"
	"sync"

	mapset "github.com/deckarep/golang-set/v2"
	eio "github.com/tomruk/socket.io-go/engine.io"
	"github.com/tomruk/socket.io-go/parser"
)

type serverSocket struct {
	id        SocketID
	pid       PrivateSessionID
	recovered bool

	server  *Server
	conn    *serverConn
	nsp     *Namespace
	adapter Adapter

	emitter *eventEmitter
	parser  parser.Parser

	acks   map[uint64]*ackHandler
	ackID  uint64
	acksMu sync.Mutex

	join   func(room ...Room)
	joinMu sync.Mutex

	closeOnce sync.Once
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
		emitter: newEventEmitter(),
	}

	s.join = func(room ...Room) {
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

func (s *serverSocket) Recovered() bool { return s.recovered }

func (s *serverSocket) onPacket(header *parser.PacketHeader, eventName string, decode parser.Decode) error {
	switch header.Type {
	case parser.PacketTypeEvent, parser.PacketTypeBinaryEvent:
		handlers := s.emitter.GetHandlers(eventName)

		go func() {
			for _, handler := range handlers {
				s.onEvent(handler, header, decode)
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
	s.onClose("client namespace disconnect")
}

func (s *serverSocket) onEvent(handler *eventHandler, header *parser.PacketHeader, decode parser.Decode) {
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

	ret, err := handler.Call(values...)
	if err != nil {
		s.onError(wrapInternalError(err))
		return
	}

	if header.ID != nil {
		s.sendAckPacket(*header.ID, ret)
	}
}

func (s *serverSocket) onAck(header *parser.PacketHeader, decode parser.Decode) {
	if header.ID == nil {
		s.onError(wrapInternalError(fmt.Errorf("header.ID is nil")))
		return
	}

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
	SID string `json:"sid"`
	PID string `json:"pid"`
}

func (s *serverSocket) onConnect() {
	s.Join(Room(s.ID()))

	header := &parser.PacketHeader{
		Type:      parser.PacketTypeConnect,
		Namespace: s.nsp.Name(),
	}

	c := &sidInfo{
		SID: string(s.ID()),
		PID: string(s.pid),
	}

	buffers, err := s.parser.Encode(header, c)
	if err != nil {
		panic(wrapInternalError(err))
	}

	s.conn.sendBuffers(buffers...)
}

// Convenience method for emitting events to the user.
func (s *serverSocket) emitReserved(eventName string, v ...interface{}) {
	handlers := s.emitter.GetHandlers(eventName)
	values := make([]reflect.Value, len(v))
	for i := range values {
		values[i] = reflect.ValueOf(v)
	}

	go func() {
		for _, handler := range handlers {
			_, err := handler.Call(values...)
			if err != nil {
				s.onError(wrapInternalError(fmt.Errorf("emitReserved: %s", err)))
				return
			}
		}
	}()
}

func (s *serverSocket) onError(err error) {
	// emitReserved is not used because if an error would happen in handler.Call
	// onError would be called recursively.

	errValue := reflect.ValueOf(err)

	handlers := s.emitter.GetHandlers("error")
	go func() {
		for _, handler := range handlers {
			_, err := handler.Call(errValue)
			if err != nil {
				// This should panic.
				// If you cannot handle the error via `onError`
				// then what option do you have?
				panic(fmt.Errorf("sio: %w", err))
			}
		}
	}()
}

// TODO: Check these
var recoverableDisconnectReasons = mapset.NewThreadUnsafeSet(
	"transport error",
	"transport close",
	"forced close",
	"ping timeout",
	"server shutting down",
	"forced server close",
)

func (s *serverSocket) onClose(reason string) {
	s.closeOnce.Do(func() {
		s.emitReserved("disconnecting", reason)

		if s.server.connectionStateRecovery.Enabled && recoverableDisconnectReasons.Contains(reason) {
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
		s.emitReserved("disconnect", reason)
	})
}

func (s *serverSocket) leaveAll() {
	s.adapter.DeleteAll(s.ID())
}

func (s *serverSocket) ID() SocketID {
	return s.id
}

func (s *serverSocket) setAck(handler *ackHandler) (id uint64) {
	s.acksMu.Lock()
	id = s.ackID
	s.acks[id] = handler
	s.ackID++
	s.acksMu.Unlock()
	return
}

func (s *serverSocket) Emit(eventName string, v ...interface{}) {
	s.sendDataPacket(parser.PacketTypeEvent, eventName, v...)
}

func (s *serverSocket) sendDataPacket(typ parser.PacketType, eventName string, _v ...interface{}) {
	header := &parser.PacketHeader{
		Type:      typ,
		Namespace: s.nsp.Name(),
	}

	if IsEventReservedForServer(eventName) {
		panic(fmt.Errorf("sio: Emit: attempted to emit a reserved event"))
	}

	// One extra space for eventName,
	// the other for ID (see the Broadcast method of sessionAwareAdapter)
	v := make([]interface{}, 0, len(_v)+2)
	v = append(v, eventName)
	v = append(v, v...)

	if len(v) > 0 {
		f := v[len(v)-1]
		rt := reflect.TypeOf(f)

		if rt.Kind() == reflect.Func {
			ackID := s.setAck(newAckHandler(f))
			header.ID = &ackID
			v = v[:len(v)-1]
		}
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

func (s *serverSocket) sendControlPacket(typ parser.PacketType, v ...interface{}) {
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

	v := make([]interface{}, len(values))

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

func (s *serverSocket) Server() *Server { return s.server }

func (s *serverSocket) Namespace() *Namespace { return s.nsp }

func (s *serverSocket) On(eventName string, handler interface{}) {
	s.checkHandler(eventName, handler)
	s.emitter.On(eventName, handler)
}

func (s *serverSocket) Once(eventName string, handler interface{}) {
	s.checkHandler(eventName, handler)
	s.emitter.On(eventName, handler)
}

func (s *serverSocket) checkHandler(eventName string, handler interface{}) {
	switch eventName {
	case "":
		fallthrough
	case "connect":
		fallthrough
	case "connect_error":
		fallthrough
	case "disconnecting":
		fallthrough
	case "disconnect":
		err := checkHandler(eventName, handler)
		if err != nil {
			panic(fmt.Errorf("sio: %w", err))
		}
	}
}

func (s *serverSocket) Off(eventName string, handler interface{}) {
	s.emitter.Off(eventName, handler)
}

func (s *serverSocket) OffAll() {
	s.emitter.OffAll()
}

func (s *serverSocket) Disconnect(close bool) {
	if close {
		s.conn.DisconnectAll()
		s.conn.Close()
	} else {
		s.sendControlPacket(parser.PacketTypeDisconnect)
		s.onClose("server namespace disconnect")
	}
}
