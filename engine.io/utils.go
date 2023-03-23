package eio

import (
	"sync"
	"testing"
	"time"

	"github.com/tomruk/socket.io-go/engine.io/parser"
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

type TestSocket struct {
	id       string
	Closed   bool
	SendFunc func(packets ...*parser.Packet)
}

func NewTestSocket(id string) *TestSocket {
	return &TestSocket{
		id:       id,
		SendFunc: func(packets ...*parser.Packet) {},
	}
}

// Session ID (sid)
func (s *TestSocket) ID() string { return s.id }

func (s *TestSocket) PingInterval() time.Duration { return defaultPingInterval }
func (s *TestSocket) PingTimeout() time.Duration  { return defaultPingTimeout }

// Name of the current transport
func (s *TestSocket) TransportName() string { return "polling" }

func (s *TestSocket) Send(packets ...*parser.Packet) {
	s.SendFunc(packets...)
}

func (s *TestSocket) Close() {
	s.Closed = true
}
