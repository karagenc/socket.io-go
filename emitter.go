package sio

import "time"

type Emitter struct {
	socket  emitter
	timeout time.Duration
}

type emitter interface {
	emit(eventName string, timeout time.Duration, v ...any)
}

func (e *Emitter) Emit(eventName string, v ...any) {
	e.socket.emit(eventName, e.timeout, v...)
}
