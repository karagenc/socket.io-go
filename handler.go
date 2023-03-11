package sio

import (
	"fmt"
	"reflect"
	"sync"
	"time"
)

type eventHandler struct {
	rv         reflect.Value
	inputArgs  []reflect.Type
	outputArgs []reflect.Type
}

func newEventHandler(f any) *eventHandler {
	rv := reflect.ValueOf(f)

	if rv.Kind() != reflect.Func {
		panic("sio: function expected")
	}

	rt := rv.Type()

	inputArgs := make([]reflect.Type, rt.NumIn())
	for i := range inputArgs {
		inputArgs[i] = rt.In(i)
	}

	outputArgs := make([]reflect.Type, rt.NumOut())
	for i := range outputArgs {
		outputArgs[i] = rt.Out(i)
	}

	return &eventHandler{
		rv:         rv,
		inputArgs:  inputArgs,
		outputArgs: outputArgs,
	}
}

func (f *eventHandler) Call(args ...reflect.Value) (ret []reflect.Value, err error) {
	defer func() {
		if r := recover(); r != nil {
			var ok bool
			err, ok = r.(error)
			if !ok {
				err = fmt.Errorf("sio: handler error: %v", r)
			}
		}
	}()

	ret = f.rv.Call(args)
	return
}

type ackHandler struct {
	rv         reflect.Value
	inputArgs  []reflect.Type
	outputArgs []reflect.Type

	hasError bool

	called   bool
	timedOut bool
	mu       sync.Mutex
}

func newAckHandler(f any, hasError bool) *ackHandler {
	rv := reflect.ValueOf(f)

	if rv.Kind() != reflect.Func {
		panic("sio: function expected")
	}

	rt := rv.Type()

	if rt.NumIn() < 1 {
		panic("sio: ack handler function must include at least 1 argument")
	}

	inputArgs := make([]reflect.Type, rt.NumIn())
	for i := range inputArgs {
		inputArgs[i] = rt.In(i)
	}

	outputArgs := make([]reflect.Type, rt.NumOut())
	for i := range outputArgs {
		outputArgs[i] = rt.Out(i)
	}

	return &ackHandler{
		rv:         rv,
		inputArgs:  inputArgs,
		outputArgs: outputArgs,
		hasError:   hasError,
	}
}

func newAckHandlerWithTimeout(f any, timeout time.Duration, timeoutFunc func()) *ackHandler {
	h := newAckHandler(f, true)
	go func() {
		time.Sleep(timeout)
		h.mu.Lock()
		if h.called {
			h.mu.Unlock()
			return
		}
		h.timedOut = true
		h.mu.Unlock()

		defer func() {
			recover()
		}()

		timeoutFunc()

		args := make([]reflect.Value, len(h.inputArgs))
		args[0] = reflect.ValueOf(fmt.Errorf("operation has timed out"))
		for i := 1; i < len(args); i++ {
			args[i] = reflect.New(h.inputArgs[i]).Elem()
		}
		h.rv.Call(args)
	}()
	return h
}

func (f *ackHandler) Call(args ...reflect.Value) (err error) {
	f.mu.Lock()
	if f.timedOut {
		f.mu.Unlock()
		return nil
	}
	f.called = true
	f.mu.Unlock()

	defer func() {
		if r := recover(); r != nil {
			var ok bool
			err, ok = r.(error)
			if !ok {
				err = fmt.Errorf("sio: ack handler error: %v", r)
			}
		}
	}()

	if f.hasError {
		var e error = nil
		args = append([]reflect.Value{reflect.ValueOf(e)}, args...)
	}
	f.rv.Call(args)
	return
}

func (f *ackHandler) CallWithError(e error, args ...reflect.Value) (err error) {
	if !f.hasError {
		panic("sio: hasError is false. this shouldn't have happened")
	}

	f.mu.Lock()
	if f.timedOut {
		f.mu.Unlock()
		return nil
	}
	f.called = true
	f.mu.Unlock()

	defer func() {
		if r := recover(); r != nil {
			var ok bool
			err, ok = r.(error)
			if !ok {
				err = fmt.Errorf("sio: ack handler error: %v", r)
			}
		}
	}()

	args = append([]reflect.Value{reflect.ValueOf(e)}, args...)
	f.rv.Call(args)
	return
}

func doesAckHandlerHasAnError(f any) error {
	rt := reflect.TypeOf(f)

	if rt.Kind() != reflect.Func {
		return fmt.Errorf("sio: function expected")
	}

	if rt.NumIn() == 0 {
		return fmt.Errorf("sio: ack handler must have error as its 1st parameter")
	}
	if rt.In(0).Kind() != reflect.Interface || !rt.In(0).Implements(reflectError) {
		return fmt.Errorf("sio: ack handler must have error as its 1st parameter")
	}
	return nil
}
