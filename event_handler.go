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

func newEventHandler(v interface{}) *eventHandler {
	rv := reflect.ValueOf(v)

	if rv.Kind() != reflect.Func {
		panic("function expected")
	}

	rt := rv.Type()

	if rt.NumIn() < 1 {
		panic("handler function must include at least 1 argument")
	}

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

func (f *eventHandler) Call(args []reflect.Value) (ret []reflect.Value, err error) {
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
