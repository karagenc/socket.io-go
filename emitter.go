package sio

import (
	"reflect"
	"time"
)

type Emitter struct {
	socket   emitter
	timeout  time.Duration
	volatile bool
}

type emitter interface {
	emit(eventName string, timeout time.Duration, volatile, fromQueue bool, v ...any)
}

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
