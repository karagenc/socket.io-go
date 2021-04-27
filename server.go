package sio

import (
	"net/http"
	"time"

	eio "github.com/tomruk/socket.io-go/engine.io"
)

type ServerConfig struct {
	EIO eio.ServerConfig
}

type Server struct {
	eio *eio.Server
}

func NewServer(config *ServerConfig) *Server {
	if config == nil {
		config = new(ServerConfig)
	}

	s := &Server{
		eio: eio.NewServer(nil, &config.EIO),
	}

	return s
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
