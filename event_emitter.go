package sio

import "sync"

type eventEmitter struct {
	mu         sync.Mutex
	events     map[string][]*eventHandler
	eventsOnce map[string][]*eventHandler
}

func newEventEmitter() *eventEmitter {
	return &eventEmitter{
		events:     make(map[string][]*eventHandler),
		eventsOnce: make(map[string][]*eventHandler),
	}
}

func (e *eventEmitter) On(eventName string, handler interface{}) {
	if handler == nil {
		return
	}

	e.mu.Lock()
	handlers, _ := e.events[eventName]
	handlers = append(handlers, newEventHandler(handler))
	e.events[eventName] = handlers
	e.mu.Unlock()
}

func (e *eventEmitter) Once(eventName string, handler interface{}) {
	if handler == nil {
		return
	}

	e.mu.Lock()
	handlers, _ := e.eventsOnce[eventName]
	handlers = append(handlers, newEventHandler(handler))
	e.eventsOnce[eventName] = handlers
	e.mu.Unlock()
}

func (e *eventEmitter) Off(eventName string, handler interface{}) {
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

	remove := func(slice []*eventHandler, s int) []*eventHandler {
		return append(slice[:s], slice[s+1:]...)
	}

	handlers, ok := e.events[eventName]
	if ok {
		for i, h := range handlers {
			if h == handler {
				handlers = remove(handlers, i)
			}
		}
		e.events[eventName] = handlers
	}

	handlers, ok = e.eventsOnce[eventName]
	if ok {
		for i, h := range handlers {
			if h == handler {
				handlers = remove(handlers, i)
			}
		}
		e.eventsOnce[eventName] = handlers
	}
}

func (e *eventEmitter) OffAll() {
	e.mu.Lock()
	defer e.mu.Unlock()

	for k := range e.events {
		delete(e.events, k)
	}

	for k := range e.eventsOnce {
		delete(e.eventsOnce, k)
	}
}

func (e *eventEmitter) GetHandlers(eventName string) (handlers []*eventHandler) {
	e.mu.Lock()
	defer e.mu.Unlock()

	h, _ := e.events[eventName]
	hOnce, _ := e.eventsOnce[eventName]

	delete(e.eventsOnce, eventName)

	handlers = make([]*eventHandler, 0, len(h)+len(hOnce))
	handlers = append(handlers, h...)
	handlers = append(handlers, hOnce...)

	return
}
