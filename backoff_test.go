package sio

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestBackoff(t *testing.T) {
	b := newBackoff(1*time.Second, 50*time.Second, 0)

	testBackoffDuration(t, b)

	b.reset()
	if !assert.Equal(t, uint32(0), b.attempts()) {
		return
	}

	d := b.duration()
	if !assert.Equal(t, 1*time.Second, d) {
		return
	}

	testBackoffDuration(t, b)
}

func TestBackoffWithJitter(t *testing.T) {
	// const jitter = 0.5
	// b := newBackoff(1*time.Second, 50*time.Second, jitter)

	// testBackoffDuration(t, b)

	// b.reset()
	// if !assert.Equal(t, uint32(0), b.attempts()) {
	// 	return
	// }

	// d := b.duration()
	// if !assert.Equal(t, 1*time.Second, d) {
	// 	return
	// }

	// testBackoffDuration(t, b)
}

func testBackoffDuration(t *testing.T, b *backoff) {
	var last time.Duration
	for i := 0; i < 1000; i++ {
		d := b.duration()
		t.Logf("Duration: %s", d)
		if d < last {
			t.Fatalf("d should be higher than the last value: d: %d, last: %d", d.Milliseconds(), last.Milliseconds())
		}
		last = d
	}
}
