package sio

import "net/http/httptest"

func newTestServerAndClient(serverConfig *ServerConfig, managerConfig *ManagerConfig) (server *Server, httpServer *httptest.Server, manager *Manager) {
	if serverConfig == nil {
		serverConfig = new(ServerConfig)
	}
	if managerConfig == nil {
		managerConfig = new(ManagerConfig)
	}
	serverConfig.Debugger = NewPrintDebugger()
	managerConfig.Debugger = NewPrintDebugger()

	server = NewServer(serverConfig)
	httpServer = httptest.NewServer(server)
	manager = NewManager(httpServer.URL, managerConfig)
	return server, httpServer, manager
}
