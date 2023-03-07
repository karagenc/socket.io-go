package main

import (
	"fmt"
	"time"

	sio "github.com/tomruk/socket.io-go"
)

const url = "http://127.0.0.1:3000/socket.io"

func main() {
	manager := sio.NewManager(url, nil)
	socket := manager.Socket("/", nil)

	fmt.Println("Init")

	socket.On("connect", func() {
		fmt.Println("Connected!")
	})

	socket.On("reconnect", func() {
		fmt.Printf("reconnect happened\n")
	})

	socket.On("connect_error", func(err error) {
		fmt.Printf("connect error: %s\n", err)
	})

	//socket.Emit("echo", "Hello!")
	socket.On("e", func(message string) {
		fmt.Println(message)
	})

	socket.Connect()
	manager.Open()

	time.Sleep(time.Second * 10)
}
