package main

import (
	"fmt"
	"sync"

	sio "github.com/tomruk/socket.io-go"
)

type api struct {
	numUsers   int
	numUsersMu sync.Mutex

	usernames   map[sio.ServerSocket]string
	usernamesMu sync.Mutex
}

func newAPI() *api {
	return &api{
		usernames: make(map[sio.ServerSocket]string),
	}
}

func (a *api) setup(root *sio.Namespace) {
	root.OnConnection(func(socket sio.ServerSocket) {
		fmt.Printf("Socket with SID %s is connected\n", socket.ID())

		socket.OnEvent("new message", func(data string) {
			socket.Broadcast().Emit("new message", struct {
				Username string `json:"username"`
				Message  string `json:"message"`
			}{
				Username: a.username(socket),
				Message:  data,
			})
		})

		socket.OnEvent("add user", func(username string) {
			a.usernamesMu.Lock()
			defer a.usernamesMu.Unlock()
			_, ok := a.usernames[socket]
			if ok {
				return
			}
			a.usernames[socket] = username
			a.numUsersMu.Lock()
			a.numUsers++
			numUsers := a.numUsers
			a.numUsersMu.Unlock()
			fmt.Printf("New user: %s\n", username)

			socket.Emit("login", struct {
				NumUsers int `json:"numUsers"`
			}{
				NumUsers: numUsers,
			})

			socket.Broadcast().Emit("user joined", struct {
				Username string `json:"username"`
				NumUsers int    `json:"numUsers"`
			}{
				Username: username,
				NumUsers: numUsers,
			})
		})

		socket.OnEvent("typing", func() {
			socket.Broadcast().Emit("typing", struct {
				Username string `json:"username"`
			}{
				Username: a.username(socket),
			})
		})
		socket.OnEvent("stop typing", func() {
			socket.Broadcast().Emit("stop typing", struct {
				Username string `json:"username"`
			}{
				Username: a.username(socket),
			})
		})

		socket.OnDisconnect(func(reason sio.Reason) {
			a.usernamesMu.Lock()
			defer a.usernamesMu.Unlock()
			_, ok := a.usernames[socket]
			if ok {
				return
			}

			a.numUsersMu.Lock()
			a.numUsers--
			numUsers := a.numUsers
			a.numUsersMu.Unlock()

			socket.Broadcast().Emit("user left", struct {
				Username string `json:"username"`
				NumUsers int    `json:"numUsers"`
			}{
				Username: a.username(socket),
				NumUsers: numUsers,
			})

		})
	})
}

func (a *api) username(socket sio.ServerSocket) (username string) {
	a.usernamesMu.Lock()
	username = a.usernames[socket]
	a.usernamesMu.Unlock()
	return
}
