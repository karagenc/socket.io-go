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

	conn *serverConn
	nsp  *Namespace

	emitter *eventEmitter
	parser  parser.Parser

	acks   map[uint64]*ackHandler
	ackID  uint64
	acksMu sync.Mutex
}

func newServerSocket(c *serverConn, nsp *Namespace, parser parser.Parser) (*serverSocket, error) {
	id, err := eio.GenerateBase64ID(eio.Base64IDSize)
	if err != nil {
		return nil, err
	}

	s := &serverSocket{
		id:      id,
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
		//s.onAck(header, decode)

	case parser.PacketTypeConnectError:
		//s.onConnectError(header, decode)

	case parser.PacketTypeDisconnect:
		//s.onDisconnect()
	}
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

func (s *serverSocket) Join(room string) {

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

func (s *serverSocket) onClose(reason string, err error) {}

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

// This is client only. Do nothing.
func (s *serverSocket) Connect() {}

// This is client only. Return nil.
func (s *serverSocket) IO() *Client { return nil }

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

func (s *serverSocket) Close() {}
