package sio

import (
	"math"
	"math/rand"
	"time"

	"github.com/karagenc/socket.io-go/internal/sync"
)

type backoff struct {
	min    time.Duration
	max    time.Duration
	factor int64
	jitter float64

	numAttempts   uint32
	numAttemptsMu sync.Mutex
}

func newBackoff(min time.Duration, max time.Duration, jitter float32) *backoff {
	if jitter <= 0 || jitter > 1 {
		jitter = 0
	}
	return &backoff{
		min:    min,
		max:    max,
		factor: 2,
		jitter: float64(jitter),
	}
}

func (b *backoff) attempts() uint32 {
	b.numAttemptsMu.Lock()
	attempts := b.numAttempts
	b.numAttemptsMu.Unlock()
	return attempts
}

func (b *backoff) duration() time.Duration {
	b.numAttemptsMu.Lock()
	ms := int64(b.min) * int64(math.Pow(float64(b.factor), float64(b.numAttempts)))
	b.numAttempts++
	b.numAttemptsMu.Unlock()

	if b.jitter > 0 {
		r := rand.Float64()
		deviation := math.Floor(r * b.jitter * float64(ms))

		t := int64(math.Floor(r*10)) & 1
		if t == 0 {
			ms = ms - int64(deviation)
		} else {
			ms = ms + int64(deviation)
		}
	}
	if ms <= 0 {
		return b.max
	}
	return time.Duration(math.Min(float64(ms), float64(b.max)))
}

func (b *backoff) reset() {
	b.numAttemptsMu.Lock()
	b.numAttempts = 0
	b.numAttemptsMu.Unlock()
}
