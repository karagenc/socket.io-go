package sio

import (
	"testing"

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
		io.Use(func(socket ServerSocket, handshake *Handshake) error {
			runMu.Lock()
			run++
			runMu.Unlock()
			return nil
		})
		io.Use(func(socket ServerSocket, handshake *Handshake) error {
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
}
