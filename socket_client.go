package sio

import (
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"

	eioparser "github.com/tomruk/socket.io-go/engine.io/parser"
	"github.com/tomruk/socket.io-go/parser"
)

type clientSocket struct {
	id        atomic.Value
	namespace string
	client    *Client
	parser    parser.Parser

	connected   bool
	connectedMu sync.Mutex

	sendBuffer   []*eioparser.Packet
	sendBufferMu sync.Mutex

	receiveBuffer   []*eioparser.Packet
	receiveBufferMu sync.Mutex
}

func newClientSocket(client *Client, namespace string, parser parser.Parser) *clientSocket {
	s := &clientSocket{
		namespace: namespace,
		client:    client,
		parser:    parser,
	}
	return s
}

func (s *clientSocket) ID() string {
	id, _ := s.id.Load().(string)
	return id
}

func (s *clientSocket) setID(id string) {
	s.id.Store(id)
}

func (s *clientSocket) Connect() error {
	s.connectedMu.Lock()
	connected := s.connected
	s.connectedMu.Unlock()

	if connected {
		return nil
	} else {
		return s.client.connect()
	}
}

func (s *clientSocket) onEIOConnect() {
	header := parser.PacketHeader{
		Type:      parser.PacketTypeConnect,
		Namespace: s.namespace,
	}

	buffers, err := s.parser.Encode(&header, nil)
	if err != nil {
		s.onError(err)
		return
	} else if len(buffers) != 1 {
		s.onError(fmt.Errorf("onEIOConnect: len(buffers) != 1"))
		return
	}

	packets := []*eioparser.Packet{nil}
	buf := buffers[0]

	packets[0], err = eioparser.NewPacket(eioparser.PacketTypeMessage, false, buf)
	if err != nil {
		s.onError(err)
		return
	}
	s.client.packet(packets...)
}

func (s *clientSocket) onPacket(header *parser.PacketHeader, eventName string, decode parser.Decode) {
	switch header.Type {
	case parser.PacketTypeConnect:
		var v map[string]interface{}
		vt := reflect.TypeOf(v)
		values, err := decode(vt)
		if err != nil {
			s.onError(err)
			return
		} else if len(values) != 1 {
			s.onError(fmt.Errorf("invalid CONNECT packet"))
			return
		}

		m, ok := values[0].Interface().(*map[string]interface{})
		if !ok {
			s.onError(fmt.Errorf("invalid CONNECT packet: cast failed"))
			return
		}

		idiface, ok := (*m)["sid"]
		if !ok {
			s.onError(fmt.Errorf("invalid CONNECT packet: sid expected"))
			return
		}

		id, ok := idiface.(string)
		if !ok {
			s.onError(fmt.Errorf("invalid CONNECT packet: sid must be string"))
			return
		}

		s.setID(id)
		s.onConnect()
	}
}

func (s *clientSocket) onConnect() {
	s.connectedMu.Lock()
	s.connected = true
	s.connectedMu.Unlock()

	// TODO: emitReserved("connect")

	s.sendBufferMu.Lock()
	defer s.sendBufferMu.Unlock()
	if len(s.sendBuffer) != 0 {
		s.client.packet(s.sendBuffer...)
		s.sendBuffer = nil
	}

	// TODO: receiveBuffer
}

func (s *clientSocket) onError(err error) {
	s.client.onError(err)
}

func (s *clientSocket) Emit(v ...interface{}) {
	header := parser.PacketHeader{
		Type:      parser.PacketTypeEvent,
		Namespace: s.namespace,
	}

	buffers, err := s.parser.Encode(&header, &v)
	if err != nil {
		s.onError(err)
		return
	}

	if len(buffers) > 0 {
		packets := make([]*eioparser.Packet, len(buffers))
		buf := buffers[0]
		buffers = buffers[1:]

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

		s.connectedMu.Lock()
		if s.connected {
			s.client.packet(packets...)
		} else {
			s.sendBufferMu.Lock()
			s.sendBuffer = append(s.sendBuffer, packets...)
			s.sendBufferMu.Unlock()
		}
		s.connectedMu.Unlock()
	}
}
