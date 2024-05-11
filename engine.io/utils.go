package eio

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/tomruk/socket.io-go/internal/sync"

	"github.com/tomruk/socket.io-go/engine.io/parser"
	"github.com/tomruk/socket.io-go/engine.io/transport"
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

func (w *TestWaiterString) Wait() {
	w.wg.Wait()
}

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

func (s *TestSocket) Send(packets ...*parser.Packet) { s.SendFunc(packets...) }

func (s *TestSocket) Close() { s.Closed = true }

type testServerTransport struct {
	callbacks *transport.Callbacks
}

var _ ServerTransport = newTestServerTransport()

func newTestServerTransport() *testServerTransport {
	return &testServerTransport{callbacks: transport.NewCallbacks()}
}

func (t *testServerTransport) Name() string { return "fake" }

func (t *testServerTransport) Handshake(
	_ *parser.Packet,
	w http.ResponseWriter,
	r *http.Request,
) (string, error) {
	return "", nil
}

func (t *testServerTransport) Callbacks() *transport.Callbacks { return t.callbacks }

func (t *testServerTransport) PostHandshake(handshakePacket *parser.Packet) {}

func (t *testServerTransport) ServeHTTP(w http.ResponseWriter, r *http.Request) {}

func (t *testServerTransport) QueuedPackets() []*parser.Packet { return nil }

func (t *testServerTransport) Send(packets ...*parser.Packet) {}

func (t *testServerTransport) Discard() {}
func (t *testServerTransport) Close()   {}
