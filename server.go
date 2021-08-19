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
	ParserCreator parser.Creator
	EIO           eio.ServerConfig
}

type Server struct {
	parserCreator   parser.Creator
	eio             *eio.Server
	sockets         *serverSocketStore
	onSocketHandler atomic.Value
}

type serverSocketStore struct {
	sockets map[string]*serverSocket
	mu      sync.Mutex
}

func newServerSocketStore() *serverSocketStore {
	return &serverSocketStore{
		sockets: make(map[string]*serverSocket),
	}
}

func (s *serverSocketStore) Get(sid string) (ss *serverSocket, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ss, ok = s.sockets[sid]
	return
}

func (s *serverSocketStore) GetAll() (sockets []*serverSocket) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sockets = make([]*serverSocket, len(s.sockets))
	i := 0
	for _, ss := range s.sockets {
		sockets[i] = ss
		i++
	}
	return
}

func (s *serverSocketStore) Add(ss *serverSocket) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sockets[ss.ID()] = ss
}

func (s *serverSocketStore) Delete(sid string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sockets, sid)
}

func NewServer(config *ServerConfig) *Server {
	if config == nil {
		config = new(ServerConfig)
	}

	server := &Server{
		parserCreator: config.ParserCreator,
		sockets:       newServerSocketStore(),
	}

	var f OnSocketCallback = func(socket Socket) {}
	server.onSocketHandler.Store(f)

	server.eio = eio.NewServer(server.onSocket, &config.EIO)

	if server.parserCreator == nil {
		server.parserCreator = jsonparser.NewCreator(0)
	}

	return server
}

func (s *Server) OnSocket(handler OnSocketCallback) {
	if handler != nil {
		s.onSocketHandler.Store(handler)
	}
}

func (s *Server) onSocket(eioSocket eio.Socket) *eio.Callbacks {
	ss, callbacks, err := newServerSocket(eioSocket, s.parserCreator)
	if err != nil {
		panic(err)
	}
	s.sockets.Add(ss)

	//onSocket := s.onSocketHandler.Load().(OnSocketCallback)
	//go onSocket(ss)

	return callbacks
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
