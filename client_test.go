package sio

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	eio "github.com/tomruk/socket.io-go/engine.io"
)

func TestClientAck(t *testing.T) {
	server, _, manager := newTestServerAndClient(nil, &ManagerConfig{
		EIO: eio.ClientConfig{
			Transports: []string{"polling"},
		},
	})
	socket := manager.Socket("/", nil)
	socket.Connect()
	replied := sync.WaitGroup{}
	replied.Add(5)

	socket.OnConnect(func() {
		for i := 0; i < 5; i++ {
			socket.Emit("ack", "hello", func(reply string) {
				fmt.Println("ack")
				assert.Equal(t, "hi", reply)
				replied.Done()
			})
		}
	})

	server.OnConnection(func(socket ServerSocket) {
		socket.OnEvent("ack", func(message string, ack func(reply string)) {
			fmt.Printf("event: %s\n", message)
			assert.Equal(t, "hello", message)
			ack("hi")
		})
	})
	replied.Wait()
}
