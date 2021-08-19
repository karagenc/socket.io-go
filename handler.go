package sio

import (
	"fmt"
	"reflect"
)

// Socket callbacks.

type ConnectCallback func()

type ConnectErrorCallback func(err error)

type DisconnectCallback func()

type DisconnectingCallback func()

// Client callbacks.

type OpenCallback func()

// err can be nil. Always do a nil check.
type CloseCallback func(reason string, err error)

type ReconnectCallback func()

type ReconnectAttemptCallback func(attempt int)

type ReconnectErrorCallback func(err error)

type ReconnectFailedCallback func()

// Server callbacks.

type OnSocketCallback func(socket Socket)

type eventHandler struct {
	rv         reflect.Value
	inputArgs  []reflect.Type
	outputArgs []reflect.Type
}

func newEventHandler(v interface{}) *eventHandler {
	rv := reflect.ValueOf(v)

	if rv.Kind() != reflect.Func {
		panic("function expected")
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
				err = fmt.Errorf("handler error: %v", r)
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
}

func newAckHandler(v interface{}) *ackHandler {
	rv := reflect.ValueOf(v)

	if rv.Kind() != reflect.Func {
		panic("function expected")
	}

	rt := rv.Type()

	if rt.NumIn() < 1 {
		panic("ack handler function must include at least 1 argument")
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
	}
}

func (f *ackHandler) Call(args ...reflect.Value) (err error) {
	defer func() {
		if r := recover(); r != nil {
			var ok bool
			err, ok = r.(error)
			if !ok {
				err = fmt.Errorf("ack handler error: %v", r)
			}
		}
	}()

	f.rv.Call(args)
	return
}
