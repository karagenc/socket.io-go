package sio

import (
	"math"
	"time"

	"github.com/tomruk/socket.io-go/internal/sync"

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
		debug:          manager.debug.WithContext("[sio/client] clientConn with URL: " + truncateURL(manager.url)),
	}
}

func (c *clientConn) connected() bool {
	c.stateMu.RLock()
	defer c.stateMu.RUnlock()
	return c.state == clientConnStateConnected
}

func (c *clientConn) connect(again bool) (err error) {
	// again = Is this the first time we're doing reconnect?
	// In other words: are we recursing?
	if !again {
		c.stateMu.Lock()
		defer c.stateMu.Unlock()

		c.manager.skipReconnectMu.Lock()
		defer c.manager.skipReconnectMu.Unlock()
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
		OnError:  c.manager.onError,
		OnClose:  c.manager.onClose,
	}

	_eio, err := eio.Dial(c.manager.url, &callbacks, &c.manager.eioConfig)
	if err != nil {
		c.manager.resetParser()
		c.state = clientConnStateDisconnected
		c.manager.errorHandlers.forEach(func(handler *ManagerErrorFunc) { (*handler)(err) }, true)
		return err
	}

	c.state = clientConnStateConnected
	c.eio = _eio
	c.eioPacketQueue.reset()
	c.manager.resetParser()

	go c.eioPacketQueue.pollAndSend(c.eio)
	c.manager.openHandlers.forEach(func(handler *ManagerOpenFunc) { (*handler)() }, true)
	return
}

func (c *clientConn) maybeReconnectOnOpen() {
	reconnect := c.manager.backoff.attempts() == 0 && !c.manager.noReconnection
	if reconnect {
		c.reconnect(false)
	}
}

func (c *clientConn) reconnect(again bool) {
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

	attempts := c.manager.backoff.attempts()
	didAttemptsReachedMaxAttempts := c.manager.reconnectionAttempts > 0 && attempts >= c.manager.reconnectionAttempts
	// Just in case
	didAttemptsReachedMaxInt := c.manager.reconnectionAttempts == 0 && attempts == math.MaxUint32

	if didAttemptsReachedMaxAttempts || didAttemptsReachedMaxInt {
		c.debug.Log("Maximum attempts reached. Attempts made so far", attempts)
		c.manager.backoff.reset()
		c.state = clientConnStateDisconnected
		c.manager.reconnectFailedHandlers.forEach(func(handler *ManagerReconnectFailedFunc) { (*handler)() }, true)
		return
	}

	delay := c.manager.backoff.duration()
	c.debug.Log("Delay before reconnect attempt", delay)
	time.Sleep(delay)

	if c.manager.skipReconnect {
		c.debug.Log("Skipping reconnect")
		return
	}

	attempts = c.manager.backoff.attempts()
	c.manager.reconnectAttemptHandlers.forEach(func(handler *ManagerReconnectAttemptFunc) { (*handler)(attempts) }, true)

	if c.manager.skipReconnect {
		c.debug.Log("Skipping reconnect")
		return
	}

	c.debug.Log("Attempting to reconnect")
	err := c.connect(true)
	if err != nil {
		c.debug.Log("Reconnect failed", err)
		c.state = clientConnStateDisconnected
		c.manager.reconnectErrorHandlers.forEach(func(handler *ManagerReconnectErrorFunc) { (*handler)(err) }, true)
		c.reconnect(true)
		return
	}

	c.debug.Log("Reconnect is successful")
	c.onReconnect()
}

func (c *clientConn) onReconnect() {
	attempts := c.manager.backoff.attempts()
	c.manager.backoff.reset()
	c.manager.reconnectHandlers.forEach(func(handler *ManagerReconnectFunc) { (*handler)(attempts) }, true)
}

func (c *clientConn) packet(packets ...*eioparser.Packet) {
	c.eioMu.RLock()
	defer c.eioMu.RUnlock()
	c.eioPacketQueue.add(packets...)
}

func (c *clientConn) disconnect() {
	c.debug.Log("Disconnecting")

	c.stateMu.Lock()
	defer c.stateMu.Unlock()
	c.state = clientConnStateDisconnected

	c.manager.skipReconnectMu.Lock()
	c.manager.skipReconnect = true
	c.manager.skipReconnectMu.Unlock()

	c.manager.onClose(ReasonForcedClose, nil)

	c.eioMu.RLock()
	defer c.eioMu.RUnlock()
	eio := c.eio
	if eio != nil {
		go eio.Close()
	}
	c.eioPacketQueue.reset()
}
