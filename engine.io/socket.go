package eio

import (
	"time"

	"github.com/karagenc/socket.io-go/engine.io/parser"
)

type (
	Socket interface {
		// Session ID (sid)
		ID() string

		PingInterval() time.Duration
		PingTimeout() time.Duration

		// Name of the current transport
		TransportName() string

		Send(packets ...*parser.Packet)

		Close()
	}

	ServerSocket interface {
		Socket
	}

	ClientSocket interface {
		Socket

		// Available upgrades
		Upgrades() []string
	}
)
