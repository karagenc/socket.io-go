package utils

import (
	"fmt"
	"testing"
	"time"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/tomruk/socket.io-go/internal/sync"
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

func (w *TestWaiter) Add(delta int) { w.wg.Add(delta) }

func (w *TestWaiter) Done() { w.wg.Done() }

func (w *TestWaiter) Wait() { w.wg.Wait() }

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

type TestWaiterString struct {
	wg      *sync.WaitGroup
	strings mapset.Set[string]
}

func NewTestWaiterString() *TestWaiterString {
	wg := new(sync.WaitGroup)
	return &TestWaiterString{
		wg:      wg,
		strings: mapset.NewSet[string](),
	}
}

func (w *TestWaiterString) Add(s string) {
	w.strings.Add(s)
	w.wg.Add(1)
}

func (w *TestWaiterString) Done(s string) {
	if !w.strings.Contains(s) {
		panic(fmt.Errorf("TestWaiterString: Done was already called on '%s'", s))
	}
	w.strings.Remove(s)
	w.wg.Done()
}

func (w *TestWaiterString) Wait() { w.wg.Wait() }

func (w *TestWaiterString) WaitTimeout(t *testing.T, timeout time.Duration) (timedout bool) {
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
