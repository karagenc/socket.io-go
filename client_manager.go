package sio

import (
	"sync"
	"time"

	eio "github.com/tomruk/socket.io-go/engine.io"
	eioparser "github.com/tomruk/socket.io-go/engine.io/parser"
	"github.com/tomruk/socket.io-go/parser"
	jsonparser "github.com/tomruk/socket.io-go/parser/json"
	"github.com/tomruk/socket.io-go/parser/json/serializer/stdjson"
)

type (
	ManagerConfig struct {
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

		// For debugging purposes. Leave it nil if it is of no use.
		//
		// This only applies to Socket.IO. For Engine.IO, use EIO.Debugger.
		Debugger Debugger
	}

	Manager struct {
		url       string
		eioConfig eio.ClientConfig
		debug     Debugger

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
		backoff *backoff
		conn    *clientConn

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
	} else {
		// User can modify the config. We copy the config here in order to avoid problems.
		config = &*config
	}

	io := &Manager{
		url:       url,
		eioConfig: config.EIO,

		noReconnection:       config.NoReconnection,
		reconnectionAttempts: config.ReconnectionAttempts,

		sockets: newClientSocketStore(),

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

	io.debug = io.debug.WithContext("[sio] Manager with URL: " + truncateURL(url))

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
	go m.open()
}

func (m *Manager) open() {
	m.debug.Log("Opening")
	err := m.conn.connect(false)
	if err != nil {
		m.conn.maybeReconnectOnOpen()
	}
}

func (m *Manager) Socket(namespace string, config *ClientSocketConfig) ClientSocket {
	if namespace == "" {
		namespace = "/"
	}
	if config == nil {
		config = new(ClientSocketConfig)
	} else {
		// User can modify the config. We copy the config here in order to avoid problems.
		config = &*config
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
			err := m.parser.Add(packet.Data, m.onFinishEIOPacket)
			if err != nil {
				m.onClose(ReasonParseError, err)
				return
			}

		case eioparser.PacketTypePing:
			handlers := m.pingHandlers.getAll()
			// Avoid unnecessary overhead of creating a goroutine.
			if len(handlers) > 0 {
				go func() {
					for _, handler := range handlers {
						(*handler)()
					}
				}()
			}
		}
	}
}

func (m *Manager) onFinishEIOPacket(header *parser.PacketHeader, eventName string, decode parser.Decode) {
	if header.Namespace == "" {
		header.Namespace = "/"
	}

	socket, ok := m.sockets.get(header.Namespace)
	if !ok {
		return
	}
	socket.onPacket(header, eventName, decode)
}

func (m *Manager) onEIOError(err error) {
	m.onError(err)
}

func (m *Manager) onEIOClose(reason eio.Reason, err error) {
	m.onClose(reason, err)
}

func (m *Manager) onError(err error) {
	handlers := m.errorHandlers.getAll()
	// Avoid unnecessary overhead of creating a goroutine.
	if len(handlers) > 0 {
		go func() {
			for _, handler := range handlers {
				(*handler)(err)
			}
		}()
	}
}

func (m *Manager) destroy(socket *clientSocket) {
	for _, socket := range m.sockets.getAll() {
		if socket.Active() {
			m.debug.Log("Socket with ID", socket.ID(), "still active, skipping close")
			return
		}
	}
	m.Close()
}

func (m *Manager) onClose(reason Reason, err error) {
	m.debug.Log("Closed. Reason", reason)

	m.parserMu.Lock()
	defer m.parserMu.Unlock()
	m.parser.Reset()
	m.backoff.reset()

	for _, handler := range m.closeHandlers.getAll() {
		(*handler)(reason, err)
	}

	m.skipReconnectMu.RLock()
	skipReconnect := m.skipReconnect
	m.skipReconnectMu.RUnlock()
	if !m.noReconnection && !skipReconnect {
		go m.conn.reconnect(false)
	}
}

func (m *Manager) Close() {
	m.conn.disconnect()
}

func truncateURL(url string) string {
	if len(url) > 50 {
		return url[:50] + "..."
	}
	return url
}
