package main

import (
	"time"

	sio "github.com/tomruk/socket.io-go"
)

var (
	client = sio.NewClient("http://127.0.0.1:3000/socket.io", &sio.ClientConfig{
		PreventAutoConnect: true,
	})

	socket = client.Socket("/")
)

func main() {
	socket.Connect()

	for {
		socket.Emit("echo", "Wohooo!")
		time.Sleep(1 * time.Second)
	}
}
