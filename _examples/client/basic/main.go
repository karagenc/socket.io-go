package main

import (
	"fmt"
	"time"

	sio "github.com/tomruk/socket.io-go"
)

const url = "http://127.0.0.1:3000/socket.io"

func main() {
	client := sio.NewClient(url, nil)
	socket := client.Socket("/")

	socket.On("connect", func() {
		fmt.Println("Connected!")
	})

	socket.Emit("echo", "Hello!", func(message string) {
		fmt.Printf("ACK: %s\n", message)
	})

	time.Sleep(time.Second * 2)
}
