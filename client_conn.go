package sio

import (
	"math"
	"sync"
	"time"

	eio "github.com/tomruk/socket.io-go/engine.io"
	eioparser "github.com/tomruk/socket.io-go/engine.io/parser"
)

type clientConnectionState int

const (
	clientConnStateConnecting clientConnectionState = iota
	clientConnStateConnected
	clientConnStateReconnecting
	clientConnStateDisconnected
)

type clientConn struct {
	state   clientConnectionState
	stateMu sync.RWMutex

	eio            eio.ClientSocket
	eioPacketQueue *packetQueue
	eioMu          sync.RWMutex

	manager *Manager
}

func newClientConn(manager *Manager) *clientConn {
	return &clientConn{
		manager:        manager,
		eioPacketQueue: newPacketQueue(),
	}
}

func (c *clientConn) Connected() bool {
	c.stateMu.RLock()
	defer c.stateMu.RUnlock()
	return c.state == clientConnStateConnected
}

func (c *clientConn) Connect(again bool) (err error) {
	// again = Is this the first time we're doing reconnect?
	// In other words: are we recursing?
	if !again {
		c.stateMu.Lock()
		defer c.stateMu.Unlock()
	}

	if c.state == clientConnStateConnected {
		return nil
	}
	c.state = clientConnStateConnecting

	c.eioMu.Lock()
	defer c.eioMu.Unlock()

	callbacks := eio.Callbacks{
		OnPacket: c.manager.onEIOPacket,
		OnError:  c.manager.onEIOError,
		OnClose:  c.manager.onEIOClose,
	}

	_eio, err := eio.Dial(c.manager.url, &callbacks, &c.manager.eioConfig)
	if err != nil {
		c.manager.parserMu.Lock()
		defer c.manager.parserMu.Unlock()
		c.manager.parser.Reset()

		c.state = clientConnStateDisconnected
		c.manager.emitReserved("error", err)
		return err
	}
	c.eio = _eio

	c.manager.parserMu.Lock()
	defer c.manager.parserMu.Unlock()
	c.manager.parser.Reset()

	go pollAndSend(c.eio, c.eioPacketQueue)

	sockets := c.manager.sockets.GetAll()
	for _, socket := range sockets {
		go socket.onOpen()
	}

	c.manager.emitReserved("open")
	return
}

func (c *clientConn) MaybeReconnectOnOpen() {
	reconnect := c.manager.backoff.Attempts() == 0 && !c.manager.noReconnection
	if reconnect {
		c.Reconnect(false)
	}
}

func (c *clientConn) Reconnect(again bool) {
	// again = Is this the first time we're doing reconnect?
	// In other words: are we recursing?
	if !again {
		c.stateMu.Lock()
		defer c.stateMu.Unlock()
	}

	// If the state is 'connected', 'connecting', or 'reconnecting', etc; don't try to connect.
	//
	// If the state is 'reconnecting' or 'connecting', there is another goroutine trying to connect.
	// If the state is 'connected', there is nothing for this method to do.
	if c.state != clientConnStateDisconnected {
		return
	}
	c.state = clientConnStateReconnecting

	attempts := c.manager.backoff.Attempts()
	didAttemptsReachedMaxAttempts := c.manager.reconnectionAttempts > 0 && attempts >= c.manager.reconnectionAttempts
	// Just in case
	didAttemptsReachedMaxInt := c.manager.reconnectionAttempts == 0 && attempts == math.MaxUint32

	if didAttemptsReachedMaxAttempts || didAttemptsReachedMaxInt {
		c.manager.backoff.Reset()
		c.manager.emitReserved("reconnect_failed")
		c.state = clientConnStateDisconnected
		return
	}

	backoffDuration := c.manager.backoff.Duration()
	time.Sleep(backoffDuration)

	attempts = c.manager.backoff.Attempts()
	c.manager.emitReserved("reconnect_attempt", attempts)

	err := c.Connect(again)
	if err != nil {
		c.manager.emitReserved("reconnect", err)
		c.state = clientConnStateDisconnected
		c.Reconnect(true)
		return
	}

	attempts = c.manager.backoff.Attempts()
	c.manager.backoff.Reset()
	c.manager.emitReserved("reconnect", attempts)
}

func (c *clientConn) Packet(packets ...*eioparser.Packet) {
	c.eioMu.RLock()
	defer c.eioMu.RUnlock()
	// TODO: Check if eio connected
	c.eioPacketQueue.Add(packets...)
}

func (c *clientConn) Disconnect() {
	c.stateMu.Lock()
	defer c.stateMu.Unlock()
	c.state = clientConnStateDisconnected
	c.manager.onClose("forced close", nil)
	c.eioMu.Lock()
	defer c.eioMu.Unlock()
	c.eio.Close()
	c.eioPacketQueue.Reset()
}
