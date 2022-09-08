package sio

import (
	"net/http"
	"sync"
	"time"

	eio "github.com/tomruk/socket.io-go/engine.io"
	"github.com/tomruk/socket.io-go/parser"
	jsonparser "github.com/tomruk/socket.io-go/parser/json"
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

	conns *serverConnStore
	nsps  *namespaceStore
}

type serverConnStore struct {
	conns map[string]*serverConn
	mu    sync.Mutex
}

func newServerConnStore() *serverConnStore {
	return &serverConnStore{
		conns: make(map[string]*serverConn),
	}
}

func NewServer(config *ServerConfig) *Server {
	if config == nil {
		config = new(ServerConfig)
	}

	server := &Server{
		parserCreator:  config.ParserCreator,
		adapterCreator: config.AdapterCreator,
		nsps:           newNamespaceStore(),
		conns:          newServerConnStore(),
	}

	server.eio = eio.NewServer(server.onEIOSocket, &config.EIO)

	if server.parserCreator == nil {
		server.parserCreator = jsonparser.NewCreator(0)
	}

	if server.adapterCreator == nil {
		server.adapterCreator = newInMemoryAdapter
	}

	return server
}

func (s *Server) onEIOSocket(eioSocket eio.Socket) *eio.Callbacks {
	_, callbacks := newServerConn(s, eioSocket, s.parserCreator)
	return callbacks
}

func (s *Server) Of(namespace string) *Namespace {
	if len(namespace) != 0 && namespace[0] != '/' {
		namespace = "/" + namespace
	}
	return s.nsps.GetOrCreate(namespace, s, s.adapterCreator, s.parserCreator)
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

// Alias of: s.Of("/").AllSockets(...)
func (s *Server) AllSockets() (sids []string) {
	return s.Of("/").AllSockets()
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
