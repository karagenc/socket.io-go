package sio

import (
	"math"
	"math/rand"
	"sync"
	"time"
)

type backoff struct {
	min    time.Duration
	max    time.Duration
	factor int64
	jitter float64

	attempts   uint32
	attemptsMu sync.Mutex
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

func (b *backoff) Attempts() uint32 {
	b.attemptsMu.Lock()
	attempts := b.attempts
	b.attemptsMu.Unlock()
	return attempts
}

func (b *backoff) Duration() time.Duration {
	b.attemptsMu.Lock()
	ms := int64(b.min) * int64(math.Pow(float64(b.factor), float64(b.attempts)))
	b.attempts++
	b.attemptsMu.Unlock()

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

func (b *backoff) Reset() {
	b.attemptsMu.Lock()
	b.attempts = 0
	b.attemptsMu.Unlock()
}
