package sio

import (
	"net/http"
	"sync"
	"sync/atomic"
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

	connections *serverConnStore

	namespaces   map[string]*Namespace
	namespacesMu sync.Mutex

	onSocketHandler atomic.Value
}

type serverConnStore struct {
	conns map[string]*serverConn
	mu    sync.Mutex
}

func NewServer(config *ServerConfig) *Server {
	if config == nil {
		config = new(ServerConfig)
	}

	server := &Server{
		parserCreator:  config.ParserCreator,
		adapterCreator: config.AdapterCreator,
	}

	var f OnSocketCallback = func(socket Socket) {}
	server.onSocketHandler.Store(f)

	server.eio = eio.NewServer(server.onEIOSocket, &config.EIO)

	if server.parserCreator == nil {
		server.parserCreator = jsonparser.NewCreator(0)
	}

	if server.adapterCreator == nil {
		server.adapterCreator = newInMemoryAdapter
	}

	return server
}

func (s *Server) OnSocket(handler OnSocketCallback) {
	if handler != nil {
		s.onSocketHandler.Store(handler)
	}
}

func (s *Server) onEIOSocket(eioSocket eio.Socket) *eio.Callbacks {
	_, callbacks := newServerConn(eioSocket, s.parserCreator)
	return callbacks
}

func (s *Server) Of(namespace string) *Namespace {
	if len(namespace) != 0 && namespace[0] != '/' {
		namespace = "/" + namespace
	}

	s.namespacesMu.Lock()
	n, ok := s.namespaces[namespace]
	s.namespacesMu.Unlock()

	if !ok {
		n = newNamespace(namespace, s.adapterCreator)
		s.namespacesMu.Lock()
		s.namespaces[namespace] = n
		s.namespacesMu.Unlock()
	}

	return n
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
