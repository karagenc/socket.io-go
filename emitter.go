package sio

import "sync"

type emitter[T comparable] struct {
	mu        sync.Mutex
	funcs     []T
	funcsOnce []T
}

func newEmitter[T comparable]() *emitter[T] {
	return new(emitter[T])
}

func (e *emitter[T]) On(handler T) {
	e.mu.Lock()
	e.funcs = append(e.funcs, handler)
	e.mu.Unlock()
}

func (e *emitter[T]) Once(handler T) {
	e.mu.Lock()
	e.funcsOnce = append(e.funcsOnce, handler)
	e.mu.Unlock()
}

func (e *emitter[T]) Off(handler ...T) {
	e.mu.Lock()
	defer e.mu.Unlock()

	remove := func(slice []T, s int) []T {
		return append(slice[:s], slice[s+1:]...)
	}

	for i, h := range e.funcs {
		for _, _h := range handler {
			if h == _h {
				e.funcs = remove(e.funcs, i)
			}
		}
	}

	for i, h := range e.funcsOnce {
		for _, _h := range handler {
			if h == _h {
				e.funcsOnce = remove(e.funcsOnce, i)
			}
		}
	}
}

func (e *emitter[T]) OffAll() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.funcs = nil
	e.funcsOnce = nil
}

func (e *emitter[T]) GetHandlers(eventName string) (handlers []T) {
	e.mu.Lock()
	defer e.mu.Unlock()

	handlers = make([]T, 0, len(e.funcs)+len(e.funcsOnce))
	handlers = append(handlers, e.funcs...)
	handlers = append(handlers, e.funcsOnce...)
	e.funcsOnce = nil
	return
}
