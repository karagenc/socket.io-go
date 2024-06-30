package sio

import (
	"encoding/json"
	"fmt"
	"reflect"
	"time"
)

type NspMiddlewareFunc func(socket ServerSocket, handshake *Handshake) error

type Handshake struct {
	// Date of creation
	Time time.Time

	// Authentication data
	Auth json.RawMessage
}

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
			return middlewareError{err: err}
		}
	}
	return nil
}

func (s *serverSocket) Use(f any) {
	s.middlewareFuncsMu.Lock()
	defer s.middlewareFuncsMu.Unlock()
	rv := reflect.ValueOf(f)
	err := s.checkMiddlewareFunc(rv)
	if err != nil {
		panic(fmt.Errorf("sio: %w", err))
	}
	s.middlewareFuncs = append(s.middlewareFuncs, rv)
}

func (s *serverSocket) checkMiddlewareFunc(rv reflect.Value) error {
	if rv.Kind() != reflect.Func {
		return fmt.Errorf("function expected")
	}
	rt := rv.Type()
	if rt.NumIn() != 2 {
		return fmt.Errorf("function signature: func(eventName string, v ...any) error")
	}
	if rt.In(0).Kind() != reflect.String {
		return fmt.Errorf("function signature: func(eventName string, v ...any) error")
	}
	if rt.In(1).Kind() != reflect.Slice || rt.In(1).Elem().Kind() != reflect.Interface {
		return fmt.Errorf("function signature: func(eventName string, v ...any) error")
	}
	if rt.NumOut() != 1 {
		return fmt.Errorf("function signature: func(eventName string, v ...any) error")
	}
	if rt.Out(0).Kind() != reflect.Interface || !rt.Out(0).Implements(reflectError) {
		return fmt.Errorf("function signature: func(eventName string, v ...any) error")
	}
	return nil
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

type middlewareError struct {
	err error
}

func (e middlewareError) Error() string { return e.err.Error() }
