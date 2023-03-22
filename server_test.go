package sio

import (
	"net/http/httptest"
	"os"
	"testing"

	"nhooyr.io/websocket"
)

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
