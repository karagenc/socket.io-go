package sio

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	eio "github.com/tomruk/socket.io-go/engine.io"
	"github.com/tomruk/socket.io-go/internal/sync"
	"github.com/tomruk/socket.io-go/internal/utils"
	"nhooyr.io/websocket"
)

func TestServer(t *testing.T) {
	t.Run("should receive events", func(t *testing.T) {
		io, _, manager, close := newTestServerAndClient(
			t,
			nil,
			nil,
		)
		socket := manager.Socket("/", nil)
		tw := utils.NewTestWaiter(1)

		io.OnConnection(func(socket ServerSocket) {
			socket.OnEvent("random", func(a int, b string, c []int) {
				assert.Equal(t, 1, a)
				assert.Equal(t, "2", b)
				assert.Equal(t, []int{3}, c)
				tw.Done()
			})
		})
		socket.Emit("random", 1, "2", []int{3})
		socket.Connect()

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		close()
	})

	t.Run("should error with null messages", func(t *testing.T) {
		io, _, manager, close := newTestServerAndClient(
			t,
			nil,
			nil,
		)
		socket := manager.Socket("/", nil)
		tw := utils.NewTestWaiter(1)

		io.OnConnection(func(socket ServerSocket) {
			socket.OnEvent("message", func(a any) {
				assert.Equal(t, nil, a)
				tw.Done()
			})
		})
		socket.Emit("message", nil)
		socket.Connect()

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		close()
	})

	t.Run("should emit events", func(t *testing.T) {
		io, _, manager, close := newTestServerAndClient(
			t,
			nil,
			nil,
		)
		socket := manager.Socket("/", nil)
		tw := utils.NewTestWaiter(1)

		socket.OnEvent("woot", func(a string) {
			assert.Equal(t, "tobi", a)
			tw.Done()
		})
		io.OnConnection(func(socket ServerSocket) {
			socket.Emit("woot", "tobi")
		})
		socket.Connect()

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		close()
	})

	t.Run("should emit events with utf8 multibyte character", func(t *testing.T) {
		io, _, manager, close := newTestServerAndClient(
			t,
			nil,
			nil,
		)
		socket := manager.Socket("/", nil)
		tw := utils.NewTestWaiter(3)

		socket.OnEvent("hoot", func(a string) {
			assert.Equal(t, "utf8 — string", a)
			tw.Done()
		})
		io.OnConnection(func(socket ServerSocket) {
			socket.Emit("hoot", "utf8 — string")
			socket.Emit("hoot", "utf8 — string")
			socket.Emit("hoot", "utf8 — string")
		})
		socket.Connect()

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		close()
	})

	t.Run("should emit events with binary data", func(t *testing.T) {
		randomBin := []byte("\x36\x43\x78\x6a\x4c\xad\x7b\x6f\x33\x96\xc6\xdb\x4b\xd3\xe4\x8c\xc7\x12")

		io, _, manager, close := newTestServerAndClient(
			t,
			nil,
			nil,
		)
		socket := manager.Socket("/", nil)
		tw := utils.NewTestWaiter(1)

		socket.OnEvent("randomBin", func(a Binary) {
			assert.Equal(t, randomBin, []byte(a))
			tw.Done()
		})
		io.OnConnection(func(socket ServerSocket) {
			socket.Emit("randomBin", Binary(randomBin))
		})
		socket.Connect()

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		close()
	})

	t.Run("should receive events with binary data", func(t *testing.T) {
		randomBin := []byte("\x36\x43\x78\x6a\x4c\xad\x7b\x6f\x33\x96\xc6\xdb\x4b\xd3\xe4\x8c\xc7\x12")

		io, _, manager, close := newTestServerAndClient(
			t,
			nil,
			nil,
		)
		socket := manager.Socket("/", nil)
		tw := utils.NewTestWaiter(1)

		io.OnConnection(func(socket ServerSocket) {
			socket.OnEvent("randomBin", func(a Binary) {
				assert.Equal(t, randomBin, []byte(a))
				tw.Done()
			})
		})
		socket.Connect()
		socket.Emit("randomBin", Binary(randomBin))

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		close()
	})

	t.Run("should receive events with several types of data (including binary)", func(t *testing.T) {
		randomBin := []byte("\x36\x43\x78\x6a\x4c\xad\x7b\x6f\x33\x96\xc6\xdb\x4b\xd3\xe4\x8c\xc7\x12")

		io, _, manager, close := newTestServerAndClient(
			t,
			nil,
			nil,
		)
		socket := manager.Socket("/", nil)
		tw := utils.NewTestWaiter(1)

		io.OnConnection(func(socket ServerSocket) {
			socket.OnEvent("multiple", func(a int, b string, c []int, d Binary, e []any) {
				assert.Equal(t, 1, a)
				assert.Equal(t, "3", b)
				assert.Equal(t, []int{4}, c)
				assert.Equal(t, randomBin, []byte(d))
				assert.Len(t, e, 2)
				assert.Equal(t, float64(5), e[0])
				assert.Equal(t, "swag", e[1])
				tw.Done()
			})
		})
		socket.Connect()
		socket.Emit("multiple", 1, "3", []int{4}, Binary(randomBin), []any{5, "swag"})

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		close()
	})

	t.Run("should receive all events emitted from namespaced client immediately and in order", func(t *testing.T) {
		io, _, manager, close := newTestServerAndClient(
			t,
			nil,
			nil,
		)
		socket := manager.Socket("/chat", nil)
		tw := utils.NewTestWaiter(2)

		total := 0
		countMu := sync.Mutex{}

		io.Of("/chat").OnConnection(func(socket ServerSocket) {
			socket.OnEvent("hi", func(letter string) {
				countMu.Lock()
				defer countMu.Unlock()
				total++
				switch total {
				case 1:
					assert.Equal(t, 'a', int32(letter[0]))
				case 2:
					assert.Equal(t, 'b', int32(letter[0]))
				}
				tw.Done()
			})
		})
		socket.Connect()
		socket.Emit("hi", "a")
		socket.Emit("hi", "b")

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		close()
	})

	t.Run("should receive event with callbacks", func(t *testing.T) {
		io, _, manager, close := newTestServerAndClient(
			t,
			nil,
			nil,
		)
		socket := manager.Socket("/", nil)
		tw := utils.NewTestWaiter(1)

		io.OnConnection(func(socket ServerSocket) {
			socket.OnEvent("woot", func(fn func(int, int)) {
				fn(1, 2)
			})
		})
		socket.Connect()
		socket.Emit("woot", func(a, b int) {
			assert.Equal(t, 1, a)
			assert.Equal(t, 2, b)
			tw.Done()
		})

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		close()
	})

	t.Run("should emit events with callbacks", func(t *testing.T) {
		io, _, manager, close := newTestServerAndClient(
			t,
			nil,
			nil,
		)
		socket := manager.Socket("/", nil)
		tw := utils.NewTestWaiter(1)

		io.OnConnection(func(socket ServerSocket) {
			socket.OnEvent("woot", func(fn func(int, int)) {
				fn(1, 2)
			})
		})
		socket.Connect()
		socket.Emit("woot", func(a, b int) {
			assert.Equal(t, 1, a)
			assert.Equal(t, 2, b)
			tw.Done()
		})

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		close()
	})

	t.Run("should receive events with args and callback", func(t *testing.T) {
		io, _, manager, close := newTestServerAndClient(
			t,
			nil,
			nil,
		)
		socket := manager.Socket("/", nil)
		tw := utils.NewTestWaiter(1)

		io.OnConnection(func(socket ServerSocket) {
			socket.OnEvent("woot", func(a, b int, c func()) {
				assert.Equal(t, 1, a)
				assert.Equal(t, 2, b)
				c()
			})
		})
		socket.Connect()
		socket.Emit("woot", 1, 2, func() {
			tw.Done()
		})

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		close()
	})

	t.Run("should emit events with args and callback", func(t *testing.T) {
		io, _, manager, close := newTestServerAndClient(
			t,
			nil,
			nil,
		)
		socket := manager.Socket("/", nil)
		tw := utils.NewTestWaiter(2)

		io.OnConnection(func(socket ServerSocket) {
			socket.Emit("woot", 1, 2, func() {
				tw.Done()
			})
		})
		socket.OnEvent("woot", func(a, b int, c func()) {
			assert.Equal(t, 1, a)
			assert.Equal(t, 2, b)
			c()
			tw.Done()
		})
		socket.Connect()

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		close()
	})

	t.Run("should receive events with binary args and callbacks", func(t *testing.T) {
		randomBin := []byte("\x36\x43\x78\x6a\x4c\xad\x7b\x6f\x33\x96\xc6\xdb\x4b\xd3\xe4\x8c\xc7\x12")

		io, _, manager, close := newTestServerAndClient(
			t,
			nil,
			nil,
		)
		socket := manager.Socket("/", nil)
		tw := utils.NewTestWaiter(1)

		io.OnConnection(func(socket ServerSocket) {
			socket.OnEvent("woot", func(a Binary, b func(int, int)) {
				assert.Equal(t, randomBin, []byte(a))
				b(1, 2)
			})
		})
		socket.Connect()
		socket.Emit("woot", Binary(randomBin), func(a, b int) {
			assert.Equal(t, 1, a)
			assert.Equal(t, 2, b)
			tw.Done()
		})

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		close()
	})

	t.Run("should emit events with binary args and callback", func(t *testing.T) {
		randomBin := []byte("\x36\x43\x78\x6a\x4c\xad\x7b\x6f\x33\x96\xc6\xdb\x4b\xd3\xe4\x8c\xc7\x12")

		io, _, manager, close := newTestServerAndClient(
			t,
			nil,
			nil,
		)
		socket := manager.Socket("/", nil)
		tw := utils.NewTestWaiter(1)

		io.OnConnection(func(socket ServerSocket) {
			socket.Emit("woot", Binary(randomBin), func(a, b int) {
				assert.Equal(t, 1, a)
				assert.Equal(t, 2, b)
				tw.Done()
			})
		})
		socket.OnEvent("woot", func(a Binary, b func(int, int)) {
			assert.Equal(t, randomBin, []byte(a))
			b(1, 2)
		})
		socket.Connect()

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		close()
	})

	t.Run("should emit events with binary args and callback", func(t *testing.T) {
		randomBin := []byte("\x36\x43\x78\x6a\x4c\xad\x7b\x6f\x33\x96\xc6\xdb\x4b\xd3\xe4\x8c\xc7\x12")

		io, _, manager, close := newTestServerAndClient(
			t,
			nil,
			nil,
		)
		socket := manager.Socket("/", nil)
		tw := utils.NewTestWaiter(1)

		io.OnConnection(func(socket ServerSocket) {
			socket.Emit("woot", Binary(randomBin), func(a, b int) {
				assert.Equal(t, 1, a)
				assert.Equal(t, 2, b)
				tw.Done()
			})
		})
		socket.OnEvent("woot", func(a Binary, b func(int, int)) {
			assert.Equal(t, randomBin, []byte(a))
			b(1, 2)
		})
		socket.Connect()

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		close()
	})

	t.Run("should emit events and receive binary data in a callback", func(t *testing.T) {
		randomBin := []byte("\x36\x43\x78\x6a\x4c\xad\x7b\x6f\x33\x96\xc6\xdb\x4b\xd3\xe4\x8c\xc7\x12")

		io, _, manager, close := newTestServerAndClient(
			t,
			nil,
			nil,
		)
		socket := manager.Socket("/", nil)
		tw := utils.NewTestWaiter(1)

		io.OnConnection(func(socket ServerSocket) {
			socket.Emit("hi", func(a Binary) {
				assert.Equal(t, Binary(randomBin), a)
				tw.Done()
			})
		})
		socket.OnEvent("hi", func(ack func(Binary)) {
			ack(randomBin)
		})
		socket.Connect()

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		close()
	})

	t.Run("should receive events and pass binary data in a callback", func(t *testing.T) {
		randomBin := []byte("\x36\x43\x78\x6a\x4c\xad\x7b\x6f\x33\x96\xc6\xdb\x4b\xd3\xe4\x8c\xc7\x12")

		io, _, manager, close := newTestServerAndClient(
			t,
			nil,
			nil,
		)
		socket := manager.Socket("/", nil)
		tw := utils.NewTestWaiter(1)

		io.OnConnection(func(socket ServerSocket) {
			socket.OnEvent("woot", func(ack func(Binary)) {
				ack(randomBin)
			})
		})
		socket.Emit("woot", func(a Binary) {
			assert.Equal(t, Binary(randomBin), a)
			tw.Done()
		})
		socket.Connect()

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		close()
	})

	t.Run("should be able to emit after server close and restart", func(t *testing.T) {
		var (
			reconnectionDelay    = 100 * time.Millisecond
			reconnectionDelayMax = 100 * time.Millisecond
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
		socket := manager.Socket("/", nil)

		io.OnConnection(func(socket ServerSocket) {
			socket.OnEvent("ev", func(data string) {
				assert.Equal(t, "payload", data)
				tw.Done()
			})
		})

		socket.OnceConnect(func() {
			manager.OnReconnect(func(attempt uint32) {
				socket.Emit("ev", "payload")
			})

			go func() {
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
		})
		socket.Connect()

		tw.WaitTimeout(t, 20*time.Second)
		close()
	})

	t.Run("should leave all rooms joined after a middleware failure", func(t *testing.T) {
		io, _, manager, close := newTestServerAndClient(
			t,
			nil,
			nil,
		)
		tw := utils.NewTestWaiter(1)
		socket := manager.Socket("/", nil)

		io.Use(func(socket ServerSocket, handshake *Handshake) any {
			socket.Join("room1")
			return fmt.Errorf("nope")
		})
		socket.OnConnectError(func(err any) {
			_, ok := io.Of("/").Adapter().SocketRooms(socket.ID())
			assert.False(t, ok)
			tw.Done()
		})
		socket.Connect()

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		close()
	})

	t.Run("should leave all rooms joined after a middleware failure", func(t *testing.T) {
		io, _, manager, close := newTestServerAndClient(
			t,
			nil,
			nil,
		)
		tw := utils.NewTestWaiter(1)
		socket := manager.Socket("/", nil)

		io.OnConnection(func(socket ServerSocket) {
			socket.Disconnect(true)
			socket.Join("room1")
		})
		socket.OnDisconnect(func(reason Reason) {
			_, ok := io.Of("/").Adapter().SocketRooms(socket.ID())
			assert.False(t, ok)
			tw.Done()
		})
		socket.Connect()

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		close()
	})

	t.Run("should timeout if the client does not acknowledge the event", func(t *testing.T) {
		io, _, manager, close := newTestServerAndClient(
			t,
			nil,
			nil,
		)
		tw := utils.NewTestWaiter(1)
		socket := manager.Socket("/", nil)

		io.OnConnection(func(socket ServerSocket) {
			socket.Timeout(100*time.Millisecond).Emit("unknown", func(err error) {
				assert.NotNil(t, err)
				tw.Done()
			})
		})
		socket.Connect()

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		close()
	})

	t.Run("should timeout if the client does not acknowledge the event in time", func(t *testing.T) {
		io, _, manager, close := newTestServerAndClient(
			t,
			nil,
			nil,
		)
		tw := utils.NewTestWaiter(1)
		socket := manager.Socket("/", nil)

		io.OnConnection(func(socket ServerSocket) {
			socket.Timeout(1*time.Nanosecond).Emit("echo", 42, func(err error, n int) {
				assert.NotNil(t, err)
				tw.Done()
			})
		})
		socket.OnEvent("echo", func(n int, ack func(int)) {
			ack(n)
		})
		socket.Connect()

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		time.Sleep(200 * time.Millisecond)
		close()
	})
}

func newTestServerAndClient(
	t *testing.T,
	serverConfig *ServerConfig,
	managerConfig *ManagerConfig,
) (
	io *Server,
	ts *httptest.Server,
	manager *Manager,
	close func(),
) {
	enablePrintDebugger := os.Getenv("SIO_DEBUGGER_PRINT") == "1"
	enablePrintDebuggerEIO := os.Getenv("EIO_DEBUGGER_PRINT") == "1"

	if serverConfig == nil {
		serverConfig = new(ServerConfig)
	}
	if enablePrintDebugger {
		serverConfig.Debugger = NewPrintDebugger()
	}
	if enablePrintDebuggerEIO {
		serverConfig.EIO.Debugger = NewPrintDebugger()
	}
	serverConfig.EIO.WebSocketAcceptOptions = &websocket.AcceptOptions{
		CompressionMode: websocket.CompressionDisabled,
	}

	io = NewServer(serverConfig)
	err := io.Run()
	if err != nil {
		t.Fatal(err)
	}

	ts = httptest.NewServer(io)
	manager = NewManager(ts.URL, managerConfig)

	return io, ts, manager, func() {
		err = io.Close()
		if err != nil {
			t.Fatalf("io.Close: %s", err)
		}
		ts.Close()
	}
}

func newTestManager(ts *httptest.Server, managerConfig *ManagerConfig) *Manager {
	enablePrintDebugger := os.Getenv("SIO_DEBUGGER_PRINT") == "1"
	enablePrintDebuggerEIO := os.Getenv("EIO_DEBUGGER_PRINT") == "1"
	if managerConfig == nil {
		managerConfig = new(ManagerConfig)
	}
	if enablePrintDebugger {
		managerConfig.Debugger = NewPrintDebugger()
	}
	if enablePrintDebuggerEIO {
		managerConfig.EIO.Debugger = NewPrintDebugger()
	}
	managerConfig.EIO.WebSocketDialOptions = &websocket.DialOptions{
		CompressionMode: websocket.CompressionDisabled,
	}
	return NewManager(ts.URL, managerConfig)
}
