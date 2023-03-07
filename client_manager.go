package sio

import (
	"fmt"
	"reflect"
	"sync"
	"time"

	eio "github.com/tomruk/socket.io-go/engine.io"
	eioparser "github.com/tomruk/socket.io-go/engine.io/parser"
	"github.com/tomruk/socket.io-go/parser"
	jsonparser "github.com/tomruk/socket.io-go/parser/json"
	"github.com/tomruk/socket.io-go/parser/json/serializer/stdjson"
)

type ManagerConfig struct {
	// A creator function for the Socket.IO parser.
	// This function is used for creating a parser.Parser object.
	// You can use a custom parser by changing this variable.
	//
	// By default this function is nil and default JSON parser is used.
	ParserCreator parser.Creator

	// Configuration for the Engine.IO.
	EIO eio.ClientConfig

	// Should we disallow reconnections?
	// Default: false (allow reconnections)
	NoReconnection bool

	// How many reconnection attempts should we try?
	// Default: 0 (Infinite)
	ReconnectionAttempts uint32

	// The time delay between reconnection attempts.
	// Default: 1 second
	ReconnectionDelay *time.Duration

	// The max time delay between reconnection attempts.
	// Default: 5 seconds
	ReconnectionDelayMax *time.Duration

	// Used in the exponential backoff jitter when reconnecting.
	// This value is required to be between 0 and 1
	//
	// Default: 0.5
	RandomizationFactor *float32
}

type Manager struct {
	url       string
	eioConfig eio.ClientConfig

	// This mutex is used for protecting parser from concurrent calls.
	// Due to the modular and concurrent nature of Engine.IO,
	// we should use a mutex to ensure that the Engine.IO doesn't access
	// the parser's Add method from multiple goroutines.
	parserMu sync.Mutex
	parser   parser.Parser

	noReconnection       bool
	reconnectionAttempts uint32
	reconnectionDelay    time.Duration
	reconnectionDelayMax time.Duration
	randomizationFactor  float32

	sockets *clientSocketStore
	emitter *eventEmitter
	backoff *backoff
	conn    *clientConn

	skipReconnect   bool
	skipReconnectMu sync.RWMutex
}

const (
	DefaultReconnectionDelay            = 1 * time.Second
	DefaultReconnectionDelayMax         = 5 * time.Second
	DefaultRandomizationFactor  float32 = 0.5
)

// This function creates a new Manager for client sockets.
//
// You should create a new Socket using the Socket
// method of the Manager returned by this function.
// If you don't do that, server will terminate your connection. See: https://socket.io/docs/v4/server-initialization/#connectTimeout
func NewManager(url string, config *ManagerConfig) *Manager {
	if config == nil {
		config = new(ManagerConfig)
	} else {
		c := *config
		config = &c
	}

	io := &Manager{
		url:       url,
		eioConfig: config.EIO,

		noReconnection:       config.NoReconnection,
		reconnectionAttempts: config.ReconnectionAttempts,

		sockets: newClientSocketStore(),
		emitter: newEventEmitter(),
	}

	if config.ReconnectionDelay != nil {
		io.reconnectionDelay = *config.ReconnectionDelay
	} else {
		io.reconnectionDelay = DefaultReconnectionDelay
	}

	if config.ReconnectionDelayMax != nil {
		io.reconnectionDelayMax = *config.ReconnectionDelayMax
	} else {
		io.reconnectionDelayMax = DefaultReconnectionDelayMax
	}

	if config.RandomizationFactor != nil {
		io.randomizationFactor = *config.RandomizationFactor
	} else {
		io.randomizationFactor = DefaultRandomizationFactor
	}

	io.backoff = newBackoff(io.reconnectionDelay, io.reconnectionDelayMax, io.randomizationFactor)

	parserCreator := config.ParserCreator
	if parserCreator == nil {
		json := stdjson.New()
		parserCreator = jsonparser.NewCreator(0, json)
	}
	io.parser = parserCreator()
	io.conn = newClientConn(io)
	return io
}

func (m *Manager) Open() {
	go func() {
		err := m.conn.Connect(false)
		if err != nil {
			m.conn.MaybeReconnectOnOpen()
		}
	}()
}

func (m *Manager) Socket(namespace string, config *ClientSocketConfig) ClientSocket {
	if namespace == "" {
		namespace = "/"
	}
	if config == nil {
		config = new(ClientSocketConfig)
	} else {
		// Copy config in order to prevent concurrency problems.
		// User can modify config.
		temp := *config
		config = &temp
	}

	socket, ok := m.sockets.Get(namespace)
	if !ok {
		socket = newClientSocket(config, m, namespace, m.parser)
		m.sockets.Set(socket)
	}
	return socket
}

func (m *Manager) On(eventName string, handler interface{}) {
	m.checkHandler(eventName, handler)
	m.emitter.On(eventName, handler)
}

func (m *Manager) Once(eventName string, handler interface{}) {
	m.checkHandler(eventName, handler)
	m.emitter.On(eventName, handler)
}

func (m *Manager) checkHandler(eventName string, handler interface{}) {
	switch eventName {
	case "":
		fallthrough
	case "open":
		fallthrough
	case "error":
		fallthrough
	case "ping":
		fallthrough
	case "close":
		fallthrough
	case "reconnect":
		fallthrough
	case "reconnect_attempt":
		fallthrough
	case "reconnect_error":
		fallthrough
	case "reconnect_failed":
		err := checkHandler(eventName, handler)
		if err != nil {
			panic(fmt.Errorf("sio: %w", err))
		}
	}
}

func (m *Manager) Off(eventName string, handler interface{}) {
	m.emitter.Off(eventName, handler)
}

func (m *Manager) OffAll() {
	m.emitter.OffAll()
}

func (m *Manager) onEIOPacket(packets ...*eioparser.Packet) {
	m.parserMu.Lock()
	defer m.parserMu.Unlock()

	for _, packet := range packets {
		switch packet.Type {
		case eioparser.PacketTypeMessage:
			err := m.parser.Add(packet.Data, m.onFinishEIOPacket)
			if err != nil {
				m.onError(wrapInternalError(err))
				return
			}

		case eioparser.PacketTypePing:
			m.emitReserved("ping")
		}
	}
}

func (m *Manager) onFinishEIOPacket(header *parser.PacketHeader, eventName string, decode parser.Decode) {
	if header.Namespace == "" {
		header.Namespace = "/"
	}

	socket, ok := m.sockets.Get(header.Namespace)
	if !ok {
		return
	}
	socket.onPacket(header, eventName, decode)
}

func (m *Manager) onEIOError(err error) {
	m.onError(err)
}

func (m *Manager) onEIOClose(reason string, err error) {
	m.onClose(reason, err)
}

// Convenience method for emitting events to the user.
func (m *Manager) emitReserved(eventName string, v ...interface{}) {
	// On original socket.io, events are also emitted to
	// subevents registered by the socket.
	//
	// https://github.com/socketio/socket.io-client/blob/89175d0481fc7633c12bb5b233dc3421f87860ef/lib/socket.ts#L287
	for _, socket := range m.sockets.GetAll() {
		socket.invokeSubEvents(eventName, v...)
	}

	handlers := m.emitter.GetHandlers(eventName)
	values := make([]reflect.Value, len(v))
	for i := range values {
		values[i] = reflect.ValueOf(v)
	}

	for _, handler := range handlers {
		_, err := handler.Call(values...)
		if err != nil {
			m.onError(wrapInternalError(fmt.Errorf("emitReserved: %s", err)))
			return
		}
	}
}

func (m *Manager) onError(err error) {
	// emitReserved is not used because if an error would happen in handler.Call
	// onError would be called recursively.

	errValue := reflect.ValueOf(err)

	handlers := m.emitter.GetHandlers("error")
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

func (m *Manager) destroy(socket *clientSocket) {
	for _, socket := range m.sockets.GetAll() {
		if socket.Active() {
			return
		}
	}
	m.Close()
}

func (m *Manager) onClose(reason string, err error) {
	m.parserMu.Lock()
	defer m.parserMu.Unlock()
	m.parser.Reset()
	m.backoff.Reset()
	m.emitReserved("close", reason, err)

	m.skipReconnectMu.RLock()
	skipReconnect := m.skipReconnect
	m.skipReconnectMu.RUnlock()
	if !m.noReconnection && !skipReconnect {
		go m.conn.Reconnect(false)
	}
}

func (m *Manager) Close() {
	m.conn.Disconnect()
}
