package eio

import (
	"sync"
	"testing"
	"time"
)

const DefaultTestWaitTimeout = time.Second * 12

// This is a sync.WaitGroup with a WaitTimeout function. Use this for testing purposes.
type TestWaiter struct {
	wg *sync.WaitGroup
}

func NewTestWaiter(delta int) *TestWaiter {
	wg := new(sync.WaitGroup)
	wg.Add(delta)
	return &TestWaiter{
		wg: wg,
	}
}

func (w *TestWaiter) Add(delta int) {
	w.wg.Add(delta)
}

func (w *TestWaiter) Done() {
	w.wg.Done()
}

func (w *TestWaiter) Wait() {
	w.wg.Wait()
}

func (w *TestWaiter) WaitTimeout(t *testing.T, timeout time.Duration) (timedout bool) {
	c := make(chan struct{})

	go func() {
		defer close(c)
		w.wg.Wait()
	}()

	select {
	case <-c:
		return false
	case <-time.After(timeout):
		t.Error("timeout exceeded")
		return true
	}
}
