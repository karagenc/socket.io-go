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
	ParserCreator parser.Creator
	EIO           eio.ServerConfig
}

type Server struct {
	parserCreator parser.Creator
	eio           *eio.Server
	socketStore   *socketStore
}

type socketStore struct {
	sockets map[string]*serverSocket
	mu      sync.Mutex
}

func (s *socketStore) Get(sid string) (ss *serverSocket, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ss, ok = s.sockets[sid]
	return
}

func (s *socketStore) GetAll() (sockets []*serverSocket) {
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

func (s *socketStore) Add(ss *serverSocket) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sockets[ss.ID()] = ss
}

func (s *socketStore) Delete(sid string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sockets, sid)
}

func NewServer(config *ServerConfig) *Server {
	if config == nil {
		config = new(ServerConfig)
	}

	s := &Server{
		parserCreator: config.ParserCreator,
		socketStore:   new(socketStore),
	}

	s.eio = eio.NewServer(s.onSocket, &config.EIO)

	if s.parserCreator == nil {
		s.parserCreator = jsonparser.NewCreator(0)
	}

	return s
}

func (s *Server) onSocket(eioSocket eio.Socket) *eio.Callbacks {
	ss, callbacks, err := newServerSocket(eioSocket, s.parserCreator)
	if err != nil {
		panic(err)
	}
	s.socketStore.Add(ss)

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
