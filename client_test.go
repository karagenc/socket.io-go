package sio

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	eio "github.com/karagenc/socket.io-go/engine.io"
	"github.com/karagenc/socket.io-go/internal/sync"
	"github.com/karagenc/socket.io-go/internal/utils"

	"github.com/stretchr/testify/assert"
)

func TestClient(t *testing.T) {
	t.Run("should authenticate", func(t *testing.T) {
		_, _, manager, close := newTestServerAndClient(t, nil, nil)
		socket := manager.Socket("/", nil).(*clientSocket)

		type S struct {
			Num int
		}
		s := &S{
			Num: 500,
		}

		err := socket.setAuth(s)
		if err != nil {
			t.Fatal(err)
		}

		s, ok := socket.Auth().(*S)
		assert.True(t, ok)
		assert.Equal(t, s.Num, 500)

		err = socket.setAuth("Donkey")
		assert.NotNil(t, err)

		assert.PanicsWithError(t, "sio: SetAuth: non-JSON data cannot be accepted. please provide a struct or map", func() {
			socket.SetAuth("Donkey")
		})

		close()
	})

	t.Run("should connect to a namespace after connection established", func(t *testing.T) {
		_, _, manager, close := newTestServerAndClient(
			t,
			&ServerConfig{
				AcceptAnyNamespace: true,
			},
			nil,
		)
		tw := utils.NewTestWaiter(1)
		socket := manager.Socket("/", nil)

		socket.OnConnect(func() {
			t.Log("/ connected")
			asdf := manager.Socket("/asdf", nil)
			asdf.OnConnect(func() {
				t.Log("/asdf connected")
				tw.Done()
			})
			asdf.Connect()
		})
		socket.Connect()

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		close()
	})

	t.Run("should be able to connect to a new namespace after connection gets closed", func(t *testing.T) {
		_, _, manager, close := newTestServerAndClient(
			t,
			&ServerConfig{
				AcceptAnyNamespace: true,
			},
			nil,
		)
		socket := manager.Socket("/", nil)
		tw := utils.NewTestWaiter(1)
		done := sync.OnceFunc(func() { tw.Done() })

		socket.OnConnect(func() {
			t.Log("/ connected")
			socket.Disconnect()
		})
		socket.OnDisconnect(func(reason Reason) {
			t.Logf("/ disconnected with reason: %s", reason)
			asdf := manager.Socket("/asdf", nil)
			asdf.OnConnect(func() {
				t.Log("/asdf connected")
				done()
			})
			t.Log("/asdf is connecting")
			asdf.Connect()
		})
		socket.Connect()

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		close()
	})

	t.Run("manager open without socket", func(t *testing.T) {
		server, _, manager, close := newTestServerAndClient(
			t,
			&ServerConfig{
				AcceptAnyNamespace: true,
				ConnectTimeout:     1000 * time.Millisecond,
			},
			&ManagerConfig{
				NoReconnection: true,
			},
		)
		tw := utils.NewTestWaiterString()
		tw.Add("OnOpen")
		tw.Add("OnClose")

		server.OnAnyConnection(func(namespace string, socket ServerSocket) {
			t.Fatalf("Connection to `%s` was received. This shouldn't have happened", namespace)
		})

		manager.OnOpen(func() {
			t.Log("Manager connection is established")
			tw.Done("OnOpen")
		})
		manager.OnClose(func(reason Reason, err error) {
			assert.Equal(t, Reason("transport close"), reason)
			tw.Done("OnClose")
		})
		manager.Open()

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		close()
	})

	t.Run("should reconnect by default", func(t *testing.T) {
		server, _, manager, close := newTestServerAndClient(
			t,
			nil,
			nil,
		)
		tw := utils.NewTestWaiter(1)
		socket := manager.Socket("/", nil)

		server.OnConnection(func(socket ServerSocket) {
			s := socket.(*serverSocket)
			// Abruptly close the connection.
			s.conn.eio.Close()
		})
		manager.OnReconnect(func(attempt uint32) {
			socket.Disconnect()
			tw.Done()
		})

		socket.Connect()
		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		close()
	})

	t.Run("should reconnect manually", func(t *testing.T) {
		_, _, manager, close := newTestServerAndClient(
			t,
			&ServerConfig{
				AcceptAnyNamespace: true,
			},
			&ManagerConfig{
				NoReconnection: true,
			},
		)
		tw := utils.NewTestWaiter(1)
		socket := manager.Socket("/", nil)

		socket.OnceConnect(func() {
			socket.Disconnect()
		})
		socket.OnceDisconnect(func(reason Reason) {
			socket.OnceConnect(func() {
				socket.Disconnect()
				tw.Done()
			})
			socket.Connect()
		})

		socket.Connect()
		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		close()
	})

	t.Run("should reconnect automatically after reconnecting manually", func(t *testing.T) {
		_, _, manager, close := newTestServerAndClient(
			t,
			&ServerConfig{
				AcceptAnyNamespace: true,
			},
			nil,
		)
		tw := utils.NewTestWaiter(1)
		socket := manager.Socket("/", nil)

		socket.OnceConnect(func() {
			socket.Disconnect()
		})
		socket.OnceDisconnect(func(reason Reason) {
			socket.Manager().OnceReconnect(func(attempt uint32) {
				socket.Disconnect()
				tw.Done()
			})
			socket.Connect()
			time.Sleep(500 * time.Millisecond)
			socket.Manager().eioMu.Lock()
			defer socket.Manager().eioMu.Unlock()
			// Call inside another goroutine to prevent eioMu to be locked more than once at the same goroutine.
			go socket.Manager().eio.Close()
		})

		socket.Connect()
		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		close()
	})

	t.Run("should attempt reconnects after a failed reconnect", func(t *testing.T) {
		var (
			reconnectionDelay    = 10 * time.Millisecond
			reconnectionDelayMax = 10 * time.Millisecond
		)
		_, _, manager, close := newTestServerAndClient(
			t,
			&ServerConfig{
				AcceptAnyNamespace: true,
			},
			&ManagerConfig{
				ReconnectionAttempts: 2,
				ReconnectionDelay:    &reconnectionDelay,
				ReconnectionDelayMax: &reconnectionDelayMax,
				EIO: eio.ClientConfig{
					Transports: []string{"polling"}, // To buy time by not waiting for +2 other transport's connection attempts.
				},
			},
		)
		close() // To force reconnect by preventing client from connecting.
		tw := utils.NewTestWaiter(1)

		socket := manager.Socket("/timeout", nil)
		manager.OnceReconnectFailed(func() {
			var (
				reconnects = 0
				mu         sync.Mutex
			)
			manager.OnReconnectAttempt(func(attempt uint32) {
				mu.Lock()
				reconnects++
				mu.Unlock()
			})
			manager.OnReconnectFailed(func() {
				mu.Lock()
				assert.Equal(t, 2, reconnects)
				mu.Unlock()
				socket.Disconnect()
				manager.Close()
				tw.Done()
			})
			socket.Connect()
		})
		socket.Connect()

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
	})

	t.Run("should stop reconnecting when force closed", func(t *testing.T) {
		var (
			reconnectionDelay    = 10 * time.Millisecond
			reconnectionDelayMax = 10 * time.Millisecond
		)
		_, _, manager, close := newTestServerAndClient(
			t,
			&ServerConfig{
				AcceptAnyNamespace: true,
			},
			&ManagerConfig{
				ReconnectionDelay:    &reconnectionDelay,
				ReconnectionDelayMax: &reconnectionDelayMax,
				EIO: eio.ClientConfig{
					Transports: []string{"polling"}, // To buy time by not waiting for +2 other transport's connection attempts.
				},
			},
		)
		tw := utils.NewTestWaiter(1)
		close() // To force error by preventing client from connecting.
		socket := manager.Socket("/", nil)
		manager.OnceReconnectAttempt(func(attempt uint32) {
			socket.Disconnect()
			manager.OnReconnectAttempt(func(attempt uint32) {
				t.FailNow()
			})
			time.Sleep(500 * time.Millisecond)
			tw.Done()
		})
		socket.Connect()

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
	})

	t.Run("should reconnect after stopping reconnection", func(t *testing.T) {
		var (
			reconnectionDelay    = 10 * time.Millisecond
			reconnectionDelayMax = 10 * time.Millisecond
		)
		_, _, manager, close := newTestServerAndClient(
			t,
			&ServerConfig{
				AcceptAnyNamespace: true,
			},
			&ManagerConfig{
				ReconnectionDelay:    &reconnectionDelay,
				ReconnectionDelayMax: &reconnectionDelayMax,
				EIO: eio.ClientConfig{
					Transports: []string{"polling"}, // To buy time by not waiting for +2 other transport's connection attempts.
				},
			},
		)
		tw := utils.NewTestWaiter(1)
		close() // To force error by preventing client from connecting.
		socket := manager.Socket("/", nil)
		manager.OnceReconnectAttempt(func(attempt uint32) {
			manager.OnReconnectAttempt(func(attempt uint32) {
				socket.Disconnect()
				tw.Done()
			})
			socket.Disconnect()
			socket.Connect()
			tw.Done()
		})
		socket.Connect()

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
	})

	t.Run("should stop reconnecting on a socket and keep to reconnect on another", func(t *testing.T) {
		var (
			reconnectionDelay    = 10 * time.Millisecond
			reconnectionDelayMax = 10 * time.Millisecond
		)
		io, ts, manager, close := newTestServerAndClient(
			t,
			&ServerConfig{
				AcceptAnyNamespace: true,
				EIO: eio.ServerConfig{
					PingTimeout:  3000 * time.Millisecond,
					PingInterval: 1000 * time.Millisecond,
				},
			},
			&ManagerConfig{
				ReconnectionDelay:    &reconnectionDelay,
				ReconnectionDelayMax: &reconnectionDelayMax,
				EIO: eio.ClientConfig{
					Transports: []string{"polling"}, // To buy time by not waiting for +2 other transport's connection attempts.
				},
			},
		)
		tw := utils.NewTestWaiter(1)
		socket1 := manager.Socket("/", nil)
		socket2 := manager.Socket("/asd", nil)
		manager.OnceReconnectAttempt(func(attempt uint32) {
			socket1.OnConnect(func() {
				t.FailNow()
			})
			socket2.OnConnect(func() {
				time.Sleep(500 * time.Millisecond)
				socket2.Disconnect()
				manager.Close()
				tw.Done()
			})
			socket1.Disconnect()
		})
		socket1.Connect()
		socket2.Connect()

		go func() {
			time.Sleep(1000 * time.Millisecond)
			ts.Close()
			time.Sleep(5000 * time.Millisecond)
			hs := http.Server{
				Addr:    ts.Listener.Addr().String(),
				Handler: io,
			}
			err := hs.ListenAndServe()
			if err != nil && err != http.ErrServerClosed {
				panic(err)
			}
		}()

		tw.WaitTimeout(t, 20*time.Second)
		close()
	})

	t.Run("should try to reconnect twice and fail when requested two attempts with immediate timeout and reconnect enabled", func(t *testing.T) {
		var (
			reconnectionDelay    = 10 * time.Millisecond
			reconnectionDelayMax = 10 * time.Millisecond
		)
		_, _, manager, close := newTestServerAndClient(
			t,
			&ServerConfig{
				AcceptAnyNamespace: true,
			},
			&ManagerConfig{
				ReconnectionDelay:    &reconnectionDelay,
				ReconnectionDelayMax: &reconnectionDelayMax,
				ReconnectionAttempts: 2,
				EIO: eio.ClientConfig{
					Transports: []string{"polling"}, // To buy time by not waiting for +2 other transport's connection attempts.
				},
			},
		)
		close()
		tw := utils.NewTestWaiter(1)
		socket := manager.Socket("/timeout", nil)
		reconnects := 0
		reconnectsMu := sync.Mutex{}

		manager.OnReconnectAttempt(func(attempt uint32) {
			reconnectsMu.Lock()
			reconnects++
			reconnectsMu.Unlock()
		})
		manager.OnReconnectFailed(func() {
			reconnectsMu.Lock()
			assert.Equal(t, 2, reconnects)
			reconnectsMu.Unlock()
			socket.Disconnect()
			tw.Done()
		})
		socket.Connect()

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
	})

	t.Run("should fire reconnect_* events on manager", func(t *testing.T) {
		var (
			reconnectionDelay    = 10 * time.Millisecond
			reconnectionDelayMax = 10 * time.Millisecond
		)
		_, _, manager, close := newTestServerAndClient(
			t,
			&ServerConfig{
				AcceptAnyNamespace: true,
			},
			&ManagerConfig{
				ReconnectionDelay:    &reconnectionDelay,
				ReconnectionDelayMax: &reconnectionDelayMax,
				ReconnectionAttempts: 2,
				EIO: eio.ClientConfig{
					Transports: []string{"polling"}, // To buy time by not waiting for +2 other transport's connection attempts.
				},
			},
		)
		close()
		tw := utils.NewTestWaiter(1)
		socket := manager.Socket("/timeout_socket", nil)
		reconnects := 0
		reconnectsMu := sync.Mutex{}

		manager.OnReconnectAttempt(func(attempt uint32) {
			reconnectsMu.Lock()
			reconnects++
			reconnects := reconnects
			reconnectsMu.Unlock()
			assert.Equal(t, uint32(reconnects), attempt)
		})
		manager.OnReconnectFailed(func() {
			reconnectsMu.Lock()
			assert.Equal(t, 2, reconnects)
			reconnectsMu.Unlock()
			socket.Disconnect()
			tw.Done()
		})
		socket.Connect()

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
	})

	t.Run("should not try to reconnect and should form a connection when connecting to correct port with default timeout", func(t *testing.T) {
		var (
			reconnectionDelay    = 10 * time.Millisecond
			reconnectionDelayMax = 10 * time.Millisecond
		)
		_, _, manager, close := newTestServerAndClient(
			t,
			&ServerConfig{
				AcceptAnyNamespace: true,
			},
			&ManagerConfig{
				ReconnectionDelay:    &reconnectionDelay,
				ReconnectionDelayMax: &reconnectionDelayMax,
				EIO: eio.ClientConfig{
					Transports: []string{"polling"}, // To buy time by not waiting for +2 other transport's connection attempts.
				},
			},
		)
		tw := utils.NewTestWaiter(1)
		socket := manager.Socket("/valid", nil)

		manager.OnReconnectAttempt(func(attempt uint32) {
			socket.Disconnect()
			t.FailNow()
		})
		socket.OnConnect(func() {
			time.Sleep(1000 * time.Millisecond)
			socket.Disconnect()
			tw.Done()
		})
		socket.Connect()

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		close()
	})

	t.Run("should connect while disconnecting another socket", func(t *testing.T) {
		_, _, manager, close := newTestServerAndClient(
			t,
			&ServerConfig{
				AcceptAnyNamespace: true,
			},
			nil,
		)
		tw := utils.NewTestWaiter(1)
		socket1 := manager.Socket("/foo", nil)

		socket1.OnConnect(func() {
			socket2 := manager.Socket("/asd", nil)
			socket2.OnConnect(func() {
				tw.Done()
			})
			socket2.Connect()
			socket1.Disconnect()
		})
		socket1.Connect()

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		close()
	})

	t.Run("should not close the connection when disconnecting a single socket", func(t *testing.T) {
		_, _, manager, close := newTestServerAndClient(
			t,
			&ServerConfig{
				AcceptAnyNamespace: true,
			},
			nil,
		)
		tw := utils.NewTestWaiter(1)
		doneOnce := sync.OnceFunc(func() { tw.Done() })
		socket1 := manager.Socket("/foo", nil)
		socket2 := manager.Socket("/asd", nil)

		socket1.OnConnect(func() {
			socket2.Connect()
		})
		socket2.OnConnect(func() {
			socket2.OnDisconnect(func(reason Reason) {
				t.Fatal("should not happen for now")
			})
			socket1.Disconnect()
			time.Sleep(200 * time.Millisecond)
			socket2.OffDisconnect()
			manager.OnClose(func(reason Reason, err error) {
				doneOnce()
			})
			socket2.Disconnect()
		})
		socket1.Connect()

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		close()
	})

	t.Run("should have an accessible socket id equal to the server-side socket id (default namespace)", func(t *testing.T) {
		io, _, manager, close := newTestServerAndClient(
			t,
			&ServerConfig{
				AcceptAnyNamespace: true,
			},
			nil,
		)
		tw := utils.NewTestWaiter(1)
		socket := manager.Socket("/", nil)

		io.OnConnection(func(socket ServerSocket) {
			socket.OnEvent("getID", func(r func(id string)) {
				r(string(socket.ID()))
			})
		})
		socket.Connect()
		socket.Emit("getID", func(id string) {
			assert.Equal(t, socket.ID(), SocketID(id))
			assert.NotEqual(t, manager.eio.ID(), id)
			tw.Done()
		})

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		close()
	})

	t.Run("should have an accessible socket id equal to the server-side socket id (custom namespace)", func(t *testing.T) {
		io, _, manager, close := newTestServerAndClient(
			t,
			&ServerConfig{
				AcceptAnyNamespace: true,
			},
			nil,
		)
		tw := utils.NewTestWaiter(1)
		socket := manager.Socket("/foo", nil)

		io.Of("/foo").OnConnection(func(socket ServerSocket) {
			socket.OnEvent("getID", func(r func(id string)) {
				r(string(socket.ID()))
			})
		})
		socket.Connect()
		socket.Emit("getID", func(id string) {
			assert.Equal(t, socket.ID(), SocketID(id))
			assert.NotEqual(t, manager.eio.ID(), id)
			tw.Done()
		})

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		close()
	})

	t.Run("clears socket.id upon disconnection", func(t *testing.T) {
		_, _, manager, close := newTestServerAndClient(
			t,
			&ServerConfig{
				AcceptAnyNamespace: true,
			},
			nil,
		)
		tw := utils.NewTestWaiter(1)
		socket := manager.Socket("/", nil)

		socket.OnConnect(func() {
			socket.OnDisconnect(func(reason Reason) {
				assert.Equal(t, SocketID(""), socket.ID())
				tw.Done()
			})
			socket.Disconnect()
		})
		socket.Connect()

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		close()
	})

	t.Run("fire a connect_error event when the connection cannot be established", func(t *testing.T) {
		tw := utils.NewTestWaiter(1)
		manager := NewManager("http://localhost:9823", nil)
		socket := manager.Socket("/", nil)
		socket.OnConnectError(func(err any) {
			socket.Disconnect()
			tw.Done()
		})
		socket.Connect()

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
	})

	t.Run("doesn't fire a connect_error event when the connection is already established", func(t *testing.T) {
		_, _, manager, close := newTestServerAndClient(
			t,
			&ServerConfig{
				AcceptAnyNamespace: true,
			},
			&ManagerConfig{
				NoReconnection: true,
			},
		)
		tw := utils.NewTestWaiter(1)
		socket := manager.Socket("/", nil)

		socket.OnConnect(func() {
			manager.eioMu.Lock()
			go manager.eio.Close()
			manager.eioMu.Unlock()
			tw.Done()
		})
		socket.OnConnectError(func(err any) {
			t.Fatal("should not happen")
		})
		socket.Connect()

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		time.Sleep(500 * time.Millisecond)
		close()
	})

	t.Run("should change socket.id upon reconnection", func(t *testing.T) {
		var (
			reconnectionDelay    = 10 * time.Millisecond
			reconnectionDelayMax = 10 * time.Millisecond
		)
		io, ts, manager, close := newTestServerAndClient(
			t,
			&ServerConfig{
				AcceptAnyNamespace: true,
				EIO: eio.ServerConfig{
					PingTimeout:  3000 * time.Millisecond,
					PingInterval: 1000 * time.Millisecond,
				},
			},
			&ManagerConfig{
				ReconnectionDelay:    &reconnectionDelay,
				ReconnectionDelayMax: &reconnectionDelayMax,
				EIO: eio.ClientConfig{
					Transports: []string{"polling"}, // To buy time by not waiting for +2 other transport's connection attempts.
				},
			},
		)
		tw := utils.NewTestWaiterString()
		tw.Add("reconnect attempt check")
		reconnectAttemptCheck := sync.OnceFunc(func() { tw.Done("reconnect attempt check") })
		tw.Add("reconnect check")
		reconnectCheck := sync.OnceFunc(func() { tw.Done("reconnect check") })
		socket := manager.Socket("/", nil)

		socket.OnConnect(func() {
			id := socket.ID()
			manager.OnReconnectAttempt(func(attempt uint32) {
				assert.Equal(t, SocketID(""), socket.ID())
				reconnectAttemptCheck()
			})
			manager.OnReconnect(func(attempt uint32) {
				assert.NotEqual(t, socket.ID(), id)
				socket.Disconnect()
				reconnectCheck()
			})
		})
		socket.Connect()

		go func() {
			time.Sleep(1000 * time.Millisecond)
			ts.Close()
			time.Sleep(5000 * time.Millisecond)
			hs := http.Server{
				Addr:    ts.Listener.Addr().String(),
				Handler: io,
			}
			err := hs.ListenAndServe()
			if err != nil && err != http.ErrServerClosed {
				panic(err)
			}
		}()

		tw.WaitTimeout(t, 20*time.Second)
		close()
	})

	t.Run("should fire an error event on middleware failure from custom namespace", func(t *testing.T) {
		io, _, manager, close := newTestServerAndClient(
			t,
			&ServerConfig{
				AcceptAnyNamespace: true,
			},
			nil,
		)
		tw := utils.NewTestWaiter(1)
		socket := manager.Socket("/", nil)
		io.Use(func(socket ServerSocket, handshake *Handshake) any {
			return fmt.Errorf("auth failed (custom namespace)")
		})

		socket.OnConnectError(func(err any) {
			e := err.(error)
			assert.Equal(t, "auth failed (custom namespace)", e.Error())
			socket.Disconnect()
			tw.Done()
		})
		socket.Connect()

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		close()
	})

	t.Run("should not try to reconnect after a middleware failure", func(t *testing.T) {
		var (
			reconnectionDelay    = 10 * time.Millisecond
			reconnectionDelayMax = 10 * time.Millisecond
		)
		io, _, manager, close := newTestServerAndClient(
			t,
			&ServerConfig{
				AcceptAnyNamespace: true,
			},
			&ManagerConfig{
				ReconnectionDelay:    &reconnectionDelay,
				ReconnectionDelayMax: &reconnectionDelayMax,
			},
		)
		tw := utils.NewTestWaiter(1)
		socket := manager.Socket("/", nil)
		io.Use(func(socket ServerSocket, handshake *Handshake) any {
			return fmt.Errorf("auth failed (custom namespace)")
		})

		count := 0
		countMu := sync.Mutex{}
		socket.OnConnectError(func(err any) {
			countMu.Lock()
			count++
			countMu.Unlock()
			close() // Force reconnection
			tw.Done()
		})
		socket.Connect()

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		time.Sleep(500 * time.Millisecond)
		countMu.Lock()
		assert.Equal(t, 1, count)
		countMu.Unlock()
		close()
	})

	t.Run("should properly disconnect then reconnect", func(t *testing.T) {
		var (
			reconnectionDelay    = 10 * time.Millisecond
			reconnectionDelayMax = 10 * time.Millisecond
		)
		_, _, manager, close := newTestServerAndClient(
			t,
			&ServerConfig{
				AcceptAnyNamespace: true,
			},
			&ManagerConfig{
				ReconnectionDelay:    &reconnectionDelay,
				ReconnectionDelayMax: &reconnectionDelayMax,
				EIO: eio.ClientConfig{
					Transports: []string{"websocket"},
				},
			},
		)
		tw := utils.NewTestWaiter(1)
		doneOnce := sync.OnceFunc(func() { tw.Done() })
		socket := manager.Socket("/", nil)

		count := 0
		countMu := sync.Mutex{}
		socket.OnceConnect(func() {
			socket.Disconnect()
			socket.Connect()
		})
		socket.OnDisconnect(func(reason Reason) {
			countMu.Lock()
			count++
			countMu.Unlock()
			doneOnce()
		})
		socket.Connect()

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		time.Sleep(500 * time.Millisecond)
		countMu.Lock()
		assert.Equal(t, 1, count)
		countMu.Unlock()
		close()
	})

	t.Run("should throw on reserved event", func(t *testing.T) {
		_, _, manager, close := newTestServerAndClient(
			t,
			&ServerConfig{
				AcceptAnyNamespace: true,
			},
			nil,
		)
		socket := manager.Socket("/", nil)
		assert.Panics(t, func() {
			socket.Emit("disconnecting", "goodbye")
		})
		close()
	})

	t.Run("should emit events in order", func(t *testing.T) {
		_, _, manager, close := newTestServerAndClient(
			t,
			&ServerConfig{
				AcceptAnyNamespace: true,
			},
			nil,
		)
		tw := utils.NewTestWaiter(1)
		socket := manager.Socket("/", nil)

		i := 0
		iMu := sync.Mutex{}
		socket.OnConnect(func() {
			socket.Emit("echo", "second", func() {
				iMu.Lock()
				i++
				i := i
				iMu.Unlock()
				assert.Equal(t, i, 2)
				socket.Disconnect()
				tw.Done()
			})
		})
		socket.Emit("echo", "first", func() {
			iMu.Lock()
			i++
			i := i
			iMu.Unlock()
			assert.Equal(t, i, 1)
		})
		socket.Connect()

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		close()
	})

	t.Run("should discard a volatile packet when the socket is not connected", func(t *testing.T) {
		_, _, manager, close := newTestServerAndClient(
			t,
			&ServerConfig{
				AcceptAnyNamespace: true,
			},
			nil,
		)
		tw := utils.NewTestWaiter(1)
		socket := manager.Socket("/", nil)

		socket.Volatile().Emit("getID", func() {
			t.Fatal("should not happen")
		})
		socket.Emit("getID", func() {
			socket.Disconnect()
			tw.Done()
		})
		socket.Connect()

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		time.Sleep(200 * time.Millisecond)
		close()
	})

	t.Run("should send a volatile packet when the socket is connected", func(t *testing.T) {
		_, _, manager, close := newTestServerAndClient(
			t,
			&ServerConfig{
				AcceptAnyNamespace: true,
			},
			nil,
		)
		tw := utils.NewTestWaiter(1)
		socket := manager.Socket("/", nil)

		socket.OnConnect(func() {
			socket.Volatile().Emit("getID", func() {
				tw.Done()
			})
		})
		socket.Connect()

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		close()
	})

	t.Run("should timeout after the given delay when socket is not connected", func(t *testing.T) {
		_, _, manager, close := newTestServerAndClient(
			t,
			&ServerConfig{
				AcceptAnyNamespace: true,
			},
			nil,
		)
		tw := utils.NewTestWaiter(1)
		socket := manager.Socket("/", nil)

		socket.Timeout(50*time.Millisecond).Emit("event", func(err error) {
			assert.NotNil(t, err)
			clientSocket := socket.(*clientSocket)
			clientSocket.sendBufferMu.Lock()
			assert.Empty(t, clientSocket.sendBuffer)
			clientSocket.sendBufferMu.Unlock()
			tw.Done()
		})

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		close()
	})

	t.Run("should timeout when the server does not acknowledge the event", func(t *testing.T) {
		_, _, manager, close := newTestServerAndClient(
			t,
			&ServerConfig{
				AcceptAnyNamespace: true,
			},
			nil,
		)
		tw := utils.NewTestWaiter(1)
		socket := manager.Socket("/", nil)

		socket.Connect()
		socket.Timeout(50*time.Millisecond).Emit("event", func(err error) {
			assert.Equal(t, ErrAckTimeout, err)
			tw.Done()
		})

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		close()
	})

	t.Run("should timeout when the server does not acknowledge the event in time", func(t *testing.T) {
		io, _, manager, close := newTestServerAndClient(
			t,
			&ServerConfig{
				AcceptAnyNamespace: true,
			},
			nil,
		)
		tw := utils.NewTestWaiter(1)
		socket := manager.Socket("/", nil)

		io.OnConnection(func(socket ServerSocket) {
			socket.OnEvent("echo", func(n int, r func(int)) {
				r(n)
			})
		})
		socket.Timeout(1*time.Nanosecond).Emit("echo", 42, func(err error, n int) {
			assert.Equal(t, ErrAckTimeout, err)
			tw.Done()
		})
		socket.Connect()

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		close()
	})

	t.Run("should not timeout when the server does acknowledge the event", func(t *testing.T) {
		io, _, manager, close := newTestServerAndClient(
			t,
			&ServerConfig{
				AcceptAnyNamespace: true,
			},
			nil,
		)
		tw := utils.NewTestWaiter(1)
		socket := manager.Socket("/", nil)
		socket.Connect()

		io.OnConnection(func(socket ServerSocket) {
			socket.OnEvent("echo", func(n int, ack func(reply int)) {
				ack(n)
			})
		})
		socket.Timeout(3*time.Second).Emit("echo", 42, func(err error, n int) {
			assert.Nil(t, err)
			assert.Equal(t, 42, n)
			tw.Done()
		})

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		close()
	})

	t.Run("should use the default value", func(t *testing.T) {
		_, _, manager, close := newTestServerAndClient(
			t,
			&ServerConfig{
				AcceptAnyNamespace: true,
			},
			nil,
		)
		tw := utils.NewTestWaiter(1)
		socket := manager.Socket("/", &ClientSocketConfig{
			AckTimeout: 50 * time.Millisecond,
		})

		socket.Connect()
		socket.Emit("event", func(err error) {
			assert.Equal(t, ErrAckTimeout, err)
			tw.Done()
		})

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		close()
	})

	t.Run("should not fire events more than once after manually reconnecting", func(t *testing.T) {
		_, _, manager, close := newTestServerAndClient(
			t,
			&ServerConfig{
				AcceptAnyNamespace: true,
			},
			&ManagerConfig{
				NoReconnection: true,
			})
		socket := manager.Socket("/", nil)
		tw := utils.NewTestWaiter(1)

		socket.OnConnect(func() {
			socket.OffConnect()
			manager.eioMu.Lock()
			go manager.eio.Close()
			manager.eioMu.Unlock()

			manager.OnClose(func(reason Reason, err error) {
				socket.OnConnect(func() {
					tw.Done()
				})
				socket.Connect()
			})
		})
		socket.Connect()

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		close()
	})

	t.Run("should not fire reconnect_failed event more than once when server closed", func(t *testing.T) {
		var (
			reconnectionDelay    = 100 * time.Millisecond
			reconnectionDelayMax = 100 * time.Millisecond
		)
		_, _, manager, close := newTestServerAndClient(
			t,
			&ServerConfig{
				AcceptAnyNamespace: true,
				EIO: eio.ServerConfig{
					PingInterval: 1000 * time.Millisecond,
					PingTimeout:  3000 * time.Millisecond,
				},
			},
			&ManagerConfig{
				ReconnectionAttempts: 3,
				ReconnectionDelay:    &reconnectionDelay,
				ReconnectionDelayMax: &reconnectionDelayMax,
				EIO: eio.ClientConfig{
					Transports: []string{"websocket"},
				},
			})
		socket := manager.Socket("/", nil)
		tw := utils.NewTestWaiter(1)

		socket.OnConnect(func() {
			close()
		})
		manager.OnReconnectFailed(func() {
			tw.Done()
		})
		socket.Connect()

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		time.Sleep(1 * time.Second)
	})
}
