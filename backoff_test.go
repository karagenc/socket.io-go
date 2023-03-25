package sio

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestBackoff(t *testing.T) {
	b := newBackoff(1*time.Second, 50*time.Second, 0)

	testBackoffDuration(t, b)

	b.reset()
	require.Equal(t, uint32(0), b.attempts())

	d := b.duration()
	require.Equal(t, 1*time.Second, d)

	testBackoffDuration(t, b)
}

func TestBackoffWithJitter(t *testing.T) {
	// const jitter = 0.5
	// b := newBackoff(1*time.Second, 50*time.Second, jitter)

	// testBackoffDuration(t, b)

	// b.reset()
	// require.Equal(t, uint32(0), b.attempts())

	// d := b.duration()
	// require.Equal(t, 1*time.Second, d)

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
