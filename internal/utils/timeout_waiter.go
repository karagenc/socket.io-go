package utils

import (
	"time"

	"github.com/tomruk/socket.io-go/internal/sync"
)

// This is a sync.WaitGroup with a WaitTimeout function. Use this for testing purposes.
type TimeoutWaiter struct {
	wg *sync.WaitGroup
}

func NewTimeoutWaiter(delta int) *TimeoutWaiter {
	wg := new(sync.WaitGroup)
	wg.Add(delta)
	return &TimeoutWaiter{
		wg: wg,
	}
}

func (w *TimeoutWaiter) Add(delta int) { w.wg.Add(delta) }

func (w *TimeoutWaiter) Done() { w.wg.Done() }

func (w *TimeoutWaiter) Wait() { w.wg.Wait() }

func (w *TimeoutWaiter) WaitTimeout(timeout time.Duration) (timedout bool) {
	c := make(chan struct{})

	go func() {
		defer close(c)
		w.wg.Wait()
	}()

	select {
	case <-c:
		return false
	case <-time.After(timeout):
		return true
	}
}
