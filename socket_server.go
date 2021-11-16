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
		/*
			handlers := s.emitter.GetHandlers(eventName)

			for _, handler := range handlers {
				s.onEvent(handler, header, decode)
			}
		*/

	case parser.PacketTypeAck, parser.PacketTypeBinaryAck:
		//s.onAck(header, decode)

	case parser.PacketTypeConnectError:
		//s.onConnectError(header, decode)

	case parser.PacketTypeDisconnect:
		//s.onDisconnect()
	}
}

func (s *serverSocket) onConnect() {
	// TODO: this.join(this.id)

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

func (s *serverSocket) Emit(v ...interface{}) {
	header := parser.PacketHeader{
		Type:      parser.PacketTypeEvent,
		Namespace: s.nsp.Name(),
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

	s.conn.sendBuffers(buffers...)
}

func (s *serverSocket) packet(packets ...*eioparser.Packet) {
	s.conn.packet(packets...)
}

// This is client only. Do nothing.
func (s *serverSocket) Connect() {}

// This is client only. Return nil.
func (s *serverSocket) IO() *Client { return nil }

func (s *serverSocket) OnConnect(handler ConnectCallback) {}

func (s *serverSocket) OnceConnect(handler ConnectCallback) {}

func (s *serverSocket) OffConnect(handler ConnectCallback) {}

func (s *serverSocket) OnConnectError(handler ConnectErrorCallback) {}

func (s *serverSocket) OnceConnectError(handler ConnectErrorCallback) {}

func (s *serverSocket) OffConnectError(handler ConnectErrorCallback) {}

func (s *serverSocket) OnDisconnect(handler DisconnectCallback) {}

func (s *serverSocket) OnceDisconnect(handler DisconnectCallback) {}

func (s *serverSocket) OffDisconnect(handler DisconnectCallback) {}

func (s *serverSocket) OnDisconnecting(handler DisconnectingCallback) {}

func (s *serverSocket) OnceDisconnecting(handler DisconnectingCallback) {}

func (s *serverSocket) OffDisconnecting(handler DisconnectingCallback) {}

func (s *serverSocket) OnEvent(eventName string, handler interface{}) {}

func (s *serverSocket) OnceEvent(eventName string, handler interface{}) {}

func (s *serverSocket) OffEvent(eventName string, handler interface{}) {}

func (s *serverSocket) OffAll() {}

func (s *serverSocket) Close() {}
