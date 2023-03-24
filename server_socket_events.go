package sio

import (
	"fmt"
	"reflect"
)

func (s *serverSocket) OnEvent(eventName string, handler any) {
	if IsEventReservedForServer(eventName) {
		panic(fmt.Errorf("sio: OnEvent: attempted to register a reserved event: `%s`", eventName))
	}
	h, err := newEventHandler(handler)
	if err != nil {
		panic(err)
	}
	s.eventHandlers.on(eventName, h)
}

func (s *serverSocket) OnceEvent(eventName string, handler any) {
	if IsEventReservedForServer(eventName) {
		panic(fmt.Errorf("sio: OnceEvent: attempted to register a reserved event: `%s`", eventName))
	}
	h, err := newEventHandler(handler)
	if err != nil {
		panic(err)
	}
	s.eventHandlers.once(eventName, h)
}

func (s *serverSocket) OffEvent(eventName string, handler ...any) {
	values := make([]reflect.Value, len(handler))
	for i := range values {
		values[i] = reflect.ValueOf(handler[i])
	}
	s.eventHandlers.off(eventName, values...)
}

func (s *serverSocket) OffAll() {
	s.eventHandlers.offAll()
	s.errorHandlers.offAll()
	s.disconnectingHandlers.offAll()
	s.disconnectHandlers.offAll()
}

type (
	ServerSocketDisconnectingFunc func(reason Reason)
	ServerSocketDisconnectFunc    func(reason Reason)
	ServerSocketErrorFunc         func(err error)
)

func (s *serverSocket) OnError(f ServerSocketErrorFunc) {
	s.errorHandlers.on(&f)
}

func (s *serverSocket) OnceError(f ServerSocketErrorFunc) {
	s.errorHandlers.once(&f)
}

func (s *serverSocket) OffError(_f ...ServerSocketErrorFunc) {
	f := make([]*ServerSocketErrorFunc, len(_f))
	for i := range f {
		f[i] = &_f[i]
	}
	s.errorHandlers.off(f...)
}

func (s *serverSocket) OnDisconnecting(f ServerSocketDisconnectingFunc) {
	s.disconnectingHandlers.on(&f)
}

func (s *serverSocket) OnceDisconnecting(f ServerSocketDisconnectingFunc) {
	s.disconnectingHandlers.once(&f)
}

func (s *serverSocket) OffDisconnecting(_f ...ServerSocketDisconnectingFunc) {
	f := make([]*ServerSocketDisconnectingFunc, len(_f))
	for i := range f {
		f[i] = &_f[i]
	}
	s.disconnectingHandlers.off(f...)
}

func (s *serverSocket) OnDisconnect(f ServerSocketDisconnectFunc) {
	s.disconnectHandlers.on(&f)
}

func (s *serverSocket) OnceDisconnect(f ServerSocketDisconnectFunc) {
	s.disconnectHandlers.once(&f)
}

func (s *serverSocket) OffDisconnect(_f ...ServerSocketDisconnectFunc) {
	f := make([]*ServerSocketDisconnectFunc, len(_f))
	for i := range f {
		f[i] = &_f[i]
	}
	s.disconnectHandlers.off(f...)
}
