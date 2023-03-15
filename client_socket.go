package sio

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fatih/structs"
	"github.com/tomruk/socket.io-go/adapter"
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

	sendBuffer   []sendBufferItem
	sendBufferMu sync.Mutex

	receiveBuffer   []*clientEvent
	receiveBufferMu sync.Mutex

	eventHandlers        *eventHandlerStore
	connectHandlers      *handlerStore[*ClientSocketConnectFunc]
	connectErrorHandlers *handlerStore[*ClientSocketConnectErrorFunc]
	disconnectHandlers   *handlerStore[*ClientSocketDisconnectFunc]

	acks   map[uint64]*ackHandler
	ackID  uint64
	acksMu sync.Mutex

	active   bool
	activeMu sync.Mutex

	packetQueue *clientPacketQueue

	debug Debugger
}

func newClientSocket(config *ClientSocketConfig, manager *Manager, namespace string, parser parser.Parser) *clientSocket {
	s := &clientSocket{
		config:    config,
		namespace: namespace,
		manager:   manager,
		parser:    parser,
		auth:      newAuth(),
		acks:      make(map[uint64]*ackHandler),

		eventHandlers:        newEventHandlerStore(),
		connectHandlers:      newHandlerStore[*ClientSocketConnectFunc](),
		connectErrorHandlers: newHandlerStore[*ClientSocketConnectErrorFunc](),
		disconnectHandlers:   newHandlerStore[*ClientSocketDisconnectFunc](),
	}
	s.debug = manager.debug.WithDynamicContext("clientSocket", func() string {
		return string(s.ID())
	})
	s.packetQueue = newClientPacketQueue(s)
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

func (s *clientSocket) pid() (pid adapter.PrivateSessionID, ok bool) {
	pid, ok = s._pid.Load().(adapter.PrivateSessionID)
	return
}

func (s *clientSocket) setPID(pid adapter.PrivateSessionID) {
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
	s.activeMu.Lock()
	defer s.activeMu.Unlock()
	return s.active
}

func (s *clientSocket) registerSubEvents() {
	var (
		open ManagerOpenFunc = func() {
			s.onOpen()
		}
		close ManagerCloseFunc = func(reason Reason, err error) {
			s.onClose(reason)
		}
		error ManagerErrorFunc = func(err error) {
			if !s.Connected() {
				for _, handler := range s.connectErrorHandlers.GetAll() {
					(*handler)(err)
				}
			}
		}
	)

	s.activeMu.Lock()
	s.active = true
	s.manager.openHandlers.OnSubEvent(&open)
	s.manager.errorHandlers.OnSubEvent(&error)
	s.manager.closeHandlers.OnSubEvent(&close)
	s.activeMu.Unlock()
}

func (s *clientSocket) deregisterSubEvents() {
	s.activeMu.Lock()
	s.active = false
	s.manager.openHandlers.OffSubEvents()
	s.manager.errorHandlers.OffSubEvents()
	s.manager.closeHandlers.OffSubEvents()
	s.activeMu.Unlock()
}

func (s *clientSocket) Connect() {
	s.connectedMu.RLock()
	connected := s.connected
	s.connectedMu.RUnlock()

	if connected {
		return
	}

	s.registerSubEvents()

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
		s.debug.Log("Performing disconnect", s.namespace)
		s.sendControlPacket(parser.PacketTypeDisconnect)
	}

	s.destroy()

	if s.Connected() {
		s.onClose(ReasonIOClientDisconnect)
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
	s.debug.Log("onOpen called. Connecting")
	authData := s.auth.Get()
	s.sendConnectPacket(authData)
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
		handlers := s.eventHandlers.GetAll(eventName)

		go func() {
			var (
				hasAckFunc bool // This doesn't need mutex

				mu   sync.Mutex
				sent bool
			)

			sendAck := func(ackID uint64, values []reflect.Value) {
				mu.Lock()
				if sent {
					mu.Unlock()
					return
				}
				sent = true
				mu.Unlock()

				s.debug.Log("Sending ack with ID", ackID)
				s.sendAckPacket(ackID, values)
			}

			for _, handler := range handlers {
				_hasAckFunc := s.onEvent(handler, header, decode, sendAck)
				if _hasAckFunc {
					hasAckFunc = true
				}
			}
			if header.ID != nil {
				mu.Lock()
				send := !hasAckFunc
				if sent {
					mu.Unlock()
					return
				}
				sent = true
				mu.Unlock()

				// If there is no acknowledgement function
				// and there is no response already sent,
				// then send an empty acknowledgement.
				if send {
					s.debug.Log("Sending ack with ID", *header.ID)
					s.sendAckPacket(*header.ID, nil)
				}
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
		for _, handler := range s.connectErrorHandlers.GetAll() {
			(*handler)(fmt.Errorf("sio: invalid CONNECT packet: %w: it seems you are trying to reach a Socket.IO server in v2.x with a v3.x client, but they are not compatible (more information here: https://socket.io/docs/v3/migrating-from-2-x-to-3-0/)", err))
		}
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
		if ok && pid == adapter.PrivateSessionID(*v.PID) {
			s.setRecovered(true)
		}
		s.setPID(adapter.PrivateSessionID(*v.PID))
	}

	s.setID(SocketID(v.SID))

	s.connectedMu.Lock()
	s.connected = true
	s.connectedMu.Unlock()

	s.debug.Log("Socket connected")

	s.emitBuffered()
	for _, handler := range s.connectHandlers.GetAll() {
		(*handler)()
	}
	s.packetQueue.drainQueue(true)
}

type sendBufferItem struct {
	AckID  *uint64
	Packet *eioparser.Packet
}

func (s *clientSocket) emitBuffered() {
	s.receiveBufferMu.Lock()
	defer s.receiveBufferMu.Unlock()

	var (
		// The reason we use this map is that we don't want to
		// send acknowledgements with same IDs.
		ackIDs = make(map[uint64]bool, len(s.receiveBuffer))
		mu     sync.Mutex
	)

	for _, event := range s.receiveBuffer {
		if event.header.ID != nil {
			ackIDs[*event.header.ID] = false
		}
	}

	for _, event := range s.receiveBuffer {
		sendAck := func(ackID uint64, values []reflect.Value) {
			mu.Lock()
			sent, ok := ackIDs[ackID]
			if ok && sent {
				mu.Unlock()
				return
			}
			ackIDs[ackID] = true
			mu.Unlock()

			s.debug.Log("Sending ack with ID", ackID)
			s.sendAckPacket(ackID, values)
		}

		hasAckFunc := s.callEvent(event.handler, event.header, event.values, sendAck)

		if event.header.ID != nil {
			mu.Lock()
			send := !hasAckFunc
			sent, ok := ackIDs[*event.header.ID]
			if ok && sent {
				mu.Unlock()
				return
			}
			ackIDs[*event.header.ID] = true
			mu.Unlock()

			// If there is no acknowledgement function
			// and there is no response already sent,
			// then send an empty acknowledgement.
			if send {
				s.debug.Log("Sending ack with ID", *event.header.ID)
				s.sendAckPacket(*event.header.ID, nil)
			}
		}
	}
	s.receiveBuffer = nil

	s.sendBufferMu.Lock()
	defer s.sendBufferMu.Unlock()
	if len(s.sendBuffer) != 0 {
		packets := make([]*eioparser.Packet, len(s.sendBuffer))
		for i := range packets {
			packets[i] = s.sendBuffer[i].Packet
		}
		s.manager.conn.Packet(packets...)
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

	for _, handler := range s.connectErrorHandlers.GetAll() {
		(*handler)(fmt.Errorf("sio: %s", v.Message))
	}
}

func (s *clientSocket) onDisconnect() {
	s.debug.Log("Server disconnect", s.namespace)
	s.destroy()
	s.onClose(ReasonIOServerDisconnect)
}

type clientEvent struct {
	handler *eventHandler
	header  *parser.PacketHeader
	values  []reflect.Value
}

type ackSendFunc = func(id uint64, values []reflect.Value)

func (s *clientSocket) onEvent(handler *eventHandler, header *parser.PacketHeader, decode parser.Decode, sendAck ackSendFunc) (hasAckFunc bool) {
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
		return s.callEvent(handler, header, values, sendAck)
	} else {
		s.receiveBufferMu.Lock()
		defer s.receiveBufferMu.Unlock()
		s.receiveBuffer = append(s.receiveBuffer, &clientEvent{
			handler: handler,
			header:  header,
			values:  values,
		})
	}
	return
}

func (s *clientSocket) callEvent(handler *eventHandler, header *parser.PacketHeader, values []reflect.Value, sendAck ackSendFunc) (hasAckFunc bool) {
	// Set the lastOffset before calling the handler.
	// An error can occur when the handler gets called,
	// and we can miss setting the lastOffset.
	_, ok := s.pid()
	if ok && len(values) > 0 && values[len(values)-1].Kind() == reflect.String {
		s.setLastOffset(values[len(values)-1].String())
		values = values[:len(values)-1] // Remove offset
	}

	ack, _ := handler.ack()
	if header.ID != nil && ack {
		hasAckFunc = true

		// We already know that the last value of the handler is an ack function
		// and it doesn't have a return value. So dismantle it, and create it with reflect.MakeFunc.
		f := values[len(values)-1]
		in, variadic := dismantleAckFunc(reflect.TypeOf(f))
		rt := reflect.FuncOf(in, nil, variadic)

		f = reflect.MakeFunc(rt, func(args []reflect.Value) (results []reflect.Value) {
			sendAck(*header.ID, args)
			return nil
		})
		values[len(values)-1] = f
	}

	_, err := handler.Call(values...)
	if err != nil {
		s.onError(wrapInternalError(err))
		return
	}
	return
}

func (s *clientSocket) onAck(header *parser.PacketHeader, decode parser.Decode) {
	if header.ID == nil {
		s.onError(wrapInternalError(fmt.Errorf("header.ID is nil")))
		return
	}

	s.debug.Log("Calling ack with ID", *header.ID)

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

func (s *clientSocket) onError(err error) {
	// In original socket.io, errors are emitted only on `Manager` (`Client` in this implementation).
	s.manager.onError(err)
}

// Called upon forced client/server side disconnections,
// this method ensures the `Client` (manager on original socket.io implementation)
// stops tracking us and that reconnections don't get triggered for this.
func (s *clientSocket) destroy() {
	s.deregisterSubEvents()
	s.manager.destroy(s)
}

func (s *clientSocket) onClose(reason Reason) {
	s.debug.Log("Going to close the socket. Reason", reason)
	s.connectedMu.Lock()
	s.connected = false
	s.connectedMu.Unlock()
	for _, handler := range s.disconnectHandlers.GetAll() {
		(*handler)(reason)
	}
}

func (s *clientSocket) Emit(eventName string, v ...any) {
	s.emit(eventName, 0, false, false, v...)
}

func (s *clientSocket) emit(eventName string, timeout time.Duration, volatile, fromQueue bool, v ...any) {
	header := parser.PacketHeader{
		Type:      parser.PacketTypeEvent,
		Namespace: s.namespace,
	}

	if IsEventReservedForClient(eventName) {
		panic("sio: Emit: attempted to emit a reserved event: `" + eventName + "`")
	}

	if eventName != "" {
		v = append([]any{eventName}, v...)
	}

	if s.config.Retries > 0 && !fromQueue && !volatile {
		s.packetQueue.addToQueue(&header, v)
		return
	}

	f := v[len(v)-1]
	rt := reflect.TypeOf(f)
	if rt.Kind() == reflect.Func {
		ackID := s.registerAckHandler(f, timeout)
		header.ID = &ackID
		v = v[:len(v)-1]
	}

	buffers, err := s.parser.Encode(&header, &v)
	if err != nil {
		s.onError(wrapInternalError(err))
		return
	}

	s.sendBuffers(volatile, header.ID, buffers...)
}

// 0 as the timeout argument means there is no timeout.
func (s *clientSocket) registerAckHandler(f any, timeout time.Duration) (id uint64) {
	if timeout == 0 && s.config.AckTimeout > 0 {
		timeout = s.config.AckTimeout
	}
	id = s.nextAckID()
	s.debug.Log("Registering ack with ID", id)
	if timeout == 0 {
		s.acksMu.Lock()
		h, err := newAckHandler(f, false)
		if err != nil {
			panic(err)
		}
		s.acks[id] = h
		s.acksMu.Unlock()
		return
	}

	err := doesAckHandlerHasAnError(f)
	if err != nil {
		panic(err)
	}

	h, err := newAckHandlerWithTimeout(f, timeout, func() {
		s.debug.Log("Timeout occured for ack with ID", id, "timeout", timeout)
		s.acksMu.Lock()
		delete(s.acks, id)
		s.acksMu.Unlock()

		remove := func(slice []sendBufferItem, s int) []sendBufferItem {
			return append(slice[:s], slice[s+1:]...)
		}

		s.sendBufferMu.Lock()
		for i, packet := range s.sendBuffer {
			if packet.AckID != nil && *packet.AckID == id {
				s.debug.Log("Removing packet with ack ID", id)
				s.sendBuffer = remove(s.sendBuffer, i)
			}
		}
		s.sendBufferMu.Unlock()
	})
	if err != nil {
		panic(err)
	}

	s.acksMu.Lock()
	s.acks[id] = h
	s.acksMu.Unlock()
	return
}

func (s *clientSocket) nextAckID() uint64 {
	s.acksMu.Lock()
	defer s.acksMu.Unlock()
	id := s.ackID
	s.ackID++
	return id
}

func (s *clientSocket) Timeout(timeout time.Duration) Emitter {
	return Emitter{
		socket:  s,
		timeout: timeout,
	}
}

func (s *clientSocket) Volatile() Emitter {
	return Emitter{
		socket:   s,
		volatile: true,
	}
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

	s.sendBuffers(false, nil, buffers...)
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

	s.sendBuffers(false, header.ID, buffers...)
}

func (s *clientSocket) sendBuffers(volatile bool, ackID *uint64, buffers ...[]byte) {
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
		} else if !volatile {
			s.sendBufferMu.Lock()
			buffers := make([]sendBufferItem, len(packets))
			for i := range buffers {
				buffers[i] = sendBufferItem{
					AckID:  ackID,
					Packet: packets[i],
				}
			}
			s.sendBuffer = append(s.sendBuffer, buffers...)
			s.sendBufferMu.Unlock()
		} else {
			s.debug.Log("Packet is discarded")
		}
	}
}
