package eio

import "time"

type Socket interface {
	// Session ID (sid)
	ID() string

	// Available upgrades
	Upgrades() []string

	PingInterval() time.Duration
	PingTimeout() time.Duration

	// Name of the current transport
	TransportName() string

	SendMessage(data []byte, isBinary bool)

	Close()
}
