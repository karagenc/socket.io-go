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

var newTestWaiter = eio.NewTestWaiter

func TestServerAck(t *testing.T) {
	server, _, manager := newTestServerAndClient(t, nil, nil)
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
}

func newTestServerAndClient(
	t *testing.T,
	serverConfig *ServerConfig,
	managerConfig *ManagerConfig,
) (
	server *Server,
	httpServer *httptest.Server,
	manager *Manager,
) {
	enablePrintDebugger := os.Getenv("SIO_DEBUGGER_PRINT") == "yes"

	if serverConfig == nil {
		serverConfig = new(ServerConfig)
	}
	if enablePrintDebugger {
		serverConfig.Debugger = NewPrintDebugger()
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
		managerConfig.EIO.Debugger = NewPrintDebugger()
	}
	managerConfig.EIO.WebSocketDialOptions = &websocket.DialOptions{
		CompressionMode: websocket.CompressionDisabled,
	}

	server = NewServer(serverConfig)
	err := server.Run()
	if err != nil {
		t.Fatal(err)
	}

	httpServer = httptest.NewServer(server)
	manager = NewManager(httpServer.URL, managerConfig)

	manager.onNewSocket = func(socket *clientSocket) {
		socket.OnConnectError(func(err error) {
			t.Errorf("client socket connect_error (nsp: %s): %s", socket.namespace, err)
		})
	}
	manager.OnError(func(err error) {
		t.Errorf("Manager error: %s", err)
	})
	server.OnAnyConnection(func(namespace string, socket ServerSocket) {
		socket.OnError(func(err error) {
			t.Errorf("server socket error (sid: %s namespace: %s): %s", socket.ID(), namespace, err)
		})
	})

	return server, httpServer, manager
}
