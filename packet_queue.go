package sio

import (
	"time"

	"github.com/karagenc/socket.io-go/internal/sync"

	eio "github.com/karagenc/socket.io-go/engine.io"
	eioparser "github.com/karagenc/socket.io-go/engine.io/parser"
)

type packetQueue struct {
	packets []*eioparser.Packet
	mu      sync.Mutex

	ready  chan struct{}
	drain  chan struct{}
	_reset chan struct{}
	_close chan struct{}
}

func newPacketQueue() *packetQueue {
	return &packetQueue{
		ready:  make(chan struct{}, 1),
		drain:  make(chan struct{}),
		_reset: make(chan struct{}, 1),
		_close: make(chan struct{}, 1),
	}
}

func (pq *packetQueue) poll() (packets []*eioparser.Packet, ok, closed bool) {
	packets = pq.get()
	if len(packets) != 0 {
		ok = true
		return
	}

	select {
	// _close takes precedence.
	// Otherwise we would go with the already-invoked `pq.ready` channel.
	case <-pq._close:
		return nil, false, true
	case <-pq.ready:
		packets = pq.get()
		if len(packets) != 0 {
			ok = true
		}

		select {
		case pq.drain <- struct{}{}:
		default:
		}
	}
	return
}

func (pq *packetQueue) get() (packets []*eioparser.Packet) {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	packets = pq.packets
	pq.packets = nil
	return
}

func (pq *packetQueue) add(packets ...*eioparser.Packet) {
	pq.mu.Lock()

	if len(pq.packets) == 0 {
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

func (pq *packetQueue) reset() {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	pq.packets = nil
	select {
	case pq._reset <- struct{}{}:
	default:
	}
}

func (pq *packetQueue) close() {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	pq.packets = nil
	select {
	case pq._close <- struct{}{}:
	default:
	}
}

func (pq *packetQueue) waitForDrain(timeout time.Duration) (timedout bool) {
	pq.mu.Lock()
	alreadyDrained := len(pq.packets) == 0
	pq.mu.Unlock()
	if alreadyDrained {
		return
	}

	select {
	case <-pq.drain:
	case <-pq._reset:
	case <-time.After(timeout):
		timedout = true
	}
	return
}

func (pq *packetQueue) pollAndSend(socket eio.Socket) {
	for {
		packets, ok, closed := pq.poll()
		if closed {
			return
		}
		if !ok {
			continue
		}
		socket.Send(packets...)
	}
}
