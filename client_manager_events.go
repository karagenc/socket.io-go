package sio

func (m *Manager) OffAll() {
	m.openHandlers.OffAll()
	m.errorHandlers.OffAll()
	m.closeHandlers.OffAll()
	m.reconnectHandlers.OffAll()
	m.reconnectAttemptHandlers.OffAll()
	m.reconnectErrorHandlers.OffAll()
	m.reconnectFailedHandlers.OffAll()
}

type (
	ManagerOpenFunc             func()
	ManagerPingFunc             func()
	ManagerErrorFunc            func(err error)
	ManagerCloseFunc            func(reason Reason, err error)
	ManagerReconnectFunc        func(attempt int)
	ManagerReconnectAttemptFunc func(attempt int)
	ManagerReconnectErrorFunc   func(err error)
	ManagerReconnectFailedFunc  func()
)

func (m *Manager) OnOpen(f ManagerOpenFunc) {
	m.openHandlers.On(&f)
}

func (m *Manager) OnceOpen(f ManagerOpenFunc) {
	m.openHandlers.Once(&f)
}

func (m *Manager) OffOpen(_f ...ManagerOpenFunc) {
	f := make([]*ManagerOpenFunc, len(_f))
	for i := range f {
		f[i] = &_f[i]
	}
	m.openHandlers.Off(f...)
}

func (m *Manager) OnError(f ManagerErrorFunc) {
	m.errorHandlers.On(&f)
}

func (m *Manager) OnceError(f ManagerErrorFunc) {
	m.errorHandlers.Once(&f)
}

func (m *Manager) OffError(_f ...ManagerErrorFunc) {
	f := make([]*ManagerErrorFunc, len(_f))
	for i := range f {
		f[i] = &_f[i]
	}
	m.errorHandlers.Off(f...)
}

func (m *Manager) OnClose(f ManagerCloseFunc) {
	m.closeHandlers.On(&f)
}

func (m *Manager) OnceClose(f ManagerCloseFunc) {
	m.closeHandlers.Once(&f)
}

func (m *Manager) OffClose(_f ...ManagerCloseFunc) {
	f := make([]*ManagerCloseFunc, len(_f))
	for i := range f {
		f[i] = &_f[i]
	}
	m.closeHandlers.Off(f...)
}

func (m *Manager) OnReconnect(f ManagerReconnectFunc) {
	m.reconnectHandlers.On(&f)
}

func (m *Manager) OnceReconnect(f ManagerReconnectFunc) {
	m.reconnectHandlers.Once(&f)
}

func (m *Manager) OffReconnect(_f ...ManagerReconnectFunc) {
	f := make([]*ManagerReconnectFunc, len(_f))
	for i := range f {
		f[i] = &_f[i]
	}
	m.reconnectHandlers.Off(f...)
}

func (m *Manager) OnReconnectAttempt(f ManagerReconnectAttemptFunc) {
	m.reconnectAttemptHandlers.On(&f)
}

func (m *Manager) OnceReconnectAttempt(f ManagerReconnectAttemptFunc) {
	m.reconnectAttemptHandlers.Once(&f)
}

func (m *Manager) OffReconnectAttempt(_f ...ManagerReconnectAttemptFunc) {
	f := make([]*ManagerReconnectAttemptFunc, len(_f))
	for i := range f {
		f[i] = &_f[i]
	}
	m.reconnectAttemptHandlers.Off(f...)
}

func (m *Manager) OnReconnectError(f ManagerReconnectErrorFunc) {
	m.reconnectErrorHandlers.On(&f)
}

func (m *Manager) OnceReconnectError(f ManagerReconnectErrorFunc) {
	m.reconnectErrorHandlers.Once(&f)
}

func (m *Manager) OffReconnectError(_f ...ManagerReconnectErrorFunc) {
	f := make([]*ManagerReconnectErrorFunc, len(_f))
	for i := range f {
		f[i] = &_f[i]
	}
	m.reconnectErrorHandlers.Off(f...)
}

func (m *Manager) OnReconnectFailed(f ManagerReconnectFailedFunc) {
	m.reconnectFailedHandlers.On(&f)
}

func (m *Manager) OnceReconnectFailed(f ManagerReconnectFailedFunc) {
	m.reconnectFailedHandlers.Once(&f)
}

func (m *Manager) OffReconnectFailed(_f ...ManagerReconnectFailedFunc) {
	f := make([]*ManagerReconnectFailedFunc, len(_f))
	for i := range f {
		f[i] = &_f[i]
	}
	m.reconnectFailedHandlers.Off(f...)
}
