package sio

import (
	"testing"

	"github.com/tomruk/socket.io-go/internal/utils"
)

func TestNamespace(t *testing.T) {
	t.Run("should fire a `connect` event", func(t *testing.T) {
		io, _, manager, close := newTestServerAndClient(t, nil, nil)
		socket := manager.Socket("/", nil)
		tw := utils.NewTestWaiter(1)

		io.OnConnection(func(socket ServerSocket) {
			tw.Done()
		})
		socket.Connect()
		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		close()
	})

	t.Run(`should be able to equivalently start with "" or "/" on server`, func(t *testing.T) {
		io, _, manager, close := newTestServerAndClient(t, nil, nil)
		socket := manager.Socket("/", nil)
		tw := utils.NewTestWaiterString()
		tw.Add("/abc")
		tw.Add("")

		io.Of("/abc").OnConnection(func(socket ServerSocket) {
			tw.Done("/abc")
		})
		io.Of("").OnConnection(func(socket ServerSocket) {
			tw.Done("")
		})

		manager.Socket("/abc", nil).Connect()
		socket.Connect()
		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		close()
	})

	t.Run(`should be equivalent for "" and "/" on client`, func(t *testing.T) {
		io, _, manager, close := newTestServerAndClient(t, nil, nil)
		socket := manager.Socket("", nil)
		tw := utils.NewTestWaiter(1)

		io.Of("/").OnConnection(func(socket ServerSocket) {
			tw.Done()
		})
		socket.Connect()

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		close()
	})

	t.Run("should work with `of` and many sockets", func(t *testing.T) {
		io, _, manager, close := newTestServerAndClient(t, nil, nil)
		socket := manager.Socket("/", nil)
		tw := utils.NewTestWaiterString()
		tw.Add("/chat")
		tw.Add("/news")
		tw.Add("/")

		io.Of("/chat").OnConnection(func(socket ServerSocket) {
			tw.Done("/chat")
		})
		io.Of("/news").OnConnection(func(socket ServerSocket) {
			tw.Done("/news")
		})
		io.OnConnection(func(socket ServerSocket) {
			tw.Done("/")
		})
		manager.Socket("/chat", nil).Connect()
		manager.Socket("/news", nil).Connect()
		socket.Connect()

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		close()
	})

	t.Run("should work with `of` second param", func(t *testing.T) {
		io, _, manager, close := newTestServerAndClient(t, nil, nil)
		socket := manager.Socket("/news", nil)
		tw := utils.NewTestWaiter(2)

		io.Of("/news").OnConnection(func(socket ServerSocket) {
			tw.Done()
		})
		io.Of("/news").OnConnection(func(socket ServerSocket) {
			tw.Done()
		})
		socket.Connect()

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		close()
	})
}
