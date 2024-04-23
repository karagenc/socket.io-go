package main

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/pflag"
	sio "github.com/tomruk/socket.io-go"
)

const defaultURL = "http://127.0.0.1:3000/socket.io"

func main() {
	username := pflag.StringP("username", "u", "", "Username")
	url := pflag.StringP("connect", "c", defaultURL, "URL to connect to")
	pflag.Parse()

	config := &sio.ManagerConfig{
		ReconnectionAttempts: 5,
	}
	manager := sio.NewManager(*url, config)
	socket := manager.Socket("/", nil)

	term, exitFunc, err := initTerm(socket)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}

	if *username == "" {
		fmt.Fprintln(term, "Username cannot be empty. Use -u/--username to set the username.")
		exitFunc(1)
	}

	socket.OnConnect(func() {
		fmt.Fprintf(term, "Connected. SID: %s\n", socket.ID())
	})
	manager.OnReconnect(func(attempt uint32) {
		fmt.Fprintf(term, "Reconnected. Number of attempts: %d\n", attempt)
	})
	socket.OnConnectError(func(err error) {
		fmt.Fprintf(term, "Connect error: %v\n", err)
	})

	socket.OnEvent("login", func(data struct {
		NumUsers int `json:"numUsers"`
	}) {
		fmt.Fprintf(term, "You are logged in. Number of users is now: %d\n", data.NumUsers)
	})

	socket.OnEvent("user joined", func(data struct {
		Username string `json:"username"`
		NumUsers int    `json:"numUsers"`
	}) {
		color := getUsernameColor(data.Username)
		fmt.Fprintf(term, "%s is joined. Number of users is now: %d\n", color.Sprint(data.Username), data.NumUsers)
	})
	socket.OnEvent("user left", func(data struct {
		Username string `json:"username"`
		NumUsers int    `json:"numUsers"`
	}) {
		color := getUsernameColor(data.Username)
		fmt.Fprintf(term, "%s has left. Number of users is now: %d\n", color.Sprint(data.Username), data.NumUsers)
	})

	socket.OnEvent("new message", func(data struct {
		Username string `json:"username"`
		Message  string `json:"message"`
	}) {
		color := getUsernameColor(data.Username)
		fmt.Fprintf(term, "%s: %s\n", color.Sprint(data.Username), data.Message)
	})

	// This will be emitted after the socket is connected.
	socket.Emit("add user", *username)

	socket.Connect()

	for {
		line, err := term.ReadLine()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Fprintf(term, "Error: %v\n", err)
			exitFunc(1)
		}
		if line == "" {
			continue
		}
		socket.Emit("new message", string(line))
	}
	exitFunc(0)
}
