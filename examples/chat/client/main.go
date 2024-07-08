package main

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/quic-go/webtransport-go"
	"github.com/spf13/pflag"
	sio "github.com/tomruk/socket.io-go"
	eio "github.com/tomruk/socket.io-go/engine.io"
	"nhooyr.io/websocket"
)

const defaultURL = "http://127.0.0.1:3000/socket.io"

func main() {
	username := pflag.StringP("username", "u", "", "Username")
	url := pflag.StringP("connect", "c", defaultURL, "URL to connect to")
	pflag.Parse()

	term, typing, exitFunc, err := initTerm()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}

	config := &sio.ManagerConfig{
		EIO: eio.ClientConfig{
			UpgradeDone: func(transportName string) {
				fmt.Fprintf(term, "Transport upgraded to: %s\n", transportName)
			},
			HTTPTransport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true, // DO NOT USE in production. This is for self-signed TLS certificate to work.
				},
			},
			WebSocketDialOptions: &websocket.DialOptions{
				HTTPClient: &http.Client{
					Transport: &http.Transport{
						TLSClientConfig: &tls.Config{
							InsecureSkipVerify: true, // DO NOT USE in production. This is for self-signed TLS certificate to work.
						},
					},
				},
			},
			WebTransportDialer: &webtransport.Dialer{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true, // DO NOT USE in production. This is for self-signed TLS certificate to work.
				},
			},
		},
	}

	manager := sio.NewManager(*url, config)
	socket := manager.Socket("/", nil)
	typing.socket = socket

	if *username == "" {
		fmt.Fprintln(term, "Username cannot be empty. Use -u/--username to set the username.")
		exitFunc(1)
	}

	socket.OnConnect(func() {
		fmt.Fprintln(term, "Connected")
	})
	manager.OnError(func(err error) {
		fmt.Fprintf(term, "Error: %v\n", err)
	})
	manager.OnReconnect(func(attempt uint32) {
		fmt.Fprintf(term, "Reconnected. Number of attempts so far: %d\n", attempt)
	})
	socket.OnConnectError(func(err any) {
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
