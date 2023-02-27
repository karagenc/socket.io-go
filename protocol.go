package sio

import (
	eioparser "github.com/tomruk/socket.io-go/engine.io/parser"
	"github.com/tomruk/socket.io-go/parser"
)

const (
	SocketIOProtocolVersion = parser.ProtocolVersion
	EngineIOProtocolVersion = eioparser.ProtocolVersion
)
