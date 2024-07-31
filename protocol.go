package sio

import (
	eioparser "github.com/karagenc/socket.io-go/engine.io/parser"
	"github.com/karagenc/socket.io-go/parser"
)

const (
	SocketIOProtocolVersion = parser.ProtocolVersion
	EngineIOProtocolVersion = eioparser.ProtocolVersion
)
