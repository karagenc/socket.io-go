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
	debug   Debugger
}

func newClientConn(manager *Manager) *clientConn {
	return &clientConn{
		eioPacketQueue: newPacketQueue(),
		manager:        manager,
		debug:          manager.debug.WithContext("clientConn with URL: " + concatURL(manager.url)),
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
		c.manager.skipReconnectMu.RLock()
		defer c.manager.skipReconnectMu.RUnlock()
		c.manager.skipReconnect = false
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

		for _, handler := range c.manager.errorHandlers.GetAll() {
			(*handler)(err)
		}
		return err
	}
	c.eio = _eio
	c.state = clientConnStateConnected

	c.manager.parserMu.Lock()
	defer c.manager.parserMu.Unlock()
	c.manager.parser.Reset()

	go pollAndSend(c.eio, c.eioPacketQueue)

	for _, handler := range c.manager.openHandlers.GetAll() {
		(*handler)()
	}
	return
}

func (c *clientConn) MaybeReconnectOnOpen() {
	reconnect := c.manager.backoff.Attempts() == 0 && !c.manager.noReconnection
	if reconnect {
		c.Reconnect(false)
	}
}

func (c *clientConn) Reconnect(again bool) {
	c.debug.Log("`Reconnect` called. It's happening")

	// again = Is this the first time we're doing reconnect?
	// In other words: are we recursing?
	if !again {
		c.stateMu.Lock()
		defer c.stateMu.Unlock()

		c.manager.skipReconnectMu.RLock()
		defer c.manager.skipReconnectMu.RUnlock()
		if c.manager.skipReconnect {
			c.debug.Log("Skipping reconnect")
			return
		}
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
		c.debug.Log("Maximum attempts reached. Attempts made so far", attempts)
		c.manager.backoff.Reset()
		c.state = clientConnStateDisconnected
		for _, handler := range c.manager.reconnectFailedHandlers.GetAll() {
			(*handler)()
		}
		return
	}

	delay := c.manager.backoff.Duration()
	c.debug.Log("Delay before reconnect attempt", delay)
	time.Sleep(delay)

	if c.manager.skipReconnect {
		c.debug.Log("Skipping reconnect")
		return
	}

	attempts = c.manager.backoff.Attempts()
	for _, handler := range c.manager.reconnectAttemptHandlers.GetAll() {
		(*handler)(attempts)
	}

	if c.manager.skipReconnect {
		c.debug.Log("Skipping reconnect")
		return
	}

	c.debug.Log("Attempting to reconnect")
	err := c.Connect(again)
	if err != nil {
		c.debug.Log("Reconnect failed", err)
		c.state = clientConnStateDisconnected
		for _, handler := range c.manager.reconnectErrorHandlers.GetAll() {
			(*handler)(err)
		}
		c.Reconnect(true)
		return
	}

	c.debug.Log("Reconnect is successful")
	c.onReconnect()
}

func (c *clientConn) onReconnect() {
	attempts := c.manager.backoff.Attempts()
	c.manager.backoff.Reset()
	for _, handler := range c.manager.reconnectHandlers.GetAll() {
		(*handler)(attempts)
	}
}

func (c *clientConn) Packet(packets ...*eioparser.Packet) {
	c.eioMu.RLock()
	defer c.eioMu.RUnlock()
	c.eioPacketQueue.Add(packets...)
}

func (c *clientConn) Disconnect() {
	c.debug.Log("Disconnecting")

	c.stateMu.Lock()
	defer c.stateMu.Unlock()
	c.state = clientConnStateDisconnected

	c.manager.skipReconnectMu.Lock()
	defer c.manager.skipReconnectMu.Unlock()
	c.manager.skipReconnect = true

	c.manager.onClose(ReasonForcedClose, nil)

	c.eioMu.Lock()
	defer c.eioMu.Unlock()
	c.eio.Close()
	c.eioPacketQueue.Reset()
}
