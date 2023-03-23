package sio

import "sync"

type handlerStore[T comparable] struct {
	mu        sync.Mutex
	funcs     []T
	funcsOnce []T
	subs      []T
}

func newHandlerStore[T comparable]() *handlerStore[T] {
	return new(handlerStore[T])
}

func (e *handlerStore[T]) on(handler T) {
	e.mu.Lock()
	e.funcs = append(e.funcs, handler)
	e.mu.Unlock()
}

func (e *handlerStore[T]) onSubEvent(handler T) {
	e.mu.Lock()
	e.subs = append(e.subs, handler)
	e.mu.Unlock()
}

func (e *handlerStore[T]) offSubEvents() {
	e.mu.Lock()
	e.subs = nil
	e.mu.Unlock()
}

func (e *handlerStore[T]) once(handler T) {
	e.mu.Lock()
	e.funcsOnce = append(e.funcsOnce, handler)
	e.mu.Unlock()
}

func (e *handlerStore[T]) off(handler ...T) {
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

func (e *handlerStore[T]) offAll() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.funcs = nil
	e.funcsOnce = nil
}

func (e *handlerStore[T]) getAll() (handlers []T) {
	e.mu.Lock()
	defer e.mu.Unlock()

	handlers = make([]T, 0, len(e.subs)+len(e.funcs)+len(e.funcsOnce))
	handlers = append(handlers, e.subs...)
	handlers = append(handlers, e.funcs...)
	handlers = append(handlers, e.funcsOnce...)
	e.funcsOnce = nil
	return
}
