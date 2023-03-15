package sio

func (s *serverSocket) OnEvent(eventName string, handler any) {
	if IsEventReservedForServer(eventName) {
		panic("sio: OnEvent: attempted to register a reserved event: `" + eventName + "`")
	}
	h, err := newEventHandler(handler)
	if err != nil {
		panic(err)
	}
	s.eventHandlers.On(eventName, h)
}

func (s *serverSocket) OnceEvent(eventName string, handler any) {
	if IsEventReservedForServer(eventName) {
		panic("sio: OnceEvent: attempted to register a reserved event: `" + eventName + "`")
	}
	h, err := newEventHandler(handler)
	if err != nil {
		panic(err)
	}
	s.eventHandlers.Once(eventName, h)
}

func (s *serverSocket) OffEvent(eventName string, handler ...any) {
	s.eventHandlers.Off(eventName, handler...)
}

func (s *serverSocket) OffAll() {
	s.eventHandlers.OffAll()
	s.errorHandlers.OffAll()
	s.disconnectingHandlers.OffAll()
	s.disconnectHandlers.OffAll()
}

type (
	ServerSocketDisconnectingFunc func(reason Reason)
	ServerSocketDisconnectFunc    func(reason Reason)
	ServerSocketErrorFunc         func(err error)
)

func (s *serverSocket) OnError(f ServerSocketErrorFunc) {
	s.errorHandlers.On(&f)
}

func (s *serverSocket) OnceError(f ServerSocketErrorFunc) {
	s.errorHandlers.Once(&f)
}

func (s *serverSocket) OffError(_f ...ServerSocketErrorFunc) {
	f := make([]*ServerSocketErrorFunc, len(_f))
	for i := range f {
		f[i] = &_f[i]
	}
	s.errorHandlers.Off(f...)
}

func (s *serverSocket) OnDisconnecting(f ServerSocketDisconnectingFunc) {
	s.disconnectingHandlers.On(&f)
}

func (s *serverSocket) OnceDisconnecting(f ServerSocketDisconnectingFunc) {
	s.disconnectingHandlers.Once(&f)
}

func (s *serverSocket) OffDisconnecting(_f ...ServerSocketDisconnectingFunc) {
	f := make([]*ServerSocketDisconnectingFunc, len(_f))
	for i := range f {
		f[i] = &_f[i]
	}
	s.disconnectingHandlers.Off(f...)
}

func (s *serverSocket) OnDisconnect(f ServerSocketDisconnectFunc) {
	s.disconnectHandlers.On(&f)
}

func (s *serverSocket) OnceDisconnect(f ServerSocketDisconnectFunc) {
	s.disconnectHandlers.Once(&f)
}

func (s *serverSocket) OffDisconnect(_f ...ServerSocketDisconnectFunc) {
	f := make([]*ServerSocketDisconnectFunc, len(_f))
	for i := range f {
		f[i] = &_f[i]
	}
	s.disconnectHandlers.Off(f...)
}
