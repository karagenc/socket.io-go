package sio

import (
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	eio "github.com/tomruk/socket.io-go/engine.io"
	"nhooyr.io/websocket"
)

const defaultTestWaitTimeout = eio.DefaultTestWaitTimeout

var (
	newTestWaiter       = eio.NewTestWaiter
	newTestWaiterString = eio.NewTestWaiterString
)

func TestServer(t *testing.T) {
	t.Run("should fire a CONNECT event", func(t *testing.T) {
		server, _, manager, close := newTestServerAndClient(t, nil, nil)
		socket := manager.Socket("/", nil)
		tw := newTestWaiter(1)

		server.OnConnection(func(socket ServerSocket) {
			tw.Done()
		})
		socket.Connect()
		tw.WaitTimeout(t, defaultTestWaitTimeout)
		close()
	})

	t.Run(`should be able to equivalently start with "" or "/" on server`, func(t *testing.T) {
		server, _, manager, close := newTestServerAndClient(t, nil, nil)
		socket := manager.Socket("/", nil)
		tw := newTestWaiterString()
		tw.Add("/abc")
		tw.Add("")

		server.Of("/abc").OnConnection(func(socket ServerSocket) {
			tw.Done("/abc")
		})
		server.Of("").OnConnection(func(socket ServerSocket) {
			tw.Done("")
		})

		manager.Socket("/abc", nil).Connect()
		socket.Connect()
		tw.WaitTimeout(t, defaultTestWaitTimeout)
		close()
	})

	t.Run(`should be equivalent for "" and "/" on client`, func(t *testing.T) {
		server, _, manager, close := newTestServerAndClient(t, nil, nil)
		socket := manager.Socket("", nil)
		tw := newTestWaiter(1)

		server.Of("/").OnConnection(func(socket ServerSocket) {
			tw.Done()
		})

		socket.Connect()
		tw.WaitTimeout(t, defaultTestWaitTimeout)
		close()
	})

	t.Run("should work with `of` and many sockets", func(t *testing.T) {
		server, _, manager, close := newTestServerAndClient(t, nil, nil)
		socket := manager.Socket("/", nil)
		tw := newTestWaiterString()
		tw.Add("/chat")
		tw.Add("/news")
		tw.Add("/")

		server.Of("/chat").OnConnection(func(socket ServerSocket) {
			tw.Done("/chat")
		})
		server.Of("/news").OnConnection(func(socket ServerSocket) {
			tw.Done("/news")
		})
		server.OnConnection(func(socket ServerSocket) {
			tw.Done("/")
		})

		manager.Socket("/chat", nil).Connect()
		manager.Socket("/news", nil).Connect()
		socket.Connect()

		tw.WaitTimeout(t, defaultTestWaitTimeout)
		close()
	})

	t.Run("should receive ack", func(t *testing.T) {
		server, _, manager, close := newTestServerAndClient(t, nil, nil)
		socket := manager.Socket("/", nil)
		socket.Connect()
		tw := newTestWaiter(5)

		socket.OnEvent("ack", func(message string, ack func(reply string)) {
			t.Logf("event %s", message)
			assert.Equal(t, "hello", message)
			ack("hi")
		})

		server.OnConnection(func(socket ServerSocket) {
			for i := 0; i < 5; i++ {
				t.Log("Emitting to client")
				socket.Emit("ack", "hello", func(reply string) {
					defer tw.Done()
					t.Log("ack")
					assert.Equal(t, "hi", reply)
				})
			}
		})
		tw.WaitTimeout(t, defaultTestWaitTimeout)
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
