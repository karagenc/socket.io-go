package sio

import "sync"

type eventEmitter[T comparable] struct {
	mu         sync.Mutex
	events     map[string][]T
	eventsOnce map[string][]T
}

func newEventEmitter[T comparable]() *eventEmitter[T] {
	return &eventEmitter[T]{
		events:     make(map[string][]T),
		eventsOnce: make(map[string][]T),
	}
}

func (e *eventEmitter[T]) On(eventName string, handler T) {
	e.mu.Lock()
	handlers, _ := e.events[eventName]
	handlers = append(handlers, handler)
	e.events[eventName] = handlers
	e.mu.Unlock()
}

func (e *eventEmitter[T]) Once(eventName string, handler T) {
	e.mu.Lock()
	handlers, _ := e.eventsOnce[eventName]
	handlers = append(handlers, handler)
	e.eventsOnce[eventName] = handlers
	e.mu.Unlock()
}

func (e *eventEmitter[T]) Off(eventName string, handler ...T) {
	if eventName == "" {
		return
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if handler == nil {
		delete(e.events, eventName)
		delete(e.eventsOnce, eventName)
		return
	}

	remove := func(slice []T, s int) []T {
		return append(slice[:s], slice[s+1:]...)
	}

	handlers, ok := e.events[eventName]
	if ok {
		for i, h := range handlers {
			for _, _h := range handler {
				if h == _h {
					handlers = remove(handlers, i)
				}
			}
		}
		e.events[eventName] = handlers
	}

	handlers, ok = e.eventsOnce[eventName]
	if ok {
		for i, h := range handlers {
			for _, _h := range handler {
				if h == _h {
					handlers = remove(handlers, i)
				}
			}
		}
		e.eventsOnce[eventName] = handlers
	}
}

func (e *eventEmitter[T]) OffAll() {
	e.mu.Lock()
	defer e.mu.Unlock()

	for k := range e.events {
		delete(e.events, k)
	}

	for k := range e.eventsOnce {
		delete(e.eventsOnce, k)
	}
}

func (e *eventEmitter[T]) GetHandlers(eventName string) (handlers []T) {
	e.mu.Lock()
	defer e.mu.Unlock()

	h, _ := e.events[eventName]
	hOnce, _ := e.eventsOnce[eventName]

	delete(e.eventsOnce, eventName)

	handlers = make([]T, 0, len(h)+len(hOnce))
	handlers = append(handlers, h...)
	handlers = append(handlers, hOnce...)
	return
}
