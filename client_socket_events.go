package sio

func (s *clientSocket) OnEvent(eventName string, handler any) {
	if IsEventReservedForClient(eventName) {
		panic("sio: OnEvent: attempted to register a reserved event: `" + eventName + "`")
	}
	h, err := newEventHandler(handler)
	if err != nil {
		panic(err)
	}
	s.eventHandlers.On(eventName, h)
}

func (s *clientSocket) OnceEvent(eventName string, handler any) {
	if IsEventReservedForClient(eventName) {
		panic("sio: OnceEvent: attempted to register a reserved event: `" + eventName + "`")
	}
	h, err := newEventHandler(handler)
	if err != nil {
		panic(err)
	}
	s.eventHandlers.Once(eventName, h)
}

func (s *clientSocket) OffEvent(eventName string, handler ...any) {
	s.eventHandlers.Off(eventName, handler...)
}

func (s *clientSocket) OffAll() {
	s.eventHandlers.OffAll()
	s.connectHandlers.OffAll()
	s.connectErrorHandlers.OffAll()
	s.disconnectHandlers.OffAll()
}

type (
	ClientSocketConnectFunc      func()
	ClientSocketConnectErrorFunc func(err error)
	ClientSocketDisconnectFunc   func(reason Reason)
)

func (s *clientSocket) OnConnect(f ClientSocketConnectFunc) {
	s.connectHandlers.On(&f)
}

func (s *clientSocket) OnceConnect(f ClientSocketConnectFunc) {
	s.connectHandlers.Once(&f)
}

func (s *clientSocket) OffConnect(_f ...ClientSocketConnectFunc) {
	f := make([]*ClientSocketConnectFunc, len(_f))
	for i := range f {
		f[i] = &_f[i]
	}
	s.connectHandlers.Off(f...)
}

func (s *clientSocket) OnConnectError(f ClientSocketConnectErrorFunc) {
	s.connectErrorHandlers.On(&f)
}

func (s *clientSocket) OnceConnectError(f ClientSocketConnectErrorFunc) {
	s.connectErrorHandlers.Once(&f)
}

func (s *clientSocket) OffConnectError(_f ...ClientSocketConnectErrorFunc) {
	f := make([]*ClientSocketConnectErrorFunc, len(_f))
	for i := range f {
		f[i] = &_f[i]
	}
	s.connectErrorHandlers.Off(f...)
}

func (s *clientSocket) OnDisconnect(f ClientSocketDisconnectFunc) {
	s.disconnectHandlers.On(&f)
}

func (s *clientSocket) OnceDisconnect(f ClientSocketDisconnectFunc) {
	s.disconnectHandlers.Once(&f)
}

func (s *clientSocket) OffDisconnect(_f ...ClientSocketDisconnectFunc) {
	f := make([]*ClientSocketDisconnectFunc, len(_f))
	for i := range f {
		f[i] = &_f[i]
	}
	s.disconnectHandlers.Off(f...)
}
