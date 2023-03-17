package sio

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestClientAck(t *testing.T) {
	server, _, manager := newTestServerAndClient(nil, nil)
	socket := manager.Socket("/", nil)
	socket.Connect()
	replied := sync.WaitGroup{}
	replied.Add(1)

	socket.OnConnect(func() {
		go func() {
			time.Sleep(1 * time.Second)
			socket.Emit("ack", "hello", func(reply string) {
				assert.Equal(t, "hi", reply)
				replied.Done()
			})
		}()
	})

	server.OnConnection(func(socket ServerSocket) {
		socket.OnEvent("ack", func(message string, ack func(reply string)) {
			assert.Equal(t, "hello", message)
			ack("hi")
		})
	})
	replied.Wait()
}
