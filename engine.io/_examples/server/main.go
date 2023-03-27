package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/tomruk/socket.io-go/internal/sync"

	eio "github.com/tomruk/socket.io-go/engine.io"
	"github.com/tomruk/socket.io-go/engine.io/parser"
)

const addr = "127.0.0.1:3000"

var (
	server *http.Server

	sockets   []eio.ServerSocket
	socketsMu sync.RWMutex
)

func onSocket(socket eio.ServerSocket) *eio.Callbacks {
	fmt.Printf("New socket connected: %s\n", socket.ID())
	addSocket(socket)

	sendTextMessage(socket, "Hello from server")

	return &eio.Callbacks{
		OnPacket: func(packets ...*parser.Packet) {
			for _, packet := range packets {
				if packet.Type == parser.PacketTypeMessage {
					fmt.Printf("Message received: %s\n", packet.Data)
				}
			}
		},
		OnError: func(err error) {
			fmt.Printf("Socket error: %v\n", err)
		},
		OnClose: func(reason eio.Reason, err error) {
			if err == nil {
				fmt.Printf("Socket closed: %s\n", reason)
			} else {
				fmt.Printf("Socket closed: %s | Error: %v\n", reason, err)
			}
		},
	}
}

func logServerError(err error) {
	log.Printf("Server error: %v\n", err)
}

func main() {
	io := eio.NewServer(onSocket, &eio.ServerConfig{
		Authenticator: authenticator,
		OnError:       logServerError,
	})

	err := io.Run()
	if err != nil {
		log.Fatalln(err)
	}

	fs := http.FileServer(http.Dir("public"))
	router := http.NewServeMux()

	if allowOrigin == "" {
		// Make sure to have a slash at the end of the URL.
		// Otherwise instead of matching with this handler, requests might match with a file that has an engine.io prefix (such as engine.io.min.js).
		router.Handle("/engine.io/", io)
	} else {
		if !strings.HasPrefix(allowOrigin, "http://") {
			allowOrigin = "http://" + allowOrigin
		}

		fmt.Printf("ALLOW_ORIGIN is set to: %s\n", allowOrigin)
		h := corsMiddleware(io, allowOrigin)

		// Make sure to have a slash at the end of the URL.
		// Otherwise instead of matching with this handler, requests might match with a file that has an engine.io prefix (such as engine.io.min.js).
		router.Handle("/engine.io/", h)
	}

	router.Handle("/", fs)

	fmt.Printf("Listening on: %s\n", addr)

	go userInput()

	server = &http.Server{
		Addr:    addr,
		Handler: router,

		// It is always a good practice to set timeouts.
		ReadTimeout: 120 * time.Second,
		IdleTimeout: 120 * time.Second,

		// HTTPWriteTimeout returns PollTimeout + 10 seconds (an extra time to write the response).
		// You should either set this timeout to 0 (infinite) or some value greater than the PollTimeout.
		// Otherwise poll requests may fail.
		WriteTimeout: io.HTTPWriteTimeout(),
	}

	err = server.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		log.Fatalln(err)
	}
}

func userInput() {
	defer func() {
		socketsMu.RLock()
		defer socketsMu.RUnlock()

		for _, socket := range sockets {
			socket.Close()
		}

		server.Shutdown(context.Background())
	}()

	fmt.Printf("Type your message and press enter to broadcast it.\n\n")

	for {
		reader := bufio.NewReader(os.Stdin)
		text, err := reader.ReadString('\n')
		if err != nil {
			fmt.Printf("User input error: %v\n", err)
			break
		}
		text = strings.TrimRight(text, "\r\n")

		if text == "exit" {
			break
		}

		sendTextMessageToAll(text)
	}
}

func addSocket(socket eio.ServerSocket) {
	socketsMu.Lock()
	defer socketsMu.Unlock()
	sockets = append(sockets, socket)
}

// A little helper function to send a string message to all sockets with no fuss.
func sendTextMessageToAll(message string) {
	socketsMu.RLock()
	defer socketsMu.RUnlock()

	for _, socket := range sockets {
		go sendTextMessage(socket, message)
	}
}

// A little helper function to send a string message with no fuss.
func sendTextMessage(socket eio.ServerSocket, message string) {
	packet, err := parser.NewPacket(parser.PacketTypeMessage, false, []byte(message))
	if err != nil {
		log.Fatalln(err)
	}
	socket.Send(packet)
}
