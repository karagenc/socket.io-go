package sio

import (
	"net/http"
	"time"

	eio "github.com/tomruk/socket.io-go/engine.io"
	"github.com/tomruk/socket.io-go/parser"
	jsonparser "github.com/tomruk/socket.io-go/parser/json"
)

type ServerConfig struct {
	ParserCreator parser.Creator

	EIO eio.ServerConfig
}

type Server struct {
	parserCreator parser.Creator
	eio           *eio.Server
}

func NewServer(config *ServerConfig) *Server {
	if config == nil {
		config = new(ServerConfig)
	}

	s := &Server{
		parserCreator: config.ParserCreator,
		eio:           eio.NewServer(nil, &config.EIO),
	}

	if s.parserCreator == nil {
		s.parserCreator = jsonparser.NewCreator(0)
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
