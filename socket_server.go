package sio

import (
	"fmt"
	"reflect"
	"sync"

	eio "github.com/tomruk/socket.io-go/engine.io"
	eioparser "github.com/tomruk/socket.io-go/engine.io/parser"
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
	}

	return s, nil
}

func (s *serverSocket) Auth() *Auth { return nil }

func (s *serverSocket) onPacket(header *parser.PacketHeader, eventName string, decode parser.Decode) {
	switch header.Type {
	case parser.PacketTypeEvent, parser.PacketTypeBinaryEvent:
		handlers := s.emitter.GetHandlers(eventName)

		for _, handler := range handlers {
			s.onEvent(handler, header, decode)
		}
	case parser.PacketTypeAck, parser.PacketTypeBinaryAck:
		s.onAck(header, decode)

	case parser.PacketTypeConnectError:
		s.onConnectError(header, decode)

	case parser.PacketTypeDisconnect:
		s.onDisconnect()
	}
}

func (s *serverSocket) onConnectError(header *parser.PacketHeader, decode parser.Decode) {

}

func (s *serverSocket) onDisconnect() {
	s.onClose("server namespace disconnect")
}

func (s *serverSocket) onEvent(handler *eventHandler, header *parser.PacketHeader, decode parser.Decode) {
	values, err := decode(handler.inputArgs...)
	if err != nil {
		s.onError(err)
		return
	}

	if len(values) == len(handler.inputArgs) {
		for i, v := range values {
			if handler.inputArgs[i].Kind() != reflect.Ptr && v.Kind() == reflect.Ptr {
				values[i] = v.Elem()
			}
		}
	} else {
		s.onError(fmt.Errorf("onEvent: invalid number of arguments"))
		return
	}

	go func() {
		_, err := handler.Call(values...)
		if err != nil {
			s.onError(err)
			return
		}

		if header.ID != nil {
			//s.sendAck(*header.ID, ret)
		}
	}()
}

func (s *serverSocket) onAck(header *parser.PacketHeader, decode parser.Decode) {
	if header.ID == nil {
		// TODO: Error?
		return
	}
	s.acksMu.Lock()
	ack, ok := s.acks[*header.ID]
	delete(s.acks, *header.ID)
	s.acksMu.Unlock()

	if !ok {
		// TODO: Error?
		return
	}

	values, err := decode(ack.inputArgs...)
	if err != nil {
		s.onError(err)
		return
	}

	if len(values) == len(ack.inputArgs) {
		for i, v := range values {
			if ack.inputArgs[i].Kind() != reflect.Ptr && v.Kind() == reflect.Ptr {
				values[i] = v.Elem()
			}
		}
	} else {
		s.onError(fmt.Errorf("onEvent: invalid number of arguments"))
		return
	}

	go func() {
		err := ack.Call(values...)
		if err != nil {
			s.onError(err)
			return
		}
	}()
}

func (s *serverSocket) Join(room ...string) {

}

func (s *serverSocket) Leave(room ...string) {

}

func (s *serverSocket) emitReserved(eventName string, v ...interface{}) {
	handlers := s.emitter.GetHandlers(eventName)
	values := make([]reflect.Value, len(v))
	for i := range values {
		values[i] = reflect.ValueOf(v)
	}

	for _, handler := range handlers {
		go func(handler *eventHandler) {
			_, err := handler.Call(values...)
			if err != nil {
				s.onError(fmt.Errorf("emitReserved: %s", err))
				return
			}
		}(handler)
	}
}

func (s *serverSocket) onConnect() {
	header := &parser.PacketHeader{
		Type:      parser.PacketTypeConnect,
		Namespace: s.nsp.Name(),
	}

	c := struct {
		SID string `json:"sid"`
	}{
		SID: s.ID(),
	}

	buffers, err := s.parser.Encode(header, &c)
	if err != nil {
		panic(err)
	}

	s.conn.sendBuffers(buffers...)
}

func (s *serverSocket) onError(err error) {}

func (s *serverSocket) leaveAll() {
	s.nsp.adapter.DeleteAll(s.ID())
}

func (s *serverSocket) onClose(reason string) {
	s.emitReserved("disconnecting", reason)
	s.nsp.remove(s)
	s.conn.sockets.Remove(s.ID())
	s.emitReserved("disconnect", reason)
}

func (s *serverSocket) ID() string {
	return s.id
}

func (s *serverSocket) Emit(eventName string, v ...interface{}) {
	header := parser.PacketHeader{
		Type:      parser.PacketTypeEvent,
		Namespace: s.nsp.Name(),
	}

	if IsEventReservedForServer(eventName) {
		panic(fmt.Errorf("Emit: attempted to emit to a reserved event"))
	}

	v = append([]interface{}{eventName}, v...)

	if len(v) > 0 {
		f := v[len(v)-1]
		rt := reflect.TypeOf(f)

		if rt.Kind() == reflect.Func {
			ackHandler := newAckHandler(f)

			s.acksMu.Lock()
			id := s.ackID
			s.acks[id] = ackHandler
			s.ackID++
			s.acksMu.Unlock()

			header.ID = &id

			v = v[:len(v)-1]
		}
	}

	buffers, err := s.parser.Encode(&header, &v)
	if err != nil {
		s.onError(err)
		return
	}

	s.conn.sendBuffers(buffers...)
}

func (s *serverSocket) packet(packets ...*eioparser.Packet) {
	s.conn.packet(packets...)
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
		checkHandler(eventName, handler)
	}
}

func (s *serverSocket) Off(eventName string, handler interface{}) {
	s.emitter.Off(eventName, handler)
}

func (s *serverSocket) OffAll() {
	s.emitter.OffAll()
}

func (s *serverSocket) Disconnect(close bool) {}
