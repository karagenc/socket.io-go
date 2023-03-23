package sio

import (
	"fmt"
	"net/http/httptest"
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"nhooyr.io/websocket"
)

func TestServerAck(t *testing.T) {
	server, _, manager := newTestServerAndClient(t, nil, nil)
	socket := manager.Socket("/", nil)
	socket.Connect()
	replied := sync.WaitGroup{}
	replied.Add(5)

	manager.OnError(func(err error) {
		t.Fatal(err)
	})

	socket.OnEvent("ack", func(message string, ack func(reply string)) {
		fmt.Printf("event %s\n", message)
		assert.Equal(t, "hello", message)
		ack("hi")
	})

	server.OnConnection(func(socket ServerSocket) {
		for i := 0; i < 5; i++ {
			fmt.Println("Emitting to client")
			socket.Emit("ack", "hello", func(reply string) {
				defer replied.Done()
				fmt.Println("ack")
				assert.Equal(t, "hi", reply)
			})
		}
	})
	replied.Wait()
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
	return server, httpServer, manager
}
