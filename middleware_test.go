package sio

import (
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/mitchellh/mapstructure"
	"github.com/stretchr/testify/assert"
	"github.com/tomruk/socket.io-go/internal/sync"
	"github.com/tomruk/socket.io-go/internal/utils"
)

func TestMiddleware(t *testing.T) {
	t.Run("should call functions", func(t *testing.T) {
		io, _, manager, close := newTestServerAndClient(
			t,
			nil,
			nil,
		)
		tw := utils.NewTestWaiter(1)

		run := 0
		runMu := sync.Mutex{}
		io.Use(func(socket ServerSocket, handshake *Handshake) any {
			runMu.Lock()
			run++
			runMu.Unlock()
			return nil
		})
		io.Use(func(socket ServerSocket, handshake *Handshake) any {
			runMu.Lock()
			run++
			runMu.Unlock()
			return nil
		})

		socket := manager.Socket("/", nil)
		socket.OnConnect(func() {
			runMu.Lock()
			assert.Equal(t, 2, run)
			runMu.Unlock()
			tw.Done()
		})
		socket.Connect()

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		close()
	})

	t.Run("should pass errors", func(t *testing.T) {
		io, _, manager, close := newTestServerAndClient(
			t,
			nil,
			nil,
		)
		tw := utils.NewTestWaiter(1)

		io.Use(func(socket ServerSocket, handshake *Handshake) any {
			return fmt.Errorf("authentication error")
		})
		io.Use(func(socket ServerSocket, handshake *Handshake) any {
			return fmt.Errorf("nope")
		})

		socket := manager.Socket("/", nil)
		socket.OnConnect(func() {
			t.FailNow()
		})
		socket.OnConnectError(func(err any) {
			e := err.(error)
			assert.Equal(t, "authentication error", e.Error())
			tw.Done()
		})
		socket.Connect()

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		close()
	})

	t.Run("should pass an object", func(t *testing.T) {
		io, _, manager, close := newTestServerAndClient(
			t,
			nil,
			nil,
		)
		tw := utils.NewTestWaiter(1)

		type customError struct {
			Foo string `json:"foo"`
			Bar string `json:"bar"`
		}

		io.Use(func(socket ServerSocket, handshake *Handshake) any {
			err := &customError{
				Foo: "foo",
				Bar: "bar",
			}
			return err
		})
		io.Use(func(socket ServerSocket, handshake *Handshake) any {
			return fmt.Errorf("nope")
		})

		socket := manager.Socket("/", nil)
		socket.OnConnect(func() {
			t.FailNow()
		})
		socket.OnConnectError(func(e any) {
			c := &customError{}
			err := mapstructure.Decode(e, c)
			if err != nil {
				t.Fatal(err)
			}
			assert.Equal(t, "foo", c.Foo)
			assert.Equal(t, "bar", c.Bar)
			tw.Done()
		})
		socket.Connect()

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		close()
	})

	t.Run("should only call connection after fns", func(t *testing.T) {
		io, _, manager, close := newTestServerAndClient(
			t,
			nil,
			nil,
		)
		tw := utils.NewTestWaiter(1)

		ok := atomic.Bool{}
		io.Use(func(socket ServerSocket, handshake *Handshake) any {
			ok.Store(true)
			return nil
		})
		io.OnConnection(func(socket ServerSocket) {
			assert.True(t, ok.Load())
			tw.Done()
		})
		manager.Socket("/", nil).Connect()

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		close()
	})

	t.Run("should call functions in expected order", func(t *testing.T) {
		io, _, manager, close := newTestServerAndClient(
			t,
			nil,
			nil,
		)
		tw := utils.NewTestWaiter(1)

		result := []int{}
		resultMu := sync.Mutex{}
		io.Use(func(socket ServerSocket, handshake *Handshake) any {
			t.Fatal("should not happen")
			return nil
		})
		io.Of("/chat").Use(func(socket ServerSocket, handshake *Handshake) any {
			resultMu.Lock()
			result = append(result, 1)
			resultMu.Unlock()
			return nil
		})
		io.Of("/chat").Use(func(socket ServerSocket, handshake *Handshake) any {
			resultMu.Lock()
			result = append(result, 2)
			resultMu.Unlock()
			return nil
		})
		io.Of("/chat").Use(func(socket ServerSocket, handshake *Handshake) any {
			resultMu.Lock()
			result = append(result, 3)
			resultMu.Unlock()
			return nil
		})
		socket := manager.Socket("/chat", nil)
		socket.OnConnect(func() {
			resultMu.Lock()
			assert.Equal(t, []int{1, 2, 3}, result)
			resultMu.Unlock()
			tw.Done()
		})
		socket.Connect()

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		close()
	})

	t.Run("should work with a custom namespace", func(t *testing.T) {
		io, _, manager, close := newTestServerAndClient(
			t,
			nil,
			nil,
		)
		tw := utils.NewTestWaiter(2)

		io.Of("/")
		io.Of("/chat").Use(func(socket ServerSocket, handshake *Handshake) any {
			return nil
		})
		socket1 := manager.Socket("/", nil)
		socket1.OnConnect(func() {
			tw.Done()
		})
		socket1.Connect()
		socket2 := manager.Socket("/chat", nil)
		socket2.OnConnect(func() {
			tw.Done()
		})
		socket2.Connect()

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		close()
	})

	t.Run("should only set `connected` to true after the middleware execution", func(t *testing.T) {
		io, _, manager, close := newTestServerAndClient(
			t,
			nil,
			nil,
		)
		tw := utils.NewTestWaiter(2)

		io.Use(func(socket ServerSocket, handshake *Handshake) any {
			assert.False(t, socket.Connected())
			tw.Done()
			return nil
		})
		io.OnConnection(func(socket ServerSocket) {
			assert.True(t, socket.Connected())
			tw.Done()
		})
		manager.Socket("/", nil).Connect()

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		close()
	})
}
