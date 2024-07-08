package sio

import (
	"fmt"
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
}
