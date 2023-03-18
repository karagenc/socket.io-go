package sio

import (
	"net/http/httptest"

	"nhooyr.io/websocket"
)

func newTestServerAndClient(serverConfig *ServerConfig, managerConfig *ManagerConfig) (server *Server, httpServer *httptest.Server, manager *Manager) {
	if serverConfig == nil {
		serverConfig = new(ServerConfig)
	}
	if managerConfig == nil {
		managerConfig = new(ManagerConfig)
	}
	managerConfig.EIO.WebSocketDialOptions = &websocket.DialOptions{
		CompressionMode: websocket.CompressionDisabled,
	}
	serverConfig.Debugger = NewPrintDebugger()
	serverConfig.EIO.WebSocketAcceptOptions = &websocket.AcceptOptions{
		CompressionMode: websocket.CompressionDisabled,
	}
	managerConfig.Debugger = NewPrintDebugger()

	server = NewServer(serverConfig)
	httpServer = httptest.NewServer(server)
	manager = NewManager(httpServer.URL, managerConfig)
	return server, httpServer, manager
}
