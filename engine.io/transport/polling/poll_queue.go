package polling

import (
	"time"

	"github.com/karagenc/socket.io-go/internal/sync"

	"github.com/karagenc/socket.io-go/engine.io/parser"
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

// poll for packets. If we already have a packet, this function will immediately return.
// Otherwise it will wait for a packet until pollTimeout is reached.
func (pq *pollQueue) poll(pollTimeout time.Duration) []*parser.Packet {
	packets := pq.get()

	if len(packets) > 0 {
		return packets
	}

	select {
	case <-pq.ready:
		packets = pq.get()
	case <-time.After(pollTimeout):
	}
	return packets
}

// add a packet to the queue and signal the other goroutine (if any).
func (pq *pollQueue) add(packets ...*parser.Packet) {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	if len(pq.packets) == 0 {
		pq.packets = packets
	} else {
		pq.packets = append(pq.packets, packets...)
	}

	// Send the signal.
	select {
	case pq.ready <- struct{}{}:
	default:
	}
}

// Retrieve the packets without waiting.
func (pq *pollQueue) get() []*parser.Packet {
	pq.mu.Lock()
	packets := pq.packets
	pq.packets = nil
	pq.mu.Unlock()
	return packets
}

func (pq *pollQueue) len() int {
	pq.mu.Lock()
	l := len(pq.packets)
	pq.mu.Unlock()
	return l
}
