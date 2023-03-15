package sio

import (
	"reflect"
	"sync"
)

type ackSender struct {
	socket *clientSocket

	mu         sync.Mutex
	hasAckFunc bool
	sent       bool
}

func newAckSender(socket *clientSocket) *ackSender {
	return &ackSender{
		socket: socket,
	}
}

func (m *ackSender) EventHasAnAckFunc() {
	m.mu.Lock()
	m.hasAckFunc = true
	m.mu.Unlock()
}

func (m *ackSender) Finish(ackID uint64) {
	m.mu.Lock()
	send := !m.hasAckFunc
	if m.sent {
		m.mu.Unlock()
		return
	}
	m.sent = true
	m.mu.Unlock()

	if send {
		m.socket.sendAckPacket(ackID, nil)
	}
}

func (m *ackSender) SendAck(ackID uint64, ret []reflect.Value) {
	m.mu.Lock()
	if m.sent {
		m.mu.Unlock()
		return
	}
	m.sent = true
	m.mu.Unlock()
	m.socket.sendAckPacket(ackID, ret)
}
