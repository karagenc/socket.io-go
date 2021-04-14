package polling

import (
	"sync"
	"time"

	"github.com/tomruk/socket.io-go/engine.io/parser"
)

type pollQueue struct {
	packets []*parser.Packet
	ready   chan struct{}
	mu      sync.Mutex
}

func newPollQueue() *pollQueue {
	return &pollQueue{
		ready: make(chan struct{}),
	}
}

// Poll for packets. If we already have a packet, this function will immediately return.
// Otherwise it will wait for a packet until pollTimeout is reached.
func (pq *pollQueue) Poll(pollTimeout time.Duration) []*parser.Packet {
	packets := pq.Get()

	if len(packets) > 0 {
		return packets
	}

	select {
	case <-pq.ready:
		packets = pq.Get()
	case <-time.After(pollTimeout):
	}
	return packets
}

// Add a packet to the queue and signal the other goroutine (if any).
func (pq *pollQueue) Add(p *parser.Packet) {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	if len(pq.packets) == 0 {
		pq.packets = []*parser.Packet{p}
	} else {
		pq.packets = append(pq.packets, p)
	}

	// Send the signal.
	select {
	case pq.ready <- struct{}{}:
	default:
	}
}

// Retrieve the packets without waiting.
func (pq *pollQueue) Get() []*parser.Packet {
	pq.mu.Lock()
	packets := pq.packets
	pq.packets = nil
	pq.mu.Unlock()
	return packets
}

func (pq *pollQueue) Len() int {
	pq.mu.Lock()
	l := len(pq.packets)
	pq.mu.Unlock()
	return l
}
