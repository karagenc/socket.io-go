package sio

import (
	"fmt"
	"reflect"
)

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

var errorInterface = reflect.TypeOf((*error)(nil)).Elem()

func checkHandler(eventName string, handler interface{}) {
	switch eventName {
	case "":
		panic("event name cannot be empty")
	case "connect":
		rv := reflect.ValueOf(handler)

		if rv.Kind() != reflect.Func {
			panic("function expected")
		}

		rt := rv.Type()
		if rt.NumIn() != 0 || rt.NumOut() != 0 {
			panic("invalid function signature. must be: func()")
		}
	case "connect_error":
		rv := reflect.ValueOf(handler)

		if rv.Kind() != reflect.Func {
			panic("function expected")
		}

		rt := rv.Type()
		if rt.NumIn() != 1 || rt.NumOut() != 0 {
			panic("invalid function signature. must be: func(err error)")
		}

		e := rt.In(0)
		if !e.Implements(errorInterface) {
			panic("invalid function signature. must be: func(err error)")
		}
	case "disconnect":
		rv := reflect.ValueOf(handler)

		if rv.Kind() != reflect.Func {
			panic("function expected")
		}

		rt := rv.Type()
		if rt.NumIn() != 0 || rt.NumOut() != 0 {
			panic("invalid function signature. must be: func()")
		}
	case "open":
		rv := reflect.ValueOf(handler)

		if rv.Kind() != reflect.Func {
			panic("function expected")
		}

		rt := rv.Type()
		if rt.NumIn() != 0 || rt.NumOut() != 0 {
			panic("invalid function signature. must be: func()")
		}
	case "close":
		rv := reflect.ValueOf(handler)

		if rv.Kind() != reflect.Func {
			panic("function expected")
		}

		rt := rv.Type()
		if rt.NumIn() != 1 || rt.NumOut() != 0 {
			panic("invalid function signature. must be: func(reason string)")
		}

		e := rt.In(0)
		if e.Kind() != reflect.String {
			panic("invalid function signature. must be: func(reason string)")
		}
	case "error":
		rv := reflect.ValueOf(handler)

		if rv.Kind() != reflect.Func {
			panic("function expected")
		}

		rt := rv.Type()
		if rt.NumIn() != 1 || rt.NumOut() != 0 {
			panic("invalid function signature. must be: func(err error)")
		}

		e := rt.In(0)
		if !e.Implements(errorInterface) {
			panic("invalid function signature. must be: func(err error)")
		}
	case "reconnect":
		rv := reflect.ValueOf(handler)

		if rv.Kind() != reflect.Func {
			panic("function expected")
		}

		rt := rv.Type()
		if rt.NumIn() != 1 || rt.NumOut() != 0 {
			panic("invalid function signature. must be: func(attempt int)")
		}

		e := rt.In(0)
		if e.Kind() != reflect.Int32 {
			panic("invalid function signature. must be: func(attempt int)")
		}
	case "reconnect_attempt":
		rv := reflect.ValueOf(handler)

		if rv.Kind() != reflect.Func {
			panic("function expected")
		}

		rt := rv.Type()
		if rt.NumIn() != 1 || rt.NumOut() != 0 {
			panic("invalid function signature. must be: func(attempt int)")
		}

		e := rt.In(0)
		if e.Kind() != reflect.Int32 {
			panic("invalid function signature. must be: func(attempt int)")
		}
	case "reconnect_error":
		rv := reflect.ValueOf(handler)

		if rv.Kind() != reflect.Func {
			panic("function expected")
		}

		rt := rv.Type()
		if rt.NumIn() != 1 || rt.NumOut() != 0 {
			panic("invalid function signature. must be: func(err error)")
		}

		e := rt.In(0)
		if !e.Implements(errorInterface) {
			panic("invalid function signature. must be: func(err error)")
		}
	case "reconnect_failed":
		rv := reflect.ValueOf(handler)

		if rv.Kind() != reflect.Func {
			panic("function expected")
		}

		rt := rv.Type()
		if rt.NumIn() != 0 || rt.NumOut() != 0 {
			panic("invalid function signature. must be: func()")
		}
	case "ping":
		rv := reflect.ValueOf(handler)

		if rv.Kind() != reflect.Func {
			panic("function expected")
		}

		rt := rv.Type()
		if rt.NumIn() != 0 || rt.NumOut() != 0 {
			panic("invalid function signature. must be: func()")
		}
	}
}
