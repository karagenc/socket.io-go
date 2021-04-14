package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
	eio "github.com/tomruk/socket.io-go/engine.io"
)

const addr = "127.0.0.1:3000"

var (
	server *http.Server

	sockets   []eio.Socket
	socketsMu sync.RWMutex
)

func onSocket(socket eio.Socket) *eio.Callbacks {
	fmt.Printf("New socket connected: %s\n", socket.ID())
	addSocket(socket)

	socket.SendMessage([]byte("Hello from server"), false)

	return &eio.Callbacks{
		OnMessage: func(data []byte, isBinary bool) {
			fmt.Printf("Message received: %s\n", data)
		},
		OnError: func(err error) {
			fmt.Printf("Socket error: %v\n", err)
		},
		OnClose: func(reason string, err error) {
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
		log.Fatal(err)
	}

	fs := http.FileServer(http.Dir("public"))
	router := mux.NewRouter()

	if allowOrigin == "" {
		// Make sure to have a slash at the end of the URL.
		// Otherwise instead of matching with this handler, requests might match with a file that has an engine.io prefix (such as engine.io.min.js).
		router.PathPrefix("/engine.io/").Handler(io)
	} else {
		if !strings.HasPrefix(allowOrigin, "http://") {
			allowOrigin = "http://" + allowOrigin
		}

		fmt.Printf("ALLOW_ORIGIN is set to: %s\n", allowOrigin)
		h := corsMiddleware(io, allowOrigin)

		// Make sure to have a slash at the end of the URL.
		// Otherwise instead of matching with this handler, requests might match with a file that has an engine.io prefix (such as engine.io.min.js).
		router.PathPrefix("/engine.io/").Handler(h)
	}

	router.PathPrefix("/").Handler(fs)

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
		log.Fatal(err)
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

		sendMessageToAll(text)
	}
}

func addSocket(socket eio.Socket) {
	socketsMu.Lock()
	defer socketsMu.Unlock()
	sockets = append(sockets, socket)
}

func sendMessageToAll(message string) {
	socketsMu.RLock()
	defer socketsMu.RUnlock()

	for _, socket := range sockets {
		go socket.SendMessage([]byte(message), false)
	}
}
