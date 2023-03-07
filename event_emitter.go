package sio

import "sync"

type eventEmitter[T comparable] struct {
	mu         sync.Mutex
	events     []T
	eventsOnce []T
}

func newEventEmitter[T comparable]() *eventEmitter[T] {
	return new(eventEmitter[T])
}

func (e *eventEmitter[T]) On(handler T) {
	e.mu.Lock()
	e.events = append(e.events, handler)
	e.mu.Unlock()
}

func (e *eventEmitter[T]) Once(handler T) {
	e.mu.Lock()
	e.eventsOnce = append(e.eventsOnce, handler)
	e.mu.Unlock()
}

func (e *eventEmitter[T]) Off(handler ...T) {
	e.mu.Lock()
	defer e.mu.Unlock()

	remove := func(slice []T, s int) []T {
		return append(slice[:s], slice[s+1:]...)
	}

	for i, h := range e.events {
		for _, _h := range handler {
			if h == _h {
				e.events = remove(e.events, i)
			}
		}
	}

	for i, h := range e.eventsOnce {
		for _, _h := range handler {
			if h == _h {
				e.eventsOnce = remove(e.eventsOnce, i)
			}
		}
	}
}

func (e *eventEmitter[T]) OffAll() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.events = nil
	e.eventsOnce = nil
}

func (e *eventEmitter[T]) GetHandlers(eventName string) (handlers []T) {
	e.mu.Lock()
	defer e.mu.Unlock()

	handlers = make([]T, 0, len(e.events)+len(e.eventsOnce))
	handlers = append(handlers, e.events...)
	handlers = append(handlers, e.eventsOnce...)
	e.eventsOnce = nil
	return
}
