package sio

import (
	"fmt"
	"reflect"
	"time"

	"github.com/tomruk/socket.io-go/internal/sync"
)

type eventHandler struct {
	rv        reflect.Value
	inputArgs []reflect.Type
}

func newEventHandler(f any) (*eventHandler, error) {
	rv := reflect.ValueOf(f)
	rt := rv.Type()

	if f == nil || rv.Kind() != reflect.Func {
		return nil, fmt.Errorf("sio: function expected")
	}

	inputArgs := make([]reflect.Type, rt.NumIn())
	for i := range inputArgs {
		inputArgs[i] = rt.In(i)
	}

	if rt.NumOut() > 0 {
		return nil, fmt.Errorf("sio: an event handler cannot have return values")
	}

	s := &eventHandler{
		rv:        rv,
		inputArgs: inputArgs,
	}
	_, err := s.ack()
	return s, err
}

func (s *eventHandler) ack() (ok bool, err error) {
	if len(s.inputArgs) > 0 && s.inputArgs[len(s.inputArgs)-1].Kind() == reflect.Func {
		if s.inputArgs[len(s.inputArgs)-1].NumOut() > 0 {
			err = fmt.Errorf("sio: an acknowledgement function of an event handler cannot have return values")
			return
		}
		ok = true
	}
	return
}

func (f *eventHandler) call(args ...reflect.Value) (ret []reflect.Value, err error) {
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
	rv        reflect.Value
	inputArgs []reflect.Type

	hasError bool

	called   bool
	timedOut bool
	mu       sync.Mutex
}

func newAckHandler(f any, hasError bool) (*ackHandler, error) {
	rv := reflect.ValueOf(f)
	rt := rv.Type()

	if f == nil || rv.Kind() != reflect.Func {
		return nil, fmt.Errorf("sio: function expected")
	}

	inputArgs := make([]reflect.Type, rt.NumIn())
	for i := range inputArgs {
		inputArgs[i] = rt.In(i)
	}

	err := checkAckFunc(f, hasError)
	if err != nil {
		return nil, err
	}

	return &ackHandler{
		rv:        rv,
		inputArgs: inputArgs,
		hasError:  hasError,
	}, nil
}

func newAckHandlerWithTimeout(f any, timeout time.Duration, timeoutFunc func()) (*ackHandler, error) {
	h, err := newAckHandler(f, true)
	if err != nil {
		return nil, err
	}
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
			_ = recover()
		}()

		timeoutFunc()

		args := make([]reflect.Value, len(h.inputArgs))
		args[0] = reflect.ValueOf(fmt.Errorf("operation has timed out"))
		for i := 1; i < len(args); i++ {
			args[i] = reflect.New(h.inputArgs[i]).Elem()
		}
		h.rv.Call(args)
	}()
	return h, nil
}

func (f *ackHandler) call(args ...reflect.Value) (err error) {
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

func (f *ackHandler) callWithError(e error, args ...reflect.Value) (err error) {
	if !f.hasError {
		panic(fmt.Errorf("sio: hasError is false. this shouldn't have happened"))
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

func dismantleAckFunc(rt reflect.Type) (in []reflect.Type, variadic bool) {
	in = make([]reflect.Type, rt.NumIn())
	for i := range in {
		in[i] = rt.In(i)
	}
	variadic = rt.IsVariadic()
	return
}

var (
	_emptyError  error
	reflectError = reflect.TypeOf(&_emptyError).Elem()
)

func checkAckFunc(f any, mustHaveError bool) error {
	rt := reflect.TypeOf(f)

	if f == nil || rt.Kind() != reflect.Func {
		return fmt.Errorf("sio: function expected")
	}

	if rt.NumOut() != 0 {
		return fmt.Errorf("sio: ack handler must not have a return value")
	}

	if mustHaveError {
		if rt.NumIn() == 0 {
			return fmt.Errorf("sio: ack handler must have error as its 1st parameter")
		}
		if rt.In(0).Kind() != reflect.Interface || !rt.In(0).Implements(reflectError) {
			return fmt.Errorf("sio: ack handler must have error as its 1st parameter")
		}
	}
	return nil
}
