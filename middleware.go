package sio

import (
	"fmt"
	"reflect"
)

type NspMiddlewareFunc func(socket ServerSocket, handshake *Handshake) error

func (n *Namespace) Use(f NspMiddlewareFunc) {
	n.middlewareFuncsMu.Lock()
	defer n.middlewareFuncsMu.Unlock()
	n.middlewareFuncs = append(n.middlewareFuncs, f)
}

func (n *Namespace) runMiddlewares(socket *serverSocket, handshake *Handshake) error {
	n.middlewareFuncsMu.RLock()
	defer n.middlewareFuncsMu.RUnlock()

	for _, f := range n.middlewareFuncs {
		err := f(socket, handshake)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *serverSocket) Use(f any) {
	s.middlewareFuncsMu.Lock()
	defer s.middlewareFuncsMu.Unlock()
	rv := reflect.ValueOf(f)
	if rv.Kind() != reflect.Func {
		panic("sio: function expected")
	}
	rt := rv.Type()
	if rt.NumIn() != 2 {
		panic("sio: function signature: func(eventName string, v ...any) error")
	}
	if rt.In(0).Kind() != reflect.String {
		panic("sio: function signature: func(eventName string, v ...any) error")
	}
	if rt.In(1).Kind() != reflect.Slice || rt.In(1).Elem().Kind() != reflect.Interface {
		panic("sio: function signature: func(eventName string, v ...any) error")
	}
	if rt.NumOut() != 1 {
		panic("sio: function signature: func(eventName string, v ...any) error")
	}
	if rt.Out(0).Kind() != reflect.Interface || !rt.Out(0).Implements(reflectError) {
		panic("sio: function signature: func(eventName string, v ...any) error")
	}
	s.middlewareFuncs = append(s.middlewareFuncs, rv)
}

func (s *serverSocket) callMiddlewares(values []reflect.Value) error {
	s.middlewareFuncsMu.RLock()
	defer s.middlewareFuncsMu.RUnlock()

	for _, f := range s.middlewareFuncs {
		err := s.callMiddlewareFunc(f, values)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *serverSocket) callMiddlewareFunc(rv reflect.Value, values []reflect.Value) (err error) {
	defer func() {
		if r := recover(); r != nil {
			var ok bool
			err, ok = r.(error)
			if !ok {
				err = fmt.Errorf("sio: middleware error: %v", r)
			}
		}
	}()
	rets := rv.Call(values)
	ret := rets[0]
	if ret.IsNil() {
		return nil
	}
	err = ret.Interface().(error)
	return
}
