package sio

import (
	"math"
	"time"

	eio "github.com/tomruk/socket.io-go/engine.io"
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

func (m *Manager) connect(again bool) (err error) {
	// again = Is this the first time we're doing reconnect?
	// In other words: are we recursing?
	if !again {
		m.stateMu.Lock()
		defer m.stateMu.Unlock()

		m.skipReconnectMu.Lock()
		defer m.skipReconnectMu.Unlock()
		m.skipReconnect = false
	}

	if m.state == clientConnStateConnected {
		return nil
	}
	m.state = clientConnStateConnecting

	m.eioMu.Lock()
	defer m.eioMu.Unlock()

	callbacks := eio.Callbacks{
		OnPacket: m.onEIOPacket,
		OnError:  m.onError,
		OnClose:  m.onClose,
	}

	_eio, err := eio.Dial(m.url, &callbacks, &m.eioConfig)
	if err != nil {
		m.resetParser()
		m.state = clientConnStateDisconnected
		m.errorHandlers.forEach(func(handler *ManagerErrorFunc) { (*handler)(err) }, true)
		return err
	}

	m.state = clientConnStateConnected
	m.eio = _eio
	m.eioPacketQueue.reset()
	m.resetParser()

	go m.eioPacketQueue.pollAndSend(m.eio)
	m.openHandlers.forEach(func(handler *ManagerOpenFunc) { (*handler)() }, true)
	return
}

func (m *Manager) reconnect(again bool) {
	m.debug.Log("`reconnect` called")

	// again = Is this the first time we're doing reconnect?
	// In other words: are we recursing?
	if !again {
		m.stateMu.Lock()
		defer m.stateMu.Unlock()

		m.skipReconnectMu.RLock()
		defer m.skipReconnectMu.RUnlock()
		if m.skipReconnect {
			m.debug.Log("Skipping reconnect")
			return
		}
	}

	// If the state is 'connected', 'connecting', or 'reconnecting', etc; don't try to connect.
	//
	// If the state is 'reconnecting' or 'connecting', there is another goroutine trying to connect.
	// If the state is 'connected', there is nothing for this method to do.
	if m.state != clientConnStateDisconnected {
		return
	}
	m.state = clientConnStateReconnecting

	attempts := m.backoff.attempts()
	didAttemptsReachedMaxAttempts := m.reconnectionAttempts > 0 && attempts >= m.reconnectionAttempts
	// Just in case
	didAttemptsReachedMaxInt := m.reconnectionAttempts == 0 && attempts == math.MaxUint32

	if didAttemptsReachedMaxAttempts || didAttemptsReachedMaxInt {
		m.debug.Log("Maximum attempts reached. Attempts made so far", attempts)
		m.backoff.reset()
		m.state = clientConnStateDisconnected
		m.reconnectFailedHandlers.forEach(func(handler *ManagerReconnectFailedFunc) { (*handler)() }, true)
		return
	}

	delay := m.backoff.duration()
	m.debug.Log("Delay before reconnect attempt", delay)
	time.Sleep(delay)

	if m.skipReconnect {
		m.debug.Log("Skipping reconnect")
		return
	}

	attempts = m.backoff.attempts()
	m.reconnectAttemptHandlers.forEach(func(handler *ManagerReconnectAttemptFunc) { (*handler)(attempts) }, true)

	if m.skipReconnect {
		m.debug.Log("Skipping reconnect")
		return
	}

	m.debug.Log("Attempting to reconnect")
	err := m.connect(true)
	if err != nil {
		m.debug.Log("Reconnect failed", err)
		m.state = clientConnStateDisconnected
		m.reconnectErrorHandlers.forEach(func(handler *ManagerReconnectErrorFunc) { (*handler)(err) }, true)
		m.reconnect(true)
		return
	}
	m.debug.Log("Reconnected")
	m.onReconnect()
}

func (m *Manager) disconnect() {
	m.debug.Log("Disconnecting")

	m.stateMu.Lock()
	defer m.stateMu.Unlock()
	m.state = clientConnStateDisconnected

	m.skipReconnectMu.Lock()
	m.skipReconnect = true
	m.skipReconnectMu.Unlock()

	m.onClose(ReasonForcedClose, nil)

	m.eioMu.RLock()
	defer m.eioMu.RUnlock()
	eio := m.eio
	if eio != nil {
		go eio.Close()
	}
	m.eioPacketQueue.reset()
}
