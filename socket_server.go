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

	emitter *eventEmitter

	acks   map[uint64]*ackHandler
	ackID  uint64
	acksMu sync.Mutex
}

func newServerSocket(c *serverConn) (*serverSocket, error) {
	id, err := eio.GenerateBase64ID(eio.Base64IDSize)
	if err != nil {
		return nil, err
	}

	s := &serverSocket{
		id:      id,
		conn:    c,
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
			//s.onEvent(handler, header, decode)
		}

	case parser.PacketTypeAck, parser.PacketTypeBinaryAck:
		//s.onAck(header, decode)

	case parser.PacketTypeConnectError:
		//s.onConnectError(header, decode)

	case parser.PacketTypeDisconnect:
		//s.onDisconnect()
	}
}

func (s *serverSocket) onError(err error) {

}

func (s *serverSocket) onClose(reason string, err error) {

}

func (s *serverSocket) ID() string {
	return s.id
}

func (s *serverSocket) Emit(v ...interface{}) {
	header := parser.PacketHeader{
		Type:      parser.PacketTypeEvent,
		Namespace: s.namespace,
	}

	if len(v) == 0 {
		panic(fmt.Errorf("Emit: at least 1 argument expected"))
	}

	eventName := reflect.ValueOf(v)
	if eventName.Kind() != reflect.String {
		panic(fmt.Errorf("Emit: string expected"))
	}

	if IsEventReserved(eventName.String()) {
		panic(fmt.Errorf("Emit: attempted to emit to a reserved event"))
	}

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

	buffers, err := s.parser.Encode(&header, &v)
	if err != nil {
		s.onError(err)
		return
	}

	s.send(buffers...)
}

func (s *serverSocket) send(buffers ...[]byte) {
	if len(buffers) > 0 {
		packets := make([]*eioparser.Packet, len(buffers))
		buf := buffers[0]
		buffers = buffers[1:]

		var err error
		packets[0], err = eioparser.NewPacket(eioparser.PacketTypeMessage, false, buf)
		if err != nil {
			s.onError(err)
			return
		}

		for i, attachment := range buffers {
			packets[i+1], err = eioparser.NewPacket(eioparser.PacketTypeMessage, true, attachment)
			if err != nil {
				s.onError(err)
				return
			}
		}

		s.packet(packets...)
	}
}

func (s *serverSocket) packet(packets ...*eioparser.Packet) {
	s.conn.packet(packets...)
}

func (s *serverSocket) Close() {}
