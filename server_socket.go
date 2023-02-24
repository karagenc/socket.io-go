package sio

import (
	"fmt"
	"reflect"
	"sync"

	eio "github.com/tomruk/socket.io-go/engine.io"
	"github.com/tomruk/socket.io-go/parser"
)

type serverSocket struct {
	id string

	server *Server
	conn   *serverConn
	nsp    *Namespace

	emitter *eventEmitter
	parser  parser.Parser

	acks   map[uint64]*ackHandler
	ackID  uint64
	acksMu sync.Mutex

	join   func(room ...string)
	joinMu sync.Mutex

	closeOnce sync.Once
}

func newServerSocket(server *Server, c *serverConn, nsp *Namespace, parser parser.Parser) (*serverSocket, error) {
	id, err := eio.GenerateBase64ID(eio.Base64IDSize)
	if err != nil {
		return nil, err
	}

	s := &serverSocket{
		id:      id,
		server:  server,
		conn:    c,
		nsp:     nsp,
		parser:  parser,
		emitter: newEventEmitter(),

		join: func(room ...string) {
			nsp.Adapter().AddAll(id, room)
		},
	}
	return s, nil
}

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
	s.onClose("server namespace disconnect")
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

func (s *serverSocket) Join(room ...string) {
	s.joinMu.Lock()
	join := s.join
	s.joinMu.Unlock()
	join(room...)
}

func (s *serverSocket) Leave(room string) {
	s.nsp.Adapter().Delete(s.ID(), room)
}

type sidInfo struct {
	SID string `json:"sid"`
}

func (s *serverSocket) onConnect() {
	header := &parser.PacketHeader{
		Type:      parser.PacketTypeConnect,
		Namespace: s.nsp.Name(),
	}

	c := &sidInfo{
		SID: s.ID(),
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

func (s *serverSocket) onClose(reason string) {
	s.closeOnce.Do(func() {
		s.emitReserved("disconnecting", reason)

		s.joinMu.Lock()
		s.join = func(room ...string) {}
		s.joinMu.Unlock()
		s.leaveAll()

		s.nsp.remove(s)
		s.conn.sockets.Remove(s.ID())
		s.emitReserved("disconnect", reason)
	})
}

func (s *serverSocket) leaveAll() {
	s.nsp.adapter.DeleteAll(s.ID())
}

func (s *serverSocket) ID() string {
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

func (s *serverSocket) sendDataPacket(typ parser.PacketType, eventName string, v ...interface{}) {
	header := parser.PacketHeader{
		Type:      typ,
		Namespace: s.nsp.Name(),
	}

	if IsEventReservedForServer(eventName) {
		panic(fmt.Errorf("sio: Emit: attempted to emit a reserved event"))
	}

	v = append([]interface{}{eventName}, v...)

	if len(v) > 0 {
		f := v[len(v)-1]
		rt := reflect.TypeOf(f)

		if rt.Kind() == reflect.Func {
			ackID := s.setAck(newAckHandler(f))
			header.ID = &ackID
			v = v[:len(v)-1]
		}
	}

	buffers, err := s.parser.Encode(&header, &v)
	if err != nil {
		s.onError(wrapInternalError(err))
		return
	}

	s.conn.sendBuffers(buffers...)
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
