package sio

import (
	"testing"
	"time"
)

func TestBackoff(t *testing.T) {
	b := newBackoff(1*time.Second, 50*time.Second, 0)

	testBackoffDuration(t, b)

	b.reset()
	if b.numAttempts != 0 {
		t.Fatalf("attempts variable should be zero")
	}

	d := b.duration()
	if d != 1*time.Second {
		t.Fatalf("d should be equal to 1 but it is equal to %d", d)
	}

	testBackoffDuration(t, b)
}

func testBackoffDuration(t *testing.T, b *backoff) {
	var last time.Duration
	for i := 0; i < 1000; i++ {
		d := b.duration()
		//fmt.Println(d.Milliseconds())
		if d < last {
			t.Fatalf("d should be higher than the last value: d: %d, last: %d", d.Milliseconds(), last.Milliseconds())
		}
		last = d
	}
}
