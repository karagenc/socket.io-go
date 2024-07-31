package sio

import (
	"math"
	"time"

	eio "github.com/karagenc/socket.io-go/engine.io"
	"github.com/karagenc/socket.io-go/engine.io/parser"
	"github.com/karagenc/socket.io-go/internal/sync"
)

// Manager methods that are directly related to
// connection, reconnection, and disconnection functionalities.

type clientConnectionState int

const (
	clientConnStateConnecting clientConnectionState = iota
	clientConnStateConnected
	clientConnStateReconnecting
	clientConnStateDisconnected
)

func (m *Manager) connected() bool {
	m.stateMu.RLock()
	defer m.stateMu.RUnlock()
	return m.state == clientConnStateConnected
}

func (m *Manager) connect(recursed bool) (err error) {
	// recursed = Is this the first time we're running the connect method?
	// In other words: are we recursing?
	if !recursed {
		m.connectMu.Lock()
		defer m.connectMu.Unlock()

		m.skipReconnectMu.Lock()
		m.skipReconnect = false
		m.skipReconnectMu.Unlock()
	}

	m.stateMu.Lock()
	if m.state == clientConnStateConnected {
		m.stateMu.Unlock()
		return nil
	}
	m.state = clientConnStateConnecting
	m.stateMu.Unlock()

	m.eioMu.Lock()
	defer m.eioMu.Unlock()

	var (
		active   = true
		activeMu sync.Mutex
	)
	callbacks := eio.Callbacks{
		OnPacket: func(packets ...*parser.Packet) {
			activeMu.Lock()
			if !active {
				activeMu.Unlock()
				return
			}
			activeMu.Unlock()
			m.onEIOPacket(packets...)
		},
		OnError: func(err error) {
			activeMu.Lock()
			if !active {
				activeMu.Unlock()
				return
			}
			activeMu.Unlock()
			m.onError(err)
		},
		OnClose: func(reason eio.Reason, err error) {
			activeMu.Lock()
			if !active {
				activeMu.Unlock()
				return
			}
			activeMu.Unlock()
			m.onClose(reason, err)
		},
	}

	_eio, err := eio.Dial(m.url, &callbacks, &m.eioConfig)
	if err != nil {
		m.resetParser()
		m.stateMu.Lock()
		m.state = clientConnStateDisconnected
		m.stateMu.Unlock()
		m.errorHandlers.forEach(func(handler *ManagerErrorFunc) { (*handler)(err) }, true)
		return err
	}

	m.stateMu.Lock()
	m.state = clientConnStateConnected
	m.stateMu.Unlock()
	m.eio = _eio
	m.resetParser()
	m.closePacketQueue(m.eioPacketQueue)

	m.cleanup()
	m.subsMu.Lock()
	activeMu.Lock()
	if active {
		m.subs = append(m.subs, func() {
			activeMu.Lock()
			active = false
			activeMu.Unlock()
		})
	}
	activeMu.Unlock()
	m.subsMu.Unlock()

	m.eioPacketQueue = newPacketQueue()
	go m.eioPacketQueue.pollAndSend(_eio)
	m.openHandlers.forEach(func(handler *ManagerOpenFunc) { (*handler)() }, true)
	return
}

func (m *Manager) reconnect(recursed bool) {
	m.debug.Log("`reconnect` called")

	// recursed = Is this the first time we're running the reconnect method?
	// In other words: are we recursing?
	if !recursed {
		m.connectMu.Lock()
		defer m.connectMu.Unlock()

		m.skipReconnectMu.RLock()
		if m.skipReconnect {
			m.skipReconnectMu.RUnlock()
			m.debug.Log("Skipping reconnect")
			return
		}
		m.skipReconnectMu.RUnlock()
	}

	// If the state is 'connected', 'connecting', or 'reconnecting', etc; don't try to connect.
	//
	// If the state is 'reconnecting' or 'connecting', there is another goroutine trying to connect.
	// If the state is 'connected', there is nothing for this method to do.
	m.stateMu.Lock()
	if m.state != clientConnStateDisconnected {
		m.stateMu.Unlock()
		return
	}
	m.state = clientConnStateReconnecting
	m.stateMu.Unlock()

	attempts := m.backoff.attempts()
	didAttemptsReachedMaxAttempts := m.reconnectionAttempts > 0 && attempts >= m.reconnectionAttempts
	// Just in case
	didAttemptsReachedMaxInt := m.reconnectionAttempts == 0 && attempts == math.MaxUint32

	if didAttemptsReachedMaxAttempts || didAttemptsReachedMaxInt {
		m.debug.Log("Maximum attempts reached. Attempts made so far", attempts)
		m.backoff.reset()
		m.stateMu.Lock()
		m.state = clientConnStateDisconnected
		m.stateMu.Unlock()
		m.reconnectFailedHandlers.forEach(func(handler *ManagerReconnectFailedFunc) { (*handler)() }, true)
		return
	}

	delay := m.backoff.duration()
	m.debug.Log("Delay before reconnect attempt", delay)
	time.Sleep(delay)

	m.skipReconnectMu.RLock()
	if m.skipReconnect {
		m.skipReconnectMu.RUnlock()
		m.debug.Log("Skipping reconnect")
		return
	}
	m.skipReconnectMu.RUnlock()

	attempts = m.backoff.attempts()
	m.reconnectAttemptHandlers.forEach(func(handler *ManagerReconnectAttemptFunc) { (*handler)(attempts) }, true)

	m.skipReconnectMu.RLock()
	if m.skipReconnect {
		m.skipReconnectMu.RUnlock()
		m.debug.Log("Skipping reconnect")
		return
	}
	m.skipReconnectMu.RUnlock()

	m.debug.Log("Attempting to reconnect")
	err := m.connect(true)
	if err != nil {
		m.debug.Log("Reconnect failed", err)
		m.stateMu.Lock()
		m.state = clientConnStateDisconnected
		m.stateMu.Unlock()
		m.reconnectErrorHandlers.forEach(func(handler *ManagerReconnectErrorFunc) { (*handler)(err) }, true)
		m.reconnect(true)
		return
	}
	m.debug.Log("Reconnected")
	m.onReconnect()
}
