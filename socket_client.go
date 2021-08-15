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

	auth *Auth

	connected   bool
	connectedMu sync.Mutex

	sendBuffer   []*eioparser.Packet
	sendBufferMu sync.Mutex

	emitter *eventEmitter

	acks   map[uint64]*ackHandler
	ackID  uint64
	acksMu sync.Mutex
}

func newClientSocket(client *Client, namespace string, parser parser.Parser, authData interface{}) *clientSocket {
	s := &clientSocket{
		namespace: namespace,
		client:    client,
		parser:    parser,
		auth:      newAuth(),
		emitter:   newEventEmitter(),
		acks:      make(map[uint64]*ackHandler),
	}
	s.auth.Set(authData)
	return s
}

func (s *clientSocket) ID() string {
	id, _ := s.id.Load().(string)
	return id
}

func (s *clientSocket) setID(id string) {
	s.id.Store(id)
}

func (s *clientSocket) Connect() {
	s.connectedMu.Lock()
	connected := s.connected
	s.connectedMu.Unlock()

	if connected {
		return
	} else {
		err := s.client.connect()
		if err != nil && s.client.noReconnection == false {
			go s.client.reconnect()
		}
	}
}

func (s *clientSocket) Auth() *Auth {
	return s.auth
}

func (s *clientSocket) onEIOConnect() {
	header := parser.PacketHeader{
		Type:      parser.PacketTypeConnect,
		Namespace: s.namespace,
	}

	authData := s.auth.Get()

	buffers, err := s.parser.Encode(&header, &authData)
	if err != nil {
		s.onError(err)
		return
	} else if len(buffers) != 1 {
		s.onError(fmt.Errorf("onEIOConnect: len(buffers) != 1"))
		return
	}

	buf := buffers[0]

	packet, err := eioparser.NewPacket(eioparser.PacketTypeMessage, false, buf)
	if err != nil {
		s.onError(err)
		return
	}
	s.client.packet(packet)
}

func (s *clientSocket) onPacket(header *parser.PacketHeader, eventName string, decode parser.Decode) {
	switch header.Type {
	case parser.PacketTypeConnect:
		s.onConnect(header, decode)

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

func (s *clientSocket) onConnect(header *parser.PacketHeader, decode parser.Decode) {
	type sidInfo struct {
		SID string `json:"sid"`
	}

	var v *sidInfo
	vt := reflect.TypeOf(v)
	values, err := decode(vt)
	if err != nil {
		s.onError(err)
		return
	} else if len(values) != 1 {
		s.onError(fmt.Errorf("invalid CONNECT packet"))
		return
	}

	v, ok := values[0].Interface().(*sidInfo)
	if !ok {
		s.onError(fmt.Errorf("invalid CONNECT packet: cast failed"))
		return
	}

	if v.SID == "" {
		s.onError(fmt.Errorf("invalid CONNECT packet: sid is empty"))
		return
	}

	s.setID(v.SID)

	s.connectedMu.Lock()
	s.connected = true
	s.connectedMu.Unlock()

	handlers := s.emitter.GetHandlers("connect")
	for _, handler := range handlers {
		_, err := handler.Call()
		if err != nil {
			go s.onError(err)
		}
	}

	// TODO: emitReserved("connect")

	s.sendBufferMu.Lock()
	defer s.sendBufferMu.Unlock()
	if len(s.sendBuffer) != 0 {
		s.client.packet(s.sendBuffer...)
		s.sendBuffer = nil
	}
}

func (s *clientSocket) onConnectError(header *parser.PacketHeader, decode parser.Decode) {
	type connectError struct {
		Message string `json:"message"`
	}

	var v *connectError
	vt := reflect.TypeOf(v)
	values, err := decode(vt)
	if err != nil {
		s.onError(err)
		return
	} else if len(values) != 1 {
		s.onError(fmt.Errorf("invalid CONNECT_ERROR packet"))
		return
	}

	v, ok := values[0].Interface().(*connectError)
	if !ok {
		s.onError(fmt.Errorf("invalid CONNECT_ERROR packet: cast failed"))
		return
	}

	connErr := fmt.Errorf("%s", v.Message)
	handlers := s.emitter.GetHandlers("connect_error")

	for _, handler := range handlers {
		rv := reflect.ValueOf(connErr)
		_, err := handler.Call(rv)
		if err != nil {
			go s.onError(err)
		}
	}
}

func (s *clientSocket) onDisconnect() {
	// TODO: onDisconnect
}

func (s *clientSocket) onEvent(handler *eventHandler, header *parser.PacketHeader, decode parser.Decode) {
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
		// TODO: Handle error?
		//return
	}

	go func() {
		ret, err := handler.Call(values...)
		if err != nil {
			s.onError(err)
			return
		}

		if header.ID != nil {
			s.sendAck(*header.ID, ret)
		}
	}()
}

func (s *clientSocket) onAck(header *parser.PacketHeader, decode parser.Decode) {
	s.acksMu.Lock()
	handler, ok := s.acks[*header.ID]
	if ok {
		delete(s.acks, *header.ID)
	}
	s.acksMu.Unlock()

	if !ok {
		return
	}

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
		// TODO: Handle error?
		//return
	}

	go func(handler *ackHandler) {
		err := handler.Call(values...)
		if err != nil {
			s.onError(err)
			return
		}
	}(handler)
}

func (s *clientSocket) sendAck(id uint64, values []reflect.Value) {
	header := parser.PacketHeader{
		Type:      parser.PacketTypeAck,
		Namespace: s.namespace,
		ID:        &id,
	}

	v := make([]interface{}, len(values))

	for i := range values {
		if values[i].CanInterface() {
			v[i] = values[i].Interface()
		} else {
			// TODO: Handle error?
		}
	}

	buffers, err := s.parser.Encode(&header, &v)
	if err != nil {
		s.onError(err)
	}

	s.send(buffers...)
}

func (s *clientSocket) onError(err error) {
	s.client.onError(err)
}

func (s *clientSocket) On(eventName string, handler interface{}) {
	s.emitter.On(eventName, handler)
}

func (s *clientSocket) Once(eventName string, handler interface{}) {
	s.emitter.Once(eventName, handler)
}

func (s *clientSocket) Off(eventName string, handler interface{}) {
	s.emitter.Off(eventName, handler)
}

func (s *clientSocket) OffAll() {
	s.emitter.OffAll()
}

func (s *clientSocket) Emit(v ...interface{}) {
	header := parser.PacketHeader{
		Type:      parser.PacketTypeEvent,
		Namespace: s.namespace,
	}

	if len(v) == 0 {
		// TODO: Handle error
		return
	}

	eventName := reflect.ValueOf(v)
	if eventName.Kind() != reflect.String {
		// TODO: Handle error (string expected)
		return
	}

	isReserved, ok := reservedEvents[eventName.String()]
	if ok && isReserved {
		// TODO: Handle error
		return
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

func (s *clientSocket) send(buffers ...[]byte) {
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
