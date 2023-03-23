package transport

import (
	"sync/atomic"

	"github.com/tomruk/socket.io-go/engine.io/parser"
)

type (
	PacketCallback func(packet ...*parser.Packet)
	CloseCallback  func(transportName string, err error)
)

type Callbacks struct {
	onPacket atomic.Value
	onClose  atomic.Value
}

func NewCallbacks() *Callbacks {
	c := new(Callbacks)
	c.Set(nil, nil)
	return c
}

func (c *Callbacks) OnPacket(packet ...*parser.Packet) {
	f := c.onPacket.Load().(PacketCallback)
	f(packet...)
}

func (c *Callbacks) OnClose(transportName string, err error) {
	f := c.onClose.Load().(CloseCallback)
	f(transportName, err)
}

func (c *Callbacks) Set(onPacket PacketCallback, onClose CloseCallback) {
	if onPacket != nil {
		c.onPacket.Store(onPacket)
	} else {
		var f PacketCallback = func(packet ...*parser.Packet) {}
		c.onPacket.Store(f)
	}

	if onClose != nil {
		c.onClose.Store(onClose)
	} else {
		var f CloseCallback = func(transportName string, err error) {}
		c.onClose.Store(f)
	}
}
