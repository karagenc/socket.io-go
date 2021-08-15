package main

import (
	"fmt"
	"time"

	sio "github.com/tomruk/socket.io-go"
)

var (
	client = sio.NewClient("http://127.0.0.1:3000/socket.io", &sio.ClientConfig{
		PreventAutoConnect: true,
		AuthData: &authData{
			Token: "12345",
		},
	})

	socket = client.Socket("/")
)

type authData struct {
	Token string `json:"token"`
}

func main() {
	socket.On("echo", func(message string) (string, string) {
		fmt.Printf("Echo received: %s\n", message)
		return "Heyyo!", "Yaay!"
	})

	socket.On("binecho", func(message sio.Binary) (string, string) {
		fmt.Printf("Binary echo received: %d %d\n", message[0], message[1])
		return "Heyyo!", "Yaay!"
	})

	socket.On("connect", func() {
		fmt.Println("Connected!")
	})

	socket.Connect()

	socket.Emit("withack", "Hello! Send me an ack", func(message string) {
		fmt.Printf("ACK: %s\n", message)
	})

	//socket.Emit("binecho", "HEY!", sio.Binary{13, 37})

	for {
		socket.Emit("echo", "Wohooo!")
		time.Sleep(1 * time.Second)
	}
}
