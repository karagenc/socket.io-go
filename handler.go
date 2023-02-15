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

func newAckHandler(v interface{}) *ackHandler {
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

func checkHandler(eventName string, handler interface{}) error {
	switch eventName {
	case "":
		return fmt.Errorf("event name cannot be empty")
	case "connect":
		rv := reflect.ValueOf(handler)

		if rv.Kind() != reflect.Func {
			return fmt.Errorf("'connect': function expected")
		}

		rt := rv.Type()
		if rt.NumIn() != 0 || rt.NumOut() != 0 {
			return fmt.Errorf("invalid function signature for event 'connect'. must be: func()")
		}
	case "connect_error":
		rv := reflect.ValueOf(handler)

		if rv.Kind() != reflect.Func {
			return fmt.Errorf("'connect_error': function expected")
		}

		rt := rv.Type()
		if rt.NumIn() != 1 || rt.NumOut() != 0 {
			return fmt.Errorf("invalid function signature for event 'connect_error'. must be: func(err error)")
		}

		e := rt.In(0)
		if !e.Implements(errorInterface) {
			return fmt.Errorf("invalid function signature for event 'connect_error'. must be: func(err error)")
		}
	case "disconnect":
		rv := reflect.ValueOf(handler)

		if rv.Kind() != reflect.Func {
			return fmt.Errorf("'disconnect': function expected")
		}

		rt := rv.Type()
		if rt.NumIn() != 0 || rt.NumOut() != 0 {
			return fmt.Errorf("invalid function signature for event 'disconnect'. must be: func()")
		}
	case "open":
		rv := reflect.ValueOf(handler)

		if rv.Kind() != reflect.Func {
			return fmt.Errorf("'open': function expected")
		}

		rt := rv.Type()
		if rt.NumIn() != 0 || rt.NumOut() != 0 {
			return fmt.Errorf("invalid function signature for event 'open'. must be: func()")
		}
	case "close":
		rv := reflect.ValueOf(handler)

		if rv.Kind() != reflect.Func {
			return fmt.Errorf("'close': function expected")
		}

		rt := rv.Type()
		if rt.NumIn() != 1 || rt.NumOut() != 0 {
			return fmt.Errorf("invalid function signature for event 'close'. must be: func(reason string)")
		}

		e := rt.In(0)
		if e.Kind() != reflect.String {
			return fmt.Errorf("invalid function signature for event 'close'. must be: func(reason string)")
		}
	case "error":
		rv := reflect.ValueOf(handler)

		if rv.Kind() != reflect.Func {
			return fmt.Errorf("'error': function expected")
		}

		rt := rv.Type()
		if rt.NumIn() != 1 || rt.NumOut() != 0 {
			return fmt.Errorf("invalid function signature for event 'error'. must be: func(err error)")
		}

		e := rt.In(0)
		if !e.Implements(errorInterface) {
			return fmt.Errorf("invalid function signature for event 'error'. must be: func(err error)")
		}
	case "reconnect":
		rv := reflect.ValueOf(handler)

		if rv.Kind() != reflect.Func {
			return fmt.Errorf("'reconnect': function expected")
		}

		rt := rv.Type()
		if rt.NumIn() != 1 || rt.NumOut() != 0 {
			return fmt.Errorf("invalid function signature for event 'reconnect'. must be: func(attempt int)")
		}

		e := rt.In(0)
		if e.Kind() != reflect.Int32 {
			return fmt.Errorf("invalid function signature for event 'reconnect'. must be: func(attempt int)")
		}
	case "reconnect_attempt":
		rv := reflect.ValueOf(handler)

		if rv.Kind() != reflect.Func {
			return fmt.Errorf("'reconnect_attempt': function expected")
		}

		rt := rv.Type()
		if rt.NumIn() != 1 || rt.NumOut() != 0 {
			return fmt.Errorf("invalid function signature for event 'reconnect_attempt'. must be: func(attempt int)")
		}

		e := rt.In(0)
		if e.Kind() != reflect.Int32 {
			return fmt.Errorf("invalid function signature for event 'reconnect_attempt'. must be: func(attempt int)")
		}
	case "reconnect_error":
		rv := reflect.ValueOf(handler)

		if rv.Kind() != reflect.Func {
			return fmt.Errorf("'reconnect_error': function expected")
		}

		rt := rv.Type()
		if rt.NumIn() != 1 || rt.NumOut() != 0 {
			return fmt.Errorf("invalid function signature for event 'reconnect_error'. must be: func(err error)")
		}

		e := rt.In(0)
		if !e.Implements(errorInterface) {
			return fmt.Errorf("invalid function signature for event 'reconnect_error'. must be: func(err error)")
		}
	case "reconnect_failed":
		rv := reflect.ValueOf(handler)

		if rv.Kind() != reflect.Func {
			return fmt.Errorf("'reconnect_failed': function expected")
		}

		rt := rv.Type()
		if rt.NumIn() != 0 || rt.NumOut() != 0 {
			return fmt.Errorf("invalid function signature for event 'reconnect_failed'. must be: func()")
		}
	case "ping":
		rv := reflect.ValueOf(handler)

		if rv.Kind() != reflect.Func {
			return fmt.Errorf("'ping': function expected")
		}

		rt := rv.Type()
		if rt.NumIn() != 0 || rt.NumOut() != 0 {
			return fmt.Errorf("invalid function signature for event 'ping'. must be: func()")
		}
	}
	return nil
}

var serverSocketInterface = reflect.TypeOf((*ServerSocket)(nil)).Elem()

func checkNamespaceHandler(eventName string, handler interface{}) error {
	switch eventName {
	case "":
		return fmt.Errorf("event name cannot be empty")
	case "connect":
		rv := reflect.ValueOf(handler)

		if rv.Kind() != reflect.Func {
			return fmt.Errorf("'connect': function expected")
		}

		rt := rv.Type()
		if rt.NumIn() != 1 || rt.NumOut() != 0 {
			return fmt.Errorf("invalid function signature for event 'connect'. must be: func(socket ServerSocket)")
		}

		e := rt.In(0)
		if !e.Implements(serverSocketInterface) {
			return fmt.Errorf("invalid function signature for event 'connect'. must be: func(socket ServerSocket)")
		}
	case "connection":
		rv := reflect.ValueOf(handler)

		if rv.Kind() != reflect.Func {
			return fmt.Errorf("'connection': function expected")
		}

		rt := rv.Type()
		if rt.NumIn() != 1 || rt.NumOut() != 0 {
			return fmt.Errorf("invalid function signature for event 'connection'. must be: func(socket ServerSocket)")
		}

		e := rt.In(0)
		if !e.Implements(serverSocketInterface) {
			return fmt.Errorf("invalid function signature for event 'connection'. must be: func(socket ServerSocket)")
		}
	}
	return nil
}
