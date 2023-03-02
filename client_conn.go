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
	connState   clientConnectionState
	connStateMu sync.RWMutex

	eio            eio.ClientSocket
	eioPacketQueue *packetQueue
	eioMu          sync.RWMutex

	client *Client
}

func newClientConn(client *Client) *clientConn {
	return &clientConn{
		client:         client,
		eioPacketQueue: newPacketQueue(),
	}
}

func (c *clientConn) IsConnected() bool {
	c.connStateMu.RLock()
	defer c.connStateMu.RUnlock()
	return c.connState == clientConnStateConnected
}

func (c *clientConn) Connect() (err error) {
	c.connStateMu.Lock()
	defer c.connStateMu.Unlock()
	if c.connState == clientConnStateConnected {
		return nil
	}
	c.connState = clientConnStateConnecting

	c.eioMu.Lock()
	defer c.eioMu.Unlock()

	callbacks := eio.Callbacks{
		OnPacket: c.client.onEIOPacket,
		OnError:  c.client.onEIOError,
		OnClose:  c.client.onEIOClose,
	}

	_eio, err := eio.Dial(c.client.url, &callbacks, &c.client.eioConfig)
	if err != nil {
		c.client.parserMu.Lock()
		defer c.client.parserMu.Unlock()
		c.client.parser.Reset()

		c.connState = clientConnStateDisconnected
		c.client.emitReserved("error", err)
		return err
	}
	c.eio = _eio

	c.client.parserMu.Lock()
	defer c.client.parserMu.Unlock()
	c.client.parser.Reset()

	go pollAndSend(c.eio, c.eioPacketQueue)

	sockets := c.client.sockets.GetAll()
	for _, socket := range sockets {
		go socket.sendConnectPacket()
	}

	c.client.emitReserved("open")
	return
}

func (c *clientConn) MaybeReconnectOnOpen() {
	c.connStateMu.RLock()
	reconnect := !(c.connState == clientConnStateReconnecting) && c.client.backoff.Attempts() == 0 && !c.client.noReconnection
	c.connStateMu.RUnlock()
	if reconnect {
		c.Reconnect(false)
	}
}

func (c *clientConn) Reconnect(again bool) {
	// again = Is this the first time we're doing reconnect?
	// In other words: are we recursing?

	if !again {
		c.connStateMu.Lock()
		defer c.connStateMu.Unlock()
	}
	if c.connState == clientConnStateReconnecting {
		return
	}
	c.connState = clientConnStateReconnecting

	attempts := c.client.backoff.Attempts()
	didAttemptsReachedMaxAttempts := c.client.reconnectionAttempts > 0 && attempts >= c.client.reconnectionAttempts
	// Just in case
	didAttemptsReachedMaxInt := c.client.reconnectionAttempts == 0 && attempts == math.MaxUint32

	if didAttemptsReachedMaxAttempts || didAttemptsReachedMaxInt {
		c.client.backoff.Reset()
		c.client.emitReserved("reconnect_failed")
		c.connState = clientConnStateDisconnected
		return
	}

	backoffDuration := c.client.backoff.Duration()
	time.Sleep(backoffDuration)

	attempts = c.client.backoff.Attempts()
	c.client.emitReserved("reconnect_attempt", attempts)

	err := c.Connect()
	if err != nil {
		c.client.emitReserved("reconnect", err)
		c.connState = clientConnStateDisconnected
		c.Reconnect(true)
		return
	}

	attempts = c.client.backoff.Attempts()
	c.client.backoff.Reset()
	c.client.emitReserved("reconnect", attempts)
}

func (c *clientConn) packet(packets ...*eioparser.Packet) {
	c.eioMu.RLock()
	defer c.eioMu.RUnlock()
	// TODO: Check if eio connected
	c.eioPacketQueue.Add(packets...)
}

func (c *clientConn) Disconnect() {
	c.connStateMu.Lock()
	defer c.connStateMu.Unlock()
	c.connState = clientConnStateDisconnected
	c.client.onClose("forced close", nil)
	c.eioMu.Lock()
	defer c.eioMu.Unlock()
	c.eio.Close()
	c.eioPacketQueue.Reset()
}
