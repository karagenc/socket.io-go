package sio

import "sync"

type eventHandlerStore struct {
	mu         sync.Mutex
	events     map[string][]*eventHandler
	eventsOnce map[string][]*eventHandler
}

func newEventHandlerStore() *eventHandlerStore {
	return &eventHandlerStore{
		events:     make(map[string][]*eventHandler),
		eventsOnce: make(map[string][]*eventHandler),
	}
}

func (e *eventHandlerStore) On(eventName string, handler *eventHandler) {
	e.mu.Lock()
	handlers, _ := e.events[eventName]
	handlers = append(handlers, handler)
	e.events[eventName] = handlers
	e.mu.Unlock()
}

func (e *eventHandlerStore) Once(eventName string, handler *eventHandler) {
	e.mu.Lock()
	handlers, _ := e.eventsOnce[eventName]
	handlers = append(handlers, handler)
	e.eventsOnce[eventName] = handlers
	e.mu.Unlock()
}

func (e *eventHandlerStore) Off(eventName string, handler ...any) {
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
			for _, _h := range handler {
				if h.rv.Interface() == _h {
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

func (e *eventHandlerStore) OffAll() {
	e.mu.Lock()
	defer e.mu.Unlock()

	for k := range e.events {
		delete(e.events, k)
	}

	for k := range e.eventsOnce {
		delete(e.eventsOnce, k)
	}
}

func (e *eventHandlerStore) GetAll(eventName string) (handlers []*eventHandler) {
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
