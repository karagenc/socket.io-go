package sio

import (
	"net/http"
	"time"

	"github.com/karagenc/socket.io-go/adapter"
	eio "github.com/karagenc/socket.io-go/engine.io"
	"github.com/karagenc/socket.io-go/parser"
	jsonparser "github.com/karagenc/socket.io-go/parser/json"
	"github.com/karagenc/socket.io-go/parser/json/serializer/stdjson"
)

const (
	DefaultConnectTimeout           = time.Second * 45
	DefaultMaxDisconnectionDuration = time.Minute * 2
)

type BroadcastOperator = adapter.BroadcastOperator

type (
	ServerConfig struct {
		// For custom parsers
		ParserCreator parser.Creator
		// For custom adapters
		AdapterCreator adapter.Creator

		// Engine.IO configuration
		EIO eio.ServerConfig

		// Duration to wait before a client without namespace is closed.
		//
		// Default: 45 seconds
		ConnectTimeout time.Duration

		// In order for a client to make a connection to a namespace,
		// the namespace must be created on server via `Server.of`.
		//
		// This option permits the client to create the namespace if it is not already created on server.
		// If this option is disabled, only namespaces created on the server can be connected.
		//
		// Default: false
		AcceptAnyNamespace bool

		ServerConnectionStateRecovery ServerConnectionStateRecovery

		// For debugging purposes. Leave it nil if it is of no use.
		//
		// This only applies to Socket.IO. For Engine.IO, use EIO.Debugger.
		Debugger Debugger
	}

	ServerConnectionStateRecovery struct {
		// Enable connection state recovery
		//
		// Default: false
		Enabled bool

		// The backup duration of the sessions and the packets
		//
		// Default: 2 minutes
		MaxDisconnectionDuration time.Duration

		// Whether to execute middlewares upon successful connection state recovery.
		//
		// Default: false
		UseMiddlewares bool
	}

	Server struct {
		parserCreator  parser.Creator
		adapterCreator adapter.Creator

		eio        *eio.Server
		namespaces *nspStore

		connectTimeout     time.Duration
		acceptAnyNamespace bool

		connectionStateRecovery ServerConnectionStateRecovery

		debug Debugger

		newNamespaceHandlers  *handlerStore[*ServerNewNamespaceFunc]
		anyConnectionHandlers *handlerStore[*ServerAnyConnectionFunc]
	}
)

func NewServer(config *ServerConfig) *Server {
	if config == nil {
		config = new(ServerConfig)
	}

	server := &Server{
		parserCreator:           config.ParserCreator,
		adapterCreator:          config.AdapterCreator,
		namespaces:              newNspStore(),
		acceptAnyNamespace:      config.AcceptAnyNamespace,
		connectionStateRecovery: config.ServerConnectionStateRecovery,
		newNamespaceHandlers:    newHandlerStore[*ServerNewNamespaceFunc](),
		anyConnectionHandlers:   newHandlerStore[*ServerAnyConnectionFunc](),
	}

	if config.Debugger != nil {
		server.debug = config.Debugger
	} else {
		server.debug = newNoopDebugger()
	}
	server.debug = server.debug.WithContext("[sio/server] Server")

	server.eio = eio.NewServer(server.onEIOSocket, &config.EIO)

	if server.parserCreator == nil {
		json := stdjson.New()
		server.parserCreator = jsonparser.NewCreator(0, json)
	}

	if server.connectionStateRecovery.Enabled {
		if server.connectionStateRecovery.MaxDisconnectionDuration == 0 {
			server.connectionStateRecovery.MaxDisconnectionDuration = DefaultMaxDisconnectionDuration
		}
		if server.adapterCreator == nil {
			server.adapterCreator = adapter.NewSessionAwareAdapterCreator(server.connectionStateRecovery.MaxDisconnectionDuration)
		}
	} else {
		if server.adapterCreator == nil {
			server.adapterCreator = adapter.NewInMemoryAdapterCreator()
		}
	}

	if config.ConnectTimeout != 0 {
		server.connectTimeout = config.ConnectTimeout
	} else {
		server.connectTimeout = DefaultConnectTimeout
	}

	return server
}

func (s *Server) onEIOSocket(eioSocket eio.ServerSocket) *eio.Callbacks {
	_, callbacks := newServerConn(s, eioSocket, s.parserCreator)
	return callbacks
}

func (s *Server) Of(namespace string) *Namespace {
	if len(namespace) == 0 || (len(namespace) != 0 && namespace[0] != '/') {
		namespace = "/" + namespace
	}
	n, created := s.namespaces.getOrCreate(namespace, s, s.adapterCreator, s.parserCreator)
	if created && namespace != "/" {
		s.newNamespaceHandlers.forEach(func(handler *ServerNewNamespaceFunc) { (*handler)(n) }, true)
	}
	return n
}

// Alias of: s.Of("/").Use(...)
func (s *Server) Use(f NspMiddlewareFunc) {
	s.Of("/").Use(f)
}

// Alias of: s.Of("/").OnConnection(...)
func (s *Server) OnConnection(f NamespaceConnectionFunc) {
	s.Of("/").OnConnection(f)
}

// Alias of: s.Of("/").OnceConnection(...)
func (s *Server) OnceConnection(f NamespaceConnectionFunc) {
	s.Of("/").OnceConnection(f)
}

// Alias of: s.Of("/").OffConnection(...)
func (s *Server) OffConnection(f ...NamespaceConnectionFunc) {
	s.Of("/").OffConnection(f...)
}

// Emits an event to all connected clients in the given namespace.
//
// Alias of: s.Of("/").Emit(...)
func (s *Server) Emit(eventName string, v ...any) {
	s.Of("/").Emit(eventName, v...)
}

// Sets a modifier for a subsequent event emission that the event
// will only be broadcast to clients that have joined the given room.
//
// To emit to multiple rooms, you can call `To` several times.
//
// Alias of: s.Of("/").To(...)
func (s *Server) To(room ...Room) *BroadcastOperator {
	return s.Of("/").To(room...)
}

// Alias of: s.Of("/").In(...)
func (s *Server) In(room ...Room) *BroadcastOperator {
	return s.Of("/").In(room...)
}

// Sets a modifier for a subsequent event emission that the event
// will only be broadcast to clients that have not joined the given rooms.
//
// Alias of: s.Of("/").To(...)
func (s *Server) Except(room ...Room) *BroadcastOperator {
	return s.Of("/").Except(room...)
}

// Compression flag is unused at the moment, thus setting this will have no effect on compression.
//
// Alias of: s.Of("/").Compress(...)
func (s *Server) Compress(compress bool) *BroadcastOperator {
	return s.Of("/").Compress(compress)
}

// Sets a modifier for a subsequent event emission that the event data will only be broadcast to the current node (when scaling to multiple nodes).
//
// See: https://socket.io/docs/v4/using-multiple-nodes
//
// Alias of: s.Of("/").Local(...)
func (s *Server) Local() *BroadcastOperator {
	return s.Of("/").Local()
}

// Gets the sockets of the namespace.
// Beware that this is local to the current node. For sockets across all nodes, use FetchSockets
//
// Alias of: s.Of("/").Sockets(...)
func (s *Server) Sockets() []ServerSocket {
	return s.Of("/").Sockets()
}

// Returns the matching socket instances. This method works across a cluster of several Socket.IO servers.
//
// Alias of: s.Of("/").FetchSockets(...)
func (s *Server) FetchSockets(room ...string) []adapter.Socket {
	return s.Of("/").FetchSockets()
}

// Makes the matching socket instances leave the specified rooms.
//
// Alias of: s.Of("/").SocketsJoin(...)
func (s *Server) SocketsJoin(room ...Room) {
	s.Of("/").SocketsJoin(room...)
}

// Makes the matching socket instances leave the specified rooms.
//
// Alias of: s.Of("/").SocketsLeave(...)
func (s *Server) SocketsLeave(room ...Room) {
	s.Of("/").SocketsLeave(room...)
}

// Makes the matching socket instances disconnect from the namespace.
//
// If value of close is true, closes the underlying connection. Otherwise, it just disconnects the namespace.
//
// Alias of: s.Of("/").DisconnectSockets(...)
func (s *Server) DisconnectSockets(close bool) {
	s.Of("/").DisconnectSockets(close)
}

// Sends a message to the other Socket.IO servers of the cluster.
func (s *Server) ServerSideEmit(eventName string, v ...any) {
	s.Of("/").ServerSideEmit(eventName, v...)
}

// Start the server.
func (s *Server) Run() error {
	return s.eio.Run()
}

func (s *Server) PollTimeout() time.Duration {
	return s.eio.PollTimeout()
}

func (s *Server) HTTPWriteTimeout() time.Duration {
	return s.eio.HTTPWriteTimeout()
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.eio.ServeHTTP(w, r)
}

func (s *Server) IsClosed() bool {
	return s.eio.IsClosed()
}

// Shut down the server. Server cannot be restarted once it is closed.
func (s *Server) Close() error {
	for _, _socket := range s.Sockets() {
		socket := _socket.(*serverSocket)
		socket.onClose(ReasonServerShuttingDown)
	}
	return s.eio.Close()
}
