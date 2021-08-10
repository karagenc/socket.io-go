package sio

import (
	"testing"
	"time"
)

func TestBackoff(t *testing.T) {
	b := newBackoff(1*time.Second, 50*time.Second, 0)

	var last time.Duration
	for i := 0; i < 1000; i++ {
		d := b.Duration()
		//fmt.Println(d.Milliseconds())
		if d < last {
			t.Fatalf("d should be higher than the last value: d: %d, last: %d", d.Milliseconds(), last.Milliseconds())
		}
		last = d
	}
}
