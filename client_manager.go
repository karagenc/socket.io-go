package sio

import (
	"time"

	"github.com/karagenc/socket.io-go/internal/sync"

	eio "github.com/karagenc/socket.io-go/engine.io"
	eioparser "github.com/karagenc/socket.io-go/engine.io/parser"
	"github.com/karagenc/socket.io-go/parser"
	jsonparser "github.com/karagenc/socket.io-go/parser/json"
	"github.com/karagenc/socket.io-go/parser/json/serializer/stdjson"
)

type (
	ManagerConfig struct {
		// For custom parsers
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

		// For debugging purposes. Leave it nil if it is of no use.
		//
		// This only applies to Socket.IO. For Engine.IO, use EIO.Debugger.
		Debugger Debugger
	}

	Manager struct {
		url       string
		eioConfig eio.ClientConfig
		debug     Debugger

		// Also a reconnectMu
		connectMu sync.Mutex

		state   clientConnectionState
		stateMu sync.RWMutex

		eio            eio.ClientSocket
		eioPacketQueue *packetQueue
		eioMu          sync.RWMutex

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
		// For testing purposes
		onNewSocket func(socket *clientSocket)

		backoff         *backoff
		skipReconnect   bool
		skipReconnectMu sync.RWMutex

		openHandlers             *handlerStore[*ManagerOpenFunc]
		pingHandlers             *handlerStore[*ManagerPingFunc]
		errorHandlers            *handlerStore[*ManagerErrorFunc]
		closeHandlers            *handlerStore[*ManagerCloseFunc]
		reconnectHandlers        *handlerStore[*ManagerReconnectFunc]
		reconnectAttemptHandlers *handlerStore[*ManagerReconnectAttemptFunc]
		reconnectErrorHandlers   *handlerStore[*ManagerReconnectErrorFunc]
		reconnectFailedHandlers  *handlerStore[*ManagerReconnectFailedFunc]

		// Callbacks to destroy subs
		subs   []func()
		subsMu sync.Mutex
	}
)

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
	}

	io := &Manager{
		url:       url,
		eioConfig: config.EIO,

		eioPacketQueue: newPacketQueue(),

		noReconnection:       config.NoReconnection,
		reconnectionAttempts: config.ReconnectionAttempts,

		sockets:     newClientSocketStore(),
		onNewSocket: func(socket *clientSocket) {}, // Noop by default.

		openHandlers:             newHandlerStore[*ManagerOpenFunc](),
		pingHandlers:             newHandlerStore[*ManagerPingFunc](),
		errorHandlers:            newHandlerStore[*ManagerErrorFunc](),
		closeHandlers:            newHandlerStore[*ManagerCloseFunc](),
		reconnectHandlers:        newHandlerStore[*ManagerReconnectFunc](),
		reconnectAttemptHandlers: newHandlerStore[*ManagerReconnectAttemptFunc](),
		reconnectErrorHandlers:   newHandlerStore[*ManagerReconnectErrorFunc](),
		reconnectFailedHandlers:  newHandlerStore[*ManagerReconnectFailedFunc](),
	}

	if config.Debugger != nil {
		io.debug = config.Debugger
	} else {
		io.debug = newNoopDebugger()
	}
	io.debug = io.debug.WithContext("[sio/client] Manager with URL: " + truncateURL(url))

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
	return io
}

func (m *Manager) Socket(namespace string, config *ClientSocketConfig) ClientSocket {
	socket := m.socket(namespace, config)
	m.onNewSocket(socket)
	return socket
}

func (m *Manager) socket(namespace string, config *ClientSocketConfig) *clientSocket {
	if namespace == "" {
		namespace = "/"
	} else if len(namespace) > 0 && namespace[0] != '/' {
		namespace = "/" + namespace
	}

	if config == nil {
		config = new(ClientSocketConfig)
	} else {
		// User can modify the config. We copy the config here in order to avoid problems.
		configCopy := *config
		config = &configCopy
	}

	socket, ok := m.sockets.get(namespace)
	if !ok {
		socket = newClientSocket(config, m, namespace, m.parser)
		m.sockets.set(socket)
	}
	return socket
}

func (m *Manager) onEIOPacket(packets ...*eioparser.Packet) {
	m.parserMu.Lock()
	defer m.parserMu.Unlock()

	for _, packet := range packets {
		switch packet.Type {
		case eioparser.PacketTypeMessage:
			err := m.parser.Add(packet.Data, m.onParserFinish)
			if err != nil {
				go m.onClose(ReasonParseError, err)
				return
			}
		case eioparser.PacketTypePing:
			m.pingHandlers.forEach(func(handler *ManagerPingFunc) { (*handler)() }, true)
		}
	}
}

func (m *Manager) onParserFinish(header *parser.PacketHeader, eventName string, decode parser.Decode) {
	if header.Namespace == "" {
		header.Namespace = "/"
	}

	socket, ok := m.sockets.get(header.Namespace)
	if !ok {
		return
	}
	go socket.onPacket(header, eventName, decode)
}

func (m *Manager) packet(packets ...*eioparser.Packet) {
	m.eioMu.RLock()
	defer m.eioMu.RUnlock()
	m.eioPacketQueue.add(packets...)
}

func (m *Manager) onError(err error) {
	m.errorHandlers.forEach(func(handler *ManagerErrorFunc) { (*handler)(err) }, true)
}

func (m *Manager) Open() {
	go m.open()
}

func (m *Manager) open() {
	m.debug.Log("Opening")
	err := m.connect(false)
	if err != nil {
		m.cleanup()
		m.maybeReconnectOnOpen()
	}
}

func (m *Manager) maybeReconnectOnOpen() {
	reconnect := m.backoff.attempts() == 0 && !m.noReconnection
	if reconnect {
		m.reconnect(false)
	}
}

func (m *Manager) onReconnect() {
	attempts := m.backoff.attempts()
	m.backoff.reset()
	m.reconnectHandlers.forEach(func(handler *ManagerReconnectFunc) { (*handler)(attempts) }, true)
}

func (m *Manager) closePacketQueue(pq *packetQueue) {
	go func() {
		pq.waitForDrain(2 * time.Minute)
		pq.close()
	}()
}

func (m *Manager) destroy(_ *clientSocket) {
	for _, socket := range m.sockets.getAll() {
		if socket.Active() {
			m.debug.Log("Socket (nsp: `" + socket.namespace + "`) is still active, skipping close")
			return
		}
	}
	m.Close()
}

func (m *Manager) cleanup() {
	m.subsMu.Lock()
	subs := m.subs
	m.subs = nil
	m.subsMu.Unlock()
	for _, sub := range subs {
		sub()
	}
	m.resetParser()
}

func (m *Manager) onClose(reason Reason, err error) {
	m.debug.Log("Closed. Reason", reason)

	m.cleanup()
	m.backoff.reset()

	m.stateMu.Lock()
	m.state = clientConnStateDisconnected
	m.stateMu.Unlock()

	m.closeHandlers.forEach(func(handler *ManagerCloseFunc) { (*handler)(reason, err) }, true)

	m.skipReconnectMu.RLock()
	skipReconnect := m.skipReconnect
	m.skipReconnectMu.RUnlock()
	if !m.noReconnection && !skipReconnect {
		go m.reconnect(false)
	}
}

func (m *Manager) Close() {
	m.debug.Log("Disconnecting")

	m.stateMu.Lock()
	m.state = clientConnStateDisconnected
	m.stateMu.Unlock()

	m.skipReconnectMu.Lock()
	m.skipReconnect = true
	m.skipReconnectMu.Unlock()

	m.onClose(ReasonForcedClose, nil)

	m.eioMu.RLock()
	defer m.eioMu.RUnlock()
	eio := m.eio
	if eio != nil {
		go eio.Close()
	}
	m.closePacketQueue(m.eioPacketQueue)
}

func (m *Manager) resetParser() {
	m.parserMu.Lock()
	defer m.parserMu.Unlock()
	m.parser.Reset()
}

func truncateURL(url string) string {
	if len(url) > 50 {
		return url[:50] + "..."
	}
	return url
}
