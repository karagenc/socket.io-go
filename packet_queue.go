package sio

import (
	"sync"

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
		return packets, true
	}

	select {
	case <-pq.ready:
		packets = pq.Get()
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

func pollAndSend(socket eio.Socket, pq *packetQueue) {
	for {
		packets, ok := pq.Poll()
		if !ok {
			break
		}
		socket.Send(packets...)
	}
}
