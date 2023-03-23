package sio

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClientAck(t *testing.T) {
	server, _, manager := newTestServerAndClient(t, nil, nil)
	socket := manager.Socket("/", nil)
	socket.Connect()
	replied := sync.WaitGroup{}
	replied.Add(5)

	manager.OnError(func(err error) {
		t.Fatal(err)
	})

	socket.OnConnect(func() {
		for i := 0; i < 5; i++ {
			fmt.Println("Emitting to server")
			socket.Emit("ack", "hello", func(reply string) {
				defer replied.Done()
				fmt.Println("ack")
				assert.Equal(t, "hi", reply)
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
