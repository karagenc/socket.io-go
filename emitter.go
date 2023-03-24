package sio

import (
	"reflect"
	"time"
)

type (
	Emitter struct {
		socket   emitter
		timeout  time.Duration
		volatile bool
	}

	emitter interface {
		Socket
		emit(eventName string, timeout time.Duration, volatile, fromQueue bool, v ...any)
	}
)

func (e *Emitter) Socket() Socket { return e.socket }

func (e *Emitter) Emit(eventName string, v ...any) {
	if len(v) != 0 {
		f := v[len(v)-1]
		// Is `f` an ack function?
		if e.timeout != 0 && f != nil && reflect.TypeOf(f).Kind() == reflect.Func {
			err := checkAckFunc(f, true)
			if err != nil {
				panic(err)
			}
		}
	}
	e.socket.emit(eventName, e.timeout, e.volatile, false, v...)
}

func (e Emitter) Timeout(timeout time.Duration) Emitter {
	e.timeout = timeout
	return e
}

func (e Emitter) Volatile() Emitter {
	e.volatile = true
	return e
}
