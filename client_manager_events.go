package sio

func (m *Manager) OffAll() {
	m.openHandlers.offAll()
	m.errorHandlers.offAll()
	m.closeHandlers.offAll()
	m.reconnectHandlers.offAll()
	m.reconnectAttemptHandlers.offAll()
	m.reconnectErrorHandlers.offAll()
	m.reconnectFailedHandlers.offAll()
}

type (
	ManagerOpenFunc             func()
	ManagerPingFunc             func()
	ManagerErrorFunc            func(err error)
	ManagerCloseFunc            func(reason Reason, err error)
	ManagerReconnectFunc        func(attempt uint32)
	ManagerReconnectAttemptFunc func(attempt uint32)
	ManagerReconnectErrorFunc   func(err error)
	ManagerReconnectFailedFunc  func()
)

func (m *Manager) OnOpen(f ManagerOpenFunc) {
	m.openHandlers.on(&f)
}

func (m *Manager) OnceOpen(f ManagerOpenFunc) {
	m.openHandlers.once(&f)
}

func (m *Manager) OffOpen(_f ...ManagerOpenFunc) {
	f := make([]*ManagerOpenFunc, len(_f))
	for i := range f {
		f[i] = &_f[i]
	}
	m.openHandlers.off(f...)
}

func (m *Manager) OnPing(f ManagerPingFunc) {
	m.pingHandlers.on(&f)
}

func (m *Manager) OncePing(f ManagerPingFunc) {
	m.pingHandlers.once(&f)
}

func (m *Manager) OffPing(_f ...ManagerPingFunc) {
	f := make([]*ManagerPingFunc, len(_f))
	for i := range f {
		f[i] = &_f[i]
	}
	m.pingHandlers.off(f...)
}

func (m *Manager) OnError(f ManagerErrorFunc) {
	m.errorHandlers.on(&f)
}

func (m *Manager) OnceError(f ManagerErrorFunc) {
	m.errorHandlers.once(&f)
}

func (m *Manager) OffError(_f ...ManagerErrorFunc) {
	f := make([]*ManagerErrorFunc, len(_f))
	for i := range f {
		f[i] = &_f[i]
	}
	m.errorHandlers.off(f...)
}

func (m *Manager) OnClose(f ManagerCloseFunc) {
	m.closeHandlers.on(&f)
}

func (m *Manager) OnceClose(f ManagerCloseFunc) {
	m.closeHandlers.once(&f)
}

func (m *Manager) OffClose(_f ...ManagerCloseFunc) {
	f := make([]*ManagerCloseFunc, len(_f))
	for i := range f {
		f[i] = &_f[i]
	}
	m.closeHandlers.off(f...)
}

func (m *Manager) OnReconnect(f ManagerReconnectFunc) {
	m.reconnectHandlers.on(&f)
}

func (m *Manager) OnceReconnect(f ManagerReconnectFunc) {
	m.reconnectHandlers.once(&f)
}

func (m *Manager) OffReconnect(_f ...ManagerReconnectFunc) {
	f := make([]*ManagerReconnectFunc, len(_f))
	for i := range f {
		f[i] = &_f[i]
	}
	m.reconnectHandlers.off(f...)
}

func (m *Manager) OnReconnectAttempt(f ManagerReconnectAttemptFunc) {
	m.reconnectAttemptHandlers.on(&f)
}

func (m *Manager) OnceReconnectAttempt(f ManagerReconnectAttemptFunc) {
	m.reconnectAttemptHandlers.once(&f)
}

func (m *Manager) OffReconnectAttempt(_f ...ManagerReconnectAttemptFunc) {
	f := make([]*ManagerReconnectAttemptFunc, len(_f))
	for i := range f {
		f[i] = &_f[i]
	}
	m.reconnectAttemptHandlers.off(f...)
}

func (m *Manager) OnReconnectError(f ManagerReconnectErrorFunc) {
	m.reconnectErrorHandlers.on(&f)
}

func (m *Manager) OnceReconnectError(f ManagerReconnectErrorFunc) {
	m.reconnectErrorHandlers.once(&f)
}

func (m *Manager) OffReconnectError(_f ...ManagerReconnectErrorFunc) {
	f := make([]*ManagerReconnectErrorFunc, len(_f))
	for i := range f {
		f[i] = &_f[i]
	}
	m.reconnectErrorHandlers.off(f...)
}

func (m *Manager) OnReconnectFailed(f ManagerReconnectFailedFunc) {
	m.reconnectFailedHandlers.on(&f)
}

func (m *Manager) OnceReconnectFailed(f ManagerReconnectFailedFunc) {
	m.reconnectFailedHandlers.once(&f)
}

func (m *Manager) OffReconnectFailed(_f ...ManagerReconnectFailedFunc) {
	f := make([]*ManagerReconnectFailedFunc, len(_f))
	for i := range f {
		f[i] = &_f[i]
	}
	m.reconnectFailedHandlers.off(f...)
}
