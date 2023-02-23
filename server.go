package sio

import (
	"net/http"
	"time"

	mapset "github.com/deckarep/golang-set/v2"
	eio "github.com/tomruk/socket.io-go/engine.io"
	"github.com/tomruk/socket.io-go/parser"
	jsonparser "github.com/tomruk/socket.io-go/parser/json"
	"github.com/tomruk/socket.io-go/parser/json/serializer/stdjson"
)

type ServerConfig struct {
	ParserCreator  parser.Creator
	AdapterCreator AdapterCreator

	EIO eio.ServerConfig
}

type Server struct {
	parserCreator  parser.Creator
	adapterCreator AdapterCreator

	eio *eio.Server

	namespaces *namespaceStore
}

func NewServer(config *ServerConfig) *Server {
	if config == nil {
		config = new(ServerConfig)
	}

	server := &Server{
		parserCreator:  config.ParserCreator,
		adapterCreator: config.AdapterCreator,
		namespaces:     newNamespaceStore(),
	}

	server.eio = eio.NewServer(server.onEIOSocket, &config.EIO)

	if server.parserCreator == nil {
		json := stdjson.New()
		server.parserCreator = jsonparser.NewCreator(0, json)
	}

	if server.adapterCreator == nil {
		server.adapterCreator = newInMemoryAdapter
	}

	return server
}

func (s *Server) onEIOSocket(eioSocket eio.ServerSocket) *eio.Callbacks {
	_, callbacks := newServerConn(s, eioSocket, s.parserCreator)
	return callbacks
}

func (s *Server) Of(namespace string) *Namespace {
	if len(namespace) != 0 && namespace[0] != '/' {
		namespace = "/" + namespace
	}
	return s.namespaces.GetOrCreate(namespace, s, s.adapterCreator, s.parserCreator)
}

// Alias of: s.Of("/").Use(...)
func (s *Server) Use(f MiddlewareFunction) {
	s.Of("/").Use(f)

	s.Of("/").Sockets()
}

// Alias of: s.Of("/").On(...)
func (s *Server) On(eventName string, handler interface{}) {
	s.Of("/").On(eventName, handler)
}

// Alias of: s.Of("/").Once(...)
func (s *Server) Once(eventName string, handler interface{}) {
	s.Of("/").Once(eventName, handler)
}

// Alias of: s.Of("/").Off(...)
func (s *Server) Off(eventName string, handler interface{}) {
	s.Of("/").Off(eventName, handler)
}

// Alias of: s.Of("/").OffAll(...)
func (s *Server) OffAll() {
	s.Of("/").OffAll()
}

// Alias of: s.Of("/").To(...)
func (s *Server) To(room ...string) *broadcastOperator {
	return s.Of("/").To(room...)
}

// Alias of: s.Of("/").In(...)
func (s *Server) In(room ...string) *broadcastOperator {
	return s.Of("/").In(room...)
}

// Alias of: s.Of("/").To(...)
func (s *Server) Except(room ...string) *broadcastOperator {
	return s.Of("/").Except(room...)
}

// Alias of: s.Of("/").Compress(...)
func (s *Server) Compress(compress bool) *broadcastOperator {
	return s.Of("/").Compress(compress)
}

// Alias of: s.Of("/").Local(...)
func (s *Server) Local() *broadcastOperator {
	return s.Of("/").Local()
}

// Alias of: s.Of("/").Sockets(...)
func (s *Server) Sockets() []ServerSocket {
	return s.Of("/").Sockets()
}

// Alias of: s.Of("/").FetchSockets(...)
//
// Gets a list of socket IDs connected to this namespace (across all nodes if applicable).
func (s *Server) FetchSockets(room ...string) (sids mapset.Set[string]) {
	return s.Of("/").FetchSockets()
}

// Alias of: s.Of("/").SocketsJoin(...)
//
// Makes the matching socket instances leave the specified rooms.
func (s *Server) SocketsJoin(room ...string) {
	s.Of("/").SocketsJoin(room...)
}

// Alias of: s.Of("/").SocketsLeave(...)
//
// Makes the matching socket instances leave the specified rooms.
func (s *Server) SocketsLeave(room ...string) {
	s.Of("/").SocketsLeave(room...)
}

// Alias of: s.Of("/").DisconnectSockets(...)
//
// Makes the matching socket instances disconnect from the namespace.
//
// If value of close is true, closes the underlying connection. Otherwise, it just disconnects the namespace.
func (s *Server) DisconnectSockets(close bool) {
	s.Of("/").DisconnectSockets(close)
}

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

func (s *Server) Close() error {
	return s.eio.Close()
}
