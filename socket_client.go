package sio

import (
	"encoding/json"
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
	io        *Client
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

func newClientSocket(io *Client, namespace string, parser parser.Parser) *clientSocket {
	return &clientSocket{
		namespace: namespace,
		io:        io,
		parser:    parser,
		auth:      newAuth(),
		emitter:   newEventEmitter(),
		acks:      make(map[uint64]*ackHandler),
	}
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
		err := s.io.connect()
		if err != nil && s.io.noReconnection == false {
			go s.io.reconnect()
		}
	}
}

func (s *clientSocket) IO() *Client {
	return s.io
}

func (s *clientSocket) Auth() *Auth {
	return s.auth
}

func (s *clientSocket) Close() {}

func (s *clientSocket) sendConnectPacket() {
	header := parser.PacketHeader{
		Type:      parser.PacketTypeConnect,
		Namespace: s.namespace,
	}

	authData := s.auth.Get()

	var (
		buffers [][]byte
		err     error
	)

	if authData == nil {
		buffers, err = s.parser.Encode(&header, nil)
	} else {
		buffers, err = s.parser.Encode(&header, &authData)
	}

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
	s.io.packet(packet)
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

	connectError := func(err error) {
		errValue := reflect.ValueOf(fmt.Errorf("invalid CONNECT packet: %w: it seems you are trying to reach a Socket.IO server in v2.x with a v3.x client, but they are not compatible (more information here: https://socket.io/docs/v3/migrating-from-2-x-to-3-0/)", err))
		handlers := s.emitter.GetHandlers("connect_error")
		for _, handler := range handlers {
			go func(handler *eventHandler) {
				handler.Call(errValue)
			}(handler)
		}
	}

	var v *sidInfo
	vt := reflect.TypeOf(v)
	values, err := decode(vt)
	if err != nil {
		connectError(err)
		return
	} else if len(values) != 1 {
		connectError(fmt.Errorf("len(values) != 1"))
		return
	}

	v, ok := values[0].Interface().(*sidInfo)
	if !ok {
		connectError(fmt.Errorf("cast failed"))
		return
	}

	if v.SID == "" {
		connectError(fmt.Errorf("sid is empty"))
		return
	}

	s.setID(v.SID)

	s.connectedMu.Lock()
	s.connected = true
	s.connectedMu.Unlock()

	handlers := s.emitter.GetHandlers("connect")
	for _, handler := range handlers {
		go func(handler *eventHandler) {
			handler.Call()
		}(handler)
	}

	s.sendBufferMu.Lock()
	defer s.sendBufferMu.Unlock()
	if len(s.sendBuffer) != 0 {
		s.io.packet(s.sendBuffer...)
		s.sendBuffer = nil
	}
}

type connectError struct {
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (s *clientSocket) onConnectError(header *parser.PacketHeader, decode parser.Decode) {
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

	errValue := reflect.ValueOf(fmt.Errorf("%s", v.Message))

	handlers := s.emitter.GetHandlers("connect_error")
	for _, handler := range handlers {
		go func(handler *eventHandler) {
			handler.Call(errValue)
		}(handler)
	}
}

func (s *clientSocket) onDisconnect() {
	handlers := s.emitter.GetHandlers("disconnect")

	for _, handler := range handlers {
		go func(handler *eventHandler) {
			handler.Call()
		}(handler)
	}
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
		s.onError(fmt.Errorf("onEvent: invalid number of arguments"))
		return
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
		s.onError(fmt.Errorf("onAck: invalid number of arguments"))
		return
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
			s.onError(fmt.Errorf("sendAck: CanInterface must be true"))
			return
		}
	}

	buffers, err := s.parser.Encode(&header, &v)
	if err != nil {
		s.onError(err)
	}

	s.sendBuffers(buffers...)
}

func (s *clientSocket) onError(err error) {
	s.io.onError(err)
}

func (s *clientSocket) OnConnect(handler ConnectCallback) {
	s.emitter.On("connect", handler)
}

func (s *clientSocket) OnceConnect(handler ConnectCallback) {
	s.emitter.Once("connect", handler)
}

func (s *clientSocket) OffConnect(handler ConnectCallback) {
	s.emitter.Off("connect", handler)
}

func (s *clientSocket) OnConnectError(handler ConnectErrorCallback) {
	s.emitter.On("connect_error", handler)
}

func (s *clientSocket) OnceConnectError(handler ConnectErrorCallback) {
	s.emitter.Once("connect_error", handler)
}

func (s *clientSocket) OffConnectError(handler ConnectErrorCallback) {
	s.emitter.Off("connect_error", handler)
}

func (s *clientSocket) OnDisconnect(handler DisconnectCallback) {
	s.emitter.On("disconnect", handler)
}

func (s *clientSocket) OnceDisconnect(handler DisconnectCallback) {
	s.emitter.Once("disconnect", handler)
}

func (s *clientSocket) OffDisconnect(handler DisconnectCallback) {
	s.emitter.Off("disconnect", handler)
}

// This is server only. Do nothing.
func (s *clientSocket) OnDisconnecting(handler DisconnectingCallback) {}

// This is server only. Do nothing.
func (s *clientSocket) OnceDisconnecting(handler DisconnectingCallback) {}

// This is server only. Do nothing.
func (s *clientSocket) OffDisconnecting(handler DisconnectingCallback) {}

func (s *clientSocket) OnEvent(eventName string, handler interface{}) {
	if IsEventReserved(eventName) {
		panic(fmt.Errorf("OnEvent: attempted to attach a handler to a reserved event"))
	}

	s.emitter.On(eventName, handler)
}

func (s *clientSocket) OnceEvent(eventName string, handler interface{}) {
	if IsEventReserved(eventName) {
		panic(fmt.Errorf("OnceEvent: attempted to attach a handler to a reserved event"))
	}

	s.emitter.Once(eventName, handler)
}

func (s *clientSocket) OffEvent(eventName string, handler interface{}) {
	if IsEventReserved(eventName) {
		panic(fmt.Errorf("OffEvent: attempted to remove a handler from a reserved event"))
	}

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

	s.sendBuffers(buffers...)
}

func (s *clientSocket) sendBuffers(buffers ...[]byte) {
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
			s.io.packet(packets...)
		} else {
			s.sendBufferMu.Lock()
			s.sendBuffer = append(s.sendBuffer, packets...)
			s.sendBufferMu.Unlock()
		}
		s.connectedMu.Unlock()
	}
}
