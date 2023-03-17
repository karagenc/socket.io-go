package sio

import (
	"sync"
	"time"

	eio "github.com/tomruk/socket.io-go/engine.io"
	eioparser "github.com/tomruk/socket.io-go/engine.io/parser"
)

type packetQueue struct {
	packets []*eioparser.Packet
	mu      sync.Mutex

	ready chan struct{}
	reset chan struct{}
}

func newPacketQueue() *packetQueue {
	return &packetQueue{
		ready: make(chan struct{}, 1),
		reset: make(chan struct{}, 1),
	}
}

func (pq *packetQueue) Poll() (packets []*eioparser.Packet, ok bool) {
	packets = pq.Get()
	if len(packets) != 0 {
		ok = true
		return
	}

	select {
	case <-pq.ready:
		packets = pq.Get()
		if len(packets) != 0 {
			ok = true
		}
	case <-pq.reset:
		return nil, false
	}
	return
}

func (pq *packetQueue) Get() (packets []*eioparser.Packet) {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	packets = pq.packets
	pq.packets = nil
	return
}

func (pq *packetQueue) Add(packets ...*eioparser.Packet) {
	pq.mu.Lock()

	if len(packets) == 0 {
		pq.packets = packets
	} else {
		pq.packets = append(pq.packets, packets...)
	}
	pq.mu.Unlock()

	select {
	case pq.ready <- struct{}{}:
	default:
	}
}

func (pq *packetQueue) Reset() {
	pq.mu.Lock()
	pq.packets = nil
	pq.mu.Unlock()
	select {
	case pq.reset <- struct{}{}:
	default:
	}
}

func (pq *packetQueue) WaitForDrain(timeout time.Duration) (timedout bool) {
	pq.mu.Lock()
	drained := len(pq.packets) == 0
	pq.mu.Unlock()
	if drained {
		return
	}

	select {
	case <-pq.ready:
	case <-pq.reset:
	case <-time.After(timeout):
		timedout = true
	}
	return
}

func (pq *packetQueue) pollAndSend(socket eio.Socket) {
	for {
		packets, ok := pq.Poll()
		if !ok {
			continue
		}
		socket.Send(packets...)
	}
}
