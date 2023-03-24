package sio

import "reflect"

func (s *clientSocket) OnEvent(eventName string, handler any) {
	if IsEventReservedForClient(eventName) {
		panic("sio: OnEvent: attempted to register a reserved event: `" + eventName + "`")
	}
	h, err := newEventHandler(handler)
	if err != nil {
		panic(err)
	}
	s.eventHandlers.on(eventName, h)
}

func (s *clientSocket) OnceEvent(eventName string, handler any) {
	if IsEventReservedForClient(eventName) {
		panic("sio: OnceEvent: attempted to register a reserved event: `" + eventName + "`")
	}
	h, err := newEventHandler(handler)
	if err != nil {
		panic(err)
	}
	s.eventHandlers.once(eventName, h)
}

func (s *clientSocket) OffEvent(eventName string, handler ...any) {
	values := make([]reflect.Value, len(handler))
	for i := range values {
		values[i] = reflect.ValueOf(handler[i])
	}
	s.eventHandlers.off(eventName, values...)
}

func (s *clientSocket) OffAll() {
	s.eventHandlers.offAll()
	s.connectHandlers.offAll()
	s.connectErrorHandlers.offAll()
	s.disconnectHandlers.offAll()
}

type (
	ClientSocketConnectFunc      func()
	ClientSocketConnectErrorFunc func(err error)
	ClientSocketDisconnectFunc   func(reason Reason)
)

func (s *clientSocket) OnConnect(f ClientSocketConnectFunc) {
	s.connectHandlers.on(&f)
}

func (s *clientSocket) OnceConnect(f ClientSocketConnectFunc) {
	s.connectHandlers.once(&f)
}

func (s *clientSocket) OffConnect(_f ...ClientSocketConnectFunc) {
	f := make([]*ClientSocketConnectFunc, len(_f))
	for i := range f {
		f[i] = &_f[i]
	}
	s.connectHandlers.off(f...)
}

func (s *clientSocket) OnConnectError(f ClientSocketConnectErrorFunc) {
	s.connectErrorHandlers.on(&f)
}

func (s *clientSocket) OnceConnectError(f ClientSocketConnectErrorFunc) {
	s.connectErrorHandlers.once(&f)
}

func (s *clientSocket) OffConnectError(_f ...ClientSocketConnectErrorFunc) {
	f := make([]*ClientSocketConnectErrorFunc, len(_f))
	for i := range f {
		f[i] = &_f[i]
	}
	s.connectErrorHandlers.off(f...)
}

func (s *clientSocket) OnDisconnect(f ClientSocketDisconnectFunc) {
	s.disconnectHandlers.on(&f)
}

func (s *clientSocket) OnceDisconnect(f ClientSocketDisconnectFunc) {
	s.disconnectHandlers.once(&f)
}

func (s *clientSocket) OffDisconnect(_f ...ClientSocketDisconnectFunc) {
	f := make([]*ClientSocketDisconnectFunc, len(_f))
	for i := range f {
		f[i] = &_f[i]
	}
	s.disconnectHandlers.off(f...)
}
