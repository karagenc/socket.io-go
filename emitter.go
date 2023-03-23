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
	hasAck := len(v) != 0 && reflect.TypeOf(v[len(v)-1]).Kind() == reflect.Func
	if hasAck && e.timeout != 0 {
		err := doesAckHandlerHasAnError(v[len(v)-1])
		if err != nil {
			panic(err)
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
