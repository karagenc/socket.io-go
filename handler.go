package sio

import (
	"fmt"
	"reflect"
)

type eventHandler struct {
	rv         reflect.Value
	inputArgs  []reflect.Type
	outputArgs []reflect.Type
}

func newEventHandler(v any) *eventHandler {
	rv := reflect.ValueOf(v)

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
}

func newAckHandler(v any) *ackHandler {
	rv := reflect.ValueOf(v)

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
	}
}

func (f *ackHandler) Call(args ...reflect.Value) (err error) {
	defer func() {
		if r := recover(); r != nil {
			var ok bool
			err, ok = r.(error)
			if !ok {
				err = fmt.Errorf("sio: ack handler error: %v", r)
			}
		}
	}()

	f.rv.Call(args)
	return
}

var errorInterface = reflect.TypeOf((*error)(nil)).Elem()

var serverSocketInterface = reflect.TypeOf((*ServerSocket)(nil)).Elem()
