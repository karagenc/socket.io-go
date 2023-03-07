package sio

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fatih/structs"
	eioparser "github.com/tomruk/socket.io-go/engine.io/parser"
	"github.com/tomruk/socket.io-go/parser"
)

type ClientSocketConfig struct {
	// Authentication data.
	//
	// This can also be set/overridden using Socket.SetAuth method.
	Auth any

	// The maximum number of retries for the packet to be sent.
	// Above the limit, the packet will be discarded.
	//
	// Using `Infinity` means the delivery guarantee is
	// "at-least-once" (instead of "at-most-once" by default),
	// but a smaller value like 10 should be sufficient in practice.
	Retries int

	// The default timeout used when waiting for an acknowledgement.
	AckTimeout time.Duration
}

type clientSocket struct {
	id atomic.Value

	_pid        atomic.Value
	_lastOffset atomic.Value
	_recovered  atomic.Value

	config    *ClientSocketConfig
	namespace string
	manager   *Manager
	parser    parser.Parser

	auth *Auth

	connected   bool
	connectedMu sync.RWMutex

	sendBuffer   []*eioparser.Packet
	sendBufferMu sync.Mutex

	receiveBuffer   []*clientEvent
	receiveBufferMu sync.Mutex

	emitterForEvents *eventEmitter[*eventHandler]

	acks   map[uint64]*ackHandler
	ackID  uint64
	acksMu sync.Mutex

	subEventsEnabled   bool
	subEventsEnabledMu sync.Mutex

	packetQueue *clientPacketQueue
}

func newClientSocket(config *ClientSocketConfig, manager *Manager, namespace string, parser parser.Parser) *clientSocket {
	s := &clientSocket{
		config:           config,
		namespace:        namespace,
		manager:          manager,
		parser:           parser,
		auth:             newAuth(),
		emitterForEvents: newEventEmitter[*eventHandler](),
		acks:             make(map[uint64]*ackHandler),
		packetQueue:      newClientPacketQueue(),
	}
	s.setRecovered(false)
	s.SetAuth(config.Auth)
	return s
}

func (s *clientSocket) ID() SocketID {
	id, _ := s.id.Load().(SocketID)
	return id
}

func (s *clientSocket) setID(id SocketID) {
	s.id.Store(id)
}

func (s *clientSocket) pid() (pid PrivateSessionID, ok bool) {
	pid, ok = s._pid.Load().(PrivateSessionID)
	return
}

func (s *clientSocket) setPID(pid PrivateSessionID) {
	s._pid.Store(pid)
}

func (s *clientSocket) lastOffset() (lastOffset string, ok bool) {
	lastOffset, ok = s._lastOffset.Load().(string)
	return
}

func (s *clientSocket) setLastOffset(lastOffset string) {
	s._lastOffset.Store(lastOffset)
}

func (s *clientSocket) Recovered() bool {
	recovered, _ := s._recovered.Load().(bool)
	return recovered
}

func (s *clientSocket) setRecovered(recovered bool) {
	s._recovered.Store(recovered)
}

func (s *clientSocket) Connected() bool {
	s.connectedMu.RLock()
	defer s.connectedMu.RUnlock()
	return s.manager.conn.Connected() && s.connected
}

// Whether the socket will try to reconnect when its Client (manager) connects or reconnects.
func (s *clientSocket) Active() bool {
	s.subEventsEnabledMu.Lock()
	defer s.subEventsEnabledMu.Unlock()
	return s.subEventsEnabled
}

func (s *clientSocket) Connect() {
	s.connectedMu.Lock()
	connected := s.connected
	s.connectedMu.Unlock()

	if connected {
		return
	}

	s.subEventsEnabledMu.Lock()
	s.subEventsEnabled = true
	s.subEventsEnabledMu.Unlock()

	go func() {
		s.manager.conn.stateMu.RLock()
		isReconnecting := s.manager.conn.state == clientConnStateReconnecting
		s.manager.conn.stateMu.RUnlock()
		if !isReconnecting {
			s.manager.Open()
		}

		s.manager.conn.stateMu.RLock()
		connected := s.manager.conn.state == clientConnStateConnected
		s.manager.conn.stateMu.RUnlock()
		if connected {
			s.onOpen()
		}
	}()
}

func (s *clientSocket) Disconnect() {
	if s.Connected() {
		s.sendControlPacket(parser.PacketTypeDisconnect)
	}

	s.destroy()

	if s.Connected() {
		s.manager.onClose("io client disconnect", nil)
	}
}

func (s *clientSocket) Manager() *Manager { return s.manager }

func (s *clientSocket) Auth() any { return s.auth.Get() }

func (s *clientSocket) SetAuth(v any) {
	err := s.auth.Set(v)
	if err != nil {
		panic(fmt.Errorf("sio: %w", err))
	}
}

func (s *clientSocket) onOpen() {
	authData := s.auth.Get()
	s.sendConnectPacket(authData)
}

func (s *clientSocket) invokeSubEvents(eventName string, v ...any) {
	s.subEventsEnabledMu.Lock()
	defer s.subEventsEnabledMu.Unlock()
	if !s.subEventsEnabled {
		return
	}

	switch eventName {
	case "open":
		s.onOpen()
	case "error":
		if len(v) != 1 {
			panic("sio: 1 argument was expected: err")
		}
		err, ok := v[0].(error)
		if !ok {
			panic("sio: type of the argument `err` must be error")
		}
		if !s.Connected() {
			s.emitReserved("connect_error", err)
		}
	case "close":
		if len(v) != 2 {
			panic("sio: 2 arguments were expected: reason and err")
		}
		reason, ok := v[0].(string)
		if !ok {
			panic("sio: type of the argument `reason` must be string")
		}
		s.onClose(reason)
	}
}

func (s *clientSocket) sendConnectPacket(authData any) {
	header := parser.PacketHeader{
		Type:      parser.PacketTypeConnect,
		Namespace: s.namespace,
	}

	var (
		buffers [][]byte
		err     error
	)

	pid, ok := s.pid()
	if ok {
		m := make(map[string]any)
		m["pid"] = pid
		lastOffset, _ := s.lastOffset()
		m["offset"] = lastOffset

		if authData != nil {
			a := structs.New(&authData)
			a.TagName = "json"
			for k, v := range a.Map() {
				m[k] = v
			}
		}

		buffers, err = s.parser.Encode(&header, m)
	} else {
		if authData == nil {
			buffers, err = s.parser.Encode(&header, nil)
		} else {
			buffers, err = s.parser.Encode(&header, &authData)
		}
	}

	if err != nil {
		s.onError(err)
		return
	} else if len(buffers) != 1 {
		s.onError(wrapInternalError(fmt.Errorf("onEIOConnect: len(buffers) != 1")))
		return
	}

	buf := buffers[0]

	packet, err := eioparser.NewPacket(eioparser.PacketTypeMessage, false, buf)
	if err != nil {
		s.onError(wrapInternalError(err))
		return
	}
	s.manager.conn.Packet(packet)
}

func (s *clientSocket) onPacket(header *parser.PacketHeader, eventName string, decode parser.Decode) {
	switch header.Type {
	case parser.PacketTypeConnect:
		s.onConnect(header, decode)

	case parser.PacketTypeEvent, parser.PacketTypeBinaryEvent:
		handlers := s.emitterForEvents.GetHandlers(eventName)

		go func() {
			for _, handler := range handlers {
				s.onEvent(handler, header, decode)
			}
		}()
	case parser.PacketTypeAck, parser.PacketTypeBinaryAck:
		go s.onAck(header, decode)

	case parser.PacketTypeConnectError:
		s.onConnectError(header, decode)

	case parser.PacketTypeDisconnect:
		s.onDisconnect()
	}
}

func (s *clientSocket) onConnect(header *parser.PacketHeader, decode parser.Decode) {
	connectError := func(err error) {
		s.emitReserved("connect_error", fmt.Errorf("sio: invalid CONNECT packet: %w: it seems you are trying to reach a Socket.IO server in v2.x with a v3.x client, but they are not compatible (more information here: https://socket.io/docs/v3/migrating-from-2-x-to-3-0/)", err))
	}

	var v *sidInfo
	vt := reflect.TypeOf(v)
	values, err := decode(vt)
	if err != nil {
		connectError(err)
		return
	} else if len(values) != 1 {
		connectError(wrapInternalError(fmt.Errorf("len(values) != 1")))
		return
	}

	v, ok := values[0].Interface().(*sidInfo)
	if !ok {
		connectError(wrapInternalError(fmt.Errorf("cast failed")))
		return
	}

	if v.SID == "" {
		connectError(wrapInternalError(fmt.Errorf("sid is empty")))
		return
	}

	if v.PID != nil {
		pid, ok := s.pid()
		if ok && pid == PrivateSessionID(*v.PID) {
			s.setRecovered(true)
		}
		s.setPID(PrivateSessionID(*v.PID))
	}

	s.setID(SocketID(v.SID))

	s.connectedMu.Lock()
	s.connected = true
	s.connectedMu.Unlock()

	s.emitBuffered()
	s.emitReserved("connect")
}

func (s *clientSocket) emitBuffered() {
	s.receiveBufferMu.Lock()
	defer s.receiveBufferMu.Unlock()
	for _, event := range s.receiveBuffer {
		s.callEvent(event.handler, event.header, event.values)
	}
	s.receiveBuffer = nil

	s.sendBufferMu.Lock()
	defer s.sendBufferMu.Unlock()
	if len(s.sendBuffer) != 0 {
		s.manager.conn.Packet(s.sendBuffer...)
		s.sendBuffer = nil
	}
}

type connectError struct {
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (s *clientSocket) onConnectError(header *parser.PacketHeader, decode parser.Decode) {
	s.destroy()

	var v *connectError
	vt := reflect.TypeOf(v)
	values, err := decode(vt)
	if err != nil {
		s.onError(wrapInternalError(err))
		return
	} else if len(values) != 1 {
		s.onError(wrapInternalError(fmt.Errorf("invalid CONNECT_ERROR packet")))
		return
	}

	v, ok := values[0].Interface().(*connectError)
	if !ok {
		s.onError(wrapInternalError(fmt.Errorf("invalid CONNECT_ERROR packet: cast failed")))
		return
	}

	s.emitReserved("connect_error", fmt.Errorf("sio: %s", v.Message))
}

func (s *clientSocket) onDisconnect() {
	s.destroy()
	s.onClose("io server disconnect")
}

type clientEvent struct {
	handler *eventHandler
	header  *parser.PacketHeader
	values  []reflect.Value
}

func (s *clientSocket) onEvent(handler *eventHandler, header *parser.PacketHeader, decode parser.Decode) {
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

	s.connectedMu.RLock()
	defer s.connectedMu.RUnlock()
	if s.connected {
		s.callEvent(handler, header, values)
	} else {
		s.receiveBufferMu.Lock()
		defer s.receiveBufferMu.Unlock()
		s.receiveBuffer = append(s.receiveBuffer, &clientEvent{
			handler: handler,
			header:  header,
			values:  values,
		})
	}
}

func (s *clientSocket) callEvent(handler *eventHandler, header *parser.PacketHeader, values []reflect.Value) {
	// Set the lastOffset before calling the handler.
	// An error can occur when the handler gets called,
	// and we can miss setting the lastOffset.
	_, ok := s.pid()
	if ok && len(values) > 0 && values[len(values)-1].Kind() == reflect.String {
		s.setLastOffset(values[len(values)-1].String())
		values = values[:len(values)-1] // Remove offset
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

func (s *clientSocket) onAck(header *parser.PacketHeader, decode parser.Decode) {
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
		s.onError(fmt.Errorf("sio: onAck: invalid number of arguments"))
		return
	}

	err = ack.Call(values...)
	if err != nil {
		s.onError(wrapInternalError(err))
		return
	}
}

// Convenience method for emitting events to the user.
func (s *clientSocket) emitReserved(eventName string, v ...any) {
	handlers := s.emitterForEvents.GetHandlers(eventName)
	values := make([]reflect.Value, len(v))
	for i := range values {
		values[i] = reflect.ValueOf(v)
	}

	for _, handler := range handlers {
		_, err := handler.Call(values...)
		if err != nil {
			s.onError(wrapInternalError(fmt.Errorf("emitReserved: %s", err)))
			return
		}
	}
}

func (s *clientSocket) onError(err error) {
	// In original socket.io, errors are emitted only on `Manager` (`Client` in this implementation).
	s.manager.onError(err)
}

// Called upon forced client/server side disconnections,
// this method ensures the `Client` (manager on original socket.io implementation)
// stops tracking us and that reconnections don't get triggered for this.
func (s *clientSocket) destroy() {
	s.subEventsEnabledMu.Lock()
	s.subEventsEnabled = false
	s.subEventsEnabledMu.Unlock()
	s.manager.destroy(s)
}

func (s *clientSocket) onClose(reason string) {
	s.connectedMu.Lock()
	s.connected = false
	s.connectedMu.Unlock()
	s.emitReserved("disconnect")
}

func (s *clientSocket) OnEvent(eventName string, handler any) {
	s.checkHandler(eventName, handler)
	s.emitterForEvents.On(eventName, newEventHandler(handler))
}

func (s *clientSocket) OnceEvent(eventName string, handler any) {
	s.checkHandler(eventName, handler)
	s.emitterForEvents.Once(eventName, newEventHandler(handler))
}

func (s *clientSocket) checkHandler(eventName string, handler any) {
	switch eventName {
	case "":
		fallthrough
	case "connect":
		fallthrough
	case "connect_error":
		fallthrough
	case "disconnect":
		err := checkHandler(eventName, handler)
		if err != nil {
			panic(fmt.Errorf("sio: %w", err))
		}
	}
}

func (s *clientSocket) OffEvent(eventName string, _handler ...any) {
	handlers := make([]*eventHandler, len(_handler))
	for i, h := range _handler {
		handlers[i] = newEventHandler(h)
	}
	s.emitterForEvents.Off(eventName, handlers...)
}

func (s *clientSocket) OffAll() {
	s.emitterForEvents.OffAll()
}

func (s *clientSocket) Emit(eventName string, v ...any) {
	header := parser.PacketHeader{
		Type:      parser.PacketTypeEvent,
		Namespace: s.namespace,
	}

	if IsEventReservedForClient(eventName) {
		panic("sio: Emit: attempted to emit a reserved event: `" + eventName + "`")
	}

	v = append([]any{eventName}, v...)

	s.packetQueue.addToQueue(&header, v)

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
		s.onError(wrapInternalError(err))
		return
	}

	s.sendBuffers(buffers...)
}

func (s *clientSocket) sendControlPacket(typ parser.PacketType, v ...any) {
	header := parser.PacketHeader{
		Type:      typ,
		Namespace: s.namespace,
	}

	buffers, err := s.parser.Encode(&header, &v)
	if err != nil {
		s.onError(wrapInternalError(err))
		return
	}

	s.sendBuffers(buffers...)
}

func (s *clientSocket) sendAckPacket(id uint64, values []reflect.Value) {
	header := parser.PacketHeader{
		Type:      parser.PacketTypeAck,
		Namespace: s.namespace,
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
			s.onError(wrapInternalError(err))
			return
		}

		for i, attachment := range buffers {
			packets[i+1], err = eioparser.NewPacket(eioparser.PacketTypeMessage, true, attachment)
			if err != nil {
				s.onError(wrapInternalError(err))
				return
			}
		}

		s.connectedMu.Lock()
		defer s.connectedMu.Unlock()
		if s.connected {
			s.manager.conn.Packet(packets...)
		} else {
			s.sendBufferMu.Lock()
			s.sendBuffer = append(s.sendBuffer, packets...)
			s.sendBufferMu.Unlock()
		}
	}
}
