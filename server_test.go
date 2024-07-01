package sio

import (
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
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

	t.Run("should receive ack", func(t *testing.T) {
		io, _, manager, close := newTestServerAndClient(t, nil, nil)
		clientSocket := manager.Socket("/", nil)
		clientSocket.Connect()
		tw := utils.NewTestWaiter(5)

		clientSocket.OnEvent("ack", func(message string, ack func(reply string)) {
			t.Logf("event %s", message)
			assert.Equal(t, "hello", message)
			ack("hi")
		})

		io.OnConnection(func(socket ServerSocket) {
			for i := 0; i < 5; i++ {
				t.Log("Emitting to client")
				socket.Emit("ack", "hello", func(reply string) {
					defer tw.Done()
					t.Log("ack")
					assert.Equal(t, "hi", reply)
				})
			}
		})
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
