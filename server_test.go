package sio

import (
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
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
		clientSocket := manager.Socket("/", nil)
		tw := utils.NewTestWaiter(1)

		io.OnConnection(func(serverSocket ServerSocket) {
			serverSocket.OnEvent("random", func(a int, b string, c []int) {
				assert.Equal(t, 1, a)
				assert.Equal(t, "2", b)
				assert.Equal(t, []int{3}, c)
				tw.Done()
			})
		})
		clientSocket.Emit("random", 1, "2", []int{3})
		clientSocket.Connect()

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		close()
	})

	t.Run("should error with null messages", func(t *testing.T) {
		io, _, manager, close := newTestServerAndClient(
			t,
			nil,
			nil,
		)
		clientSocket := manager.Socket("/", nil)
		tw := utils.NewTestWaiter(1)

		io.OnConnection(func(serverSocket ServerSocket) {
			serverSocket.OnEvent("message", func(a any) {
				assert.Equal(t, nil, a)
				tw.Done()
			})
		})
		clientSocket.Emit("message", nil)
		clientSocket.Connect()

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		close()
	})

	t.Run("should emit events", func(t *testing.T) {
		io, _, manager, close := newTestServerAndClient(
			t,
			nil,
			nil,
		)
		clientSocket := manager.Socket("/", nil)
		tw := utils.NewTestWaiter(1)

		clientSocket.OnEvent("woot", func(a string) {
			assert.Equal(t, "tobi", a)
			tw.Done()
		})
		io.OnConnection(func(serverSocket ServerSocket) {
			serverSocket.Emit("woot", "tobi")
		})
		clientSocket.Connect()

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		close()
	})

	t.Run("should emit events with utf8 multibyte character", func(t *testing.T) {
		io, _, manager, close := newTestServerAndClient(
			t,
			nil,
			nil,
		)
		clientSocket := manager.Socket("/", nil)
		tw := utils.NewTestWaiter(3)

		clientSocket.OnEvent("hoot", func(a string) {
			assert.Equal(t, "utf8 — string", a)
			tw.Done()
		})
		io.OnConnection(func(serverSocket ServerSocket) {
			serverSocket.Emit("hoot", "utf8 — string")
			serverSocket.Emit("hoot", "utf8 — string")
			serverSocket.Emit("hoot", "utf8 — string")
		})
		clientSocket.Connect()

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
		clientSocket := manager.Socket("/", nil)
		tw := utils.NewTestWaiter(1)

		clientSocket.OnEvent("randomBin", func(a Binary) {
			assert.Equal(t, randomBin, []byte(a))
			tw.Done()
		})
		io.OnConnection(func(serverSocket ServerSocket) {
			serverSocket.Emit("randomBin", Binary(randomBin))
		})
		clientSocket.Connect()

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
		clientSocket := manager.Socket("/", nil)
		tw := utils.NewTestWaiter(1)

		io.OnConnection(func(serverSocket ServerSocket) {
			serverSocket.OnEvent("randomBin", func(a Binary) {
				assert.Equal(t, randomBin, []byte(a))
				tw.Done()
			})
			clientSocket.Emit("randomBin", Binary(randomBin))
		})
		clientSocket.Connect()

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
		clientSocket := manager.Socket("/", nil)
		tw := utils.NewTestWaiter(1)

		io.OnConnection(func(serverSocket ServerSocket) {
			serverSocket.OnEvent("multiple", func(a int, b string, c []int, d Binary, e []any) {
				assert.Equal(t, 1, a)
				assert.Equal(t, "3", b)
				assert.Equal(t, []int{4}, c)
				assert.Equal(t, randomBin, []byte(d))
				assert.Len(t, e, 2)
				assert.Equal(t, float64(5), e[0])
				assert.Equal(t, "swag", e[1])
				tw.Done()
			})
			clientSocket.Emit("multiple", 1, "3", []int{4}, Binary(randomBin), []any{5, "swag"})
		})
		clientSocket.Connect()

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		close()
	})

	t.Run("should receive all events emitted from namespaced client immediately and in order", func(t *testing.T) {
		io, _, manager, close := newTestServerAndClient(
			t,
			nil,
			nil,
		)
		clientSocket := manager.Socket("/chat", nil)
		tw := utils.NewTestWaiter(2)

		total := 0
		countMu := sync.Mutex{}

		io.Of("/chat").OnConnection(func(serverSocket ServerSocket) {
			serverSocket.OnEvent("hi", func(letter string) {
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
		clientSocket.Connect()
		clientSocket.Emit("hi", "a")
		clientSocket.Emit("hi", "b")

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

	t.Run("should fire a CONNECT event", func(t *testing.T) {
		io, _, manager, close := newTestServerAndClient(t, nil, nil)
		clientSocket := manager.Socket("/", nil)
		tw := utils.NewTestWaiter(1)

		io.OnConnection(func(socket ServerSocket) {
			tw.Done()
		})
		clientSocket.Connect()
		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		close()
	})

	t.Run(`should be able to equivalently start with "" or "/" on server`, func(t *testing.T) {
		io, _, manager, close := newTestServerAndClient(t, nil, nil)
		clientSocket := manager.Socket("/", nil)
		tw := utils.NewTestWaiterString()
		tw.Add("/abc")
		tw.Add("")

		io.Of("/abc").OnConnection(func(socket ServerSocket) {
			tw.Done("/abc")
		})
		io.Of("").OnConnection(func(socket ServerSocket) {
			tw.Done("")
		})

		manager.Socket("/abc", nil).Connect()
		clientSocket.Connect()
		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		close()
	})

	t.Run(`should be equivalent for "" and "/" on client`, func(t *testing.T) {
		io, _, manager, close := newTestServerAndClient(t, nil, nil)
		clientSocket := manager.Socket("", nil)
		tw := utils.NewTestWaiter(1)

		io.Of("/").OnConnection(func(socket ServerSocket) {
			tw.Done()
		})

		clientSocket.Connect()
		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		close()
	})

	t.Run("should work with `of` and many sockets", func(t *testing.T) {
		io, _, manager, close := newTestServerAndClient(t, nil, nil)
		clientSocket := manager.Socket("/", nil)
		tw := utils.NewTestWaiterString()
		tw.Add("/chat")
		tw.Add("/news")
		tw.Add("/")

		io.Of("/chat").OnConnection(func(socket ServerSocket) {
			tw.Done("/chat")
		})
		io.Of("/news").OnConnection(func(socket ServerSocket) {
			tw.Done("/news")
		})
		io.OnConnection(func(socket ServerSocket) {
			tw.Done("/")
		})

		manager.Socket("/chat", nil).Connect()
		manager.Socket("/news", nil).Connect()
		clientSocket.Connect()

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
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
