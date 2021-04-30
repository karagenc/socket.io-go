package eio

import "github.com/tomruk/socket.io-go/engine.io/parser"

type NewSocketCallback func(socket Socket) *Callbacks

type PacketCallback func(packet *parser.Packet)

type ErrorCallback func(err error)

// err can be nil. Always do a nil check.
type CloseCallback func(reason string, err error)

type Callbacks struct {
	OnPacket PacketCallback
	OnError  ErrorCallback
	OnClose  CloseCallback
}

func (c *Callbacks) setMissing() {
	if c.OnPacket == nil {
		c.OnPacket = func(packet *parser.Packet) {}
	}
	if c.OnError == nil {
		c.OnError = func(err error) {}
	}
	if c.OnClose == nil {
		c.OnClose = func(reason string, err error) {}
	}
}
