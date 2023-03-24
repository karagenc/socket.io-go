package sio

import (
	"reflect"
	"sync"

	"github.com/tomruk/socket.io-go/adapter"
	"github.com/tomruk/socket.io-go/parser"
)

type (
	clientSocketStore struct {
		sockets map[string]*clientSocket
		mu      sync.Mutex
	}

	serverSocketStore struct {
		socketsByID  map[SocketID]*serverSocket
		socketsByNsp map[string]*serverSocket
		mu           sync.Mutex
	}

	// We could've used mapset instead of this,
	// but mapset doesn't have an equivalent of the GetOrCreate method.
	nspStore struct {
		nsps map[string]*Namespace
		mu   sync.Mutex
	}

	nspSocketStore struct {
		sockets map[SocketID]ServerSocket
		mu      sync.Mutex
	}

	// This is to ensure we have a socket store with a
	// right function signature that matches with adapter's
	// `SocketStore`.
	adapterSocketStore struct {
		store *nspSocketStore
	}

	handlerStore[T comparable] struct {
		mu        sync.Mutex
		funcs     []T
		funcsOnce []T
		subs      []T
	}

	eventHandlerStore struct {
		mu         sync.Mutex
		events     map[string][]*eventHandler
		eventsOnce map[string][]*eventHandler
	}
)

func newClientSocketStore() *clientSocketStore {
	return &clientSocketStore{sockets: make(map[string]*clientSocket)}
}

func newServerSocketStore() *serverSocketStore {
	return &serverSocketStore{
		socketsByID:  make(map[SocketID]*serverSocket),
		socketsByNsp: make(map[string]*serverSocket),
	}
}

func newNspStore() *nspStore {
	return &nspStore{nsps: make(map[string]*Namespace)}
}

func newNspSocketStore() *nspSocketStore {
	return &nspSocketStore{sockets: make(map[SocketID]ServerSocket)}
}

func newAdapterSocketStore(store *nspSocketStore) *adapterSocketStore {
	return &adapterSocketStore{store: store}
}

func newHandlerStore[T comparable]() *handlerStore[T] {
	return new(handlerStore[T])
}

func newEventHandlerStore() *eventHandlerStore {
	return &eventHandlerStore{
		events:     make(map[string][]*eventHandler),
		eventsOnce: make(map[string][]*eventHandler),
	}
}

func (s *clientSocketStore) get(nsp string) (socket *clientSocket, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	socket, ok = s.sockets[nsp]
	return
}

func (s *clientSocketStore) getAll() (sockets []*clientSocket) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sockets = make([]*clientSocket, len(s.sockets))
	i := 0
	for _, socket := range s.sockets {
		sockets[i] = socket
		i++
	}
	return
}

func (s *clientSocketStore) set(socket *clientSocket) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sockets[socket.namespace] = socket
}

func (s *clientSocketStore) remove(namespace string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sockets, namespace)
}

func (s *serverSocketStore) getByID(sid SocketID) (socket *serverSocket, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	socket, ok = s.socketsByID[sid]
	return
}

func (s *serverSocketStore) getByNsp(nsp string) (socket *serverSocket, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	socket, ok = s.socketsByNsp[nsp]
	return
}

func (s *serverSocketStore) getAll() (sockets []*serverSocket) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sockets = make([]*serverSocket, len(s.socketsByID))
	i := 0
	for _, socket := range s.socketsByID {
		sockets[i] = socket
		i++
	}
	return
}

func (s *serverSocketStore) getAndRemoveAll() (sockets []*serverSocket) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sockets = make([]*serverSocket, len(s.socketsByID))
	i := 0
	for _, socket := range s.socketsByID {
		sockets[i] = socket
		i++
	}
	s.socketsByID = make(map[SocketID]*serverSocket)
	s.socketsByNsp = make(map[string]*serverSocket)
	return
}

func (s *serverSocketStore) set(socket *serverSocket) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.socketsByID[socket.ID()] = socket
	s.socketsByNsp[socket.nsp.Name()] = socket
}

func (s *serverSocketStore) removeByID(sid SocketID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	socket, ok := s.socketsByID[sid]
	if ok {
		delete(s.socketsByID, sid)
		delete(s.socketsByNsp, socket.nsp.Name())
	}
}

func (s *nspStore) getOrCreate(
	name string,
	server *Server,
	adapterCreator adapter.Creator,
	parserCreator parser.Creator,
) (nsp *Namespace, created bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var ok bool
	nsp, ok = s.nsps[name]
	if !ok {
		nsp = newNamespace(name, server, adapterCreator, parserCreator)
		s.nsps[nsp.Name()] = nsp
		created = true
	}
	return
}

func (s *nspStore) len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.nsps)
}

func (s *nspStore) set(nsp *Namespace) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nsps[nsp.Name()] = nsp
}

func (s *nspStore) get(name string) (nsp *Namespace, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	nsp, ok = s.nsps[name]
	return
}

func (s *nspStore) remove(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.nsps, name)
}

// Send Engine.IO packets to a specific socket.
func (s *nspSocketStore) sendBuffers(sid SocketID, buffers [][]byte) (ok bool) {
	_socket, ok := s.get(sid)
	if !ok {
		return false
	}
	socket := _socket.(*serverSocket)
	socket.conn.sendBuffers(buffers...)
	return true
}

func (s *nspSocketStore) get(sid SocketID) (socket ServerSocket, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	socket, ok = s.sockets[sid]
	return socket, ok
}

func (s *nspSocketStore) getAll() []ServerSocket {
	s.mu.Lock()
	defer s.mu.Unlock()

	sockets := make([]ServerSocket, len(s.sockets))
	i := 0
	for _, s := range s.sockets {
		sockets[i] = s
		i++
	}
	return sockets
}

func (s *nspSocketStore) set(socket ServerSocket) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sockets[socket.ID()] = socket
}

func (s *nspSocketStore) remove(sid SocketID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sockets, sid)
}

// Send Engine.IO packets to a specific socket.
func (s *adapterSocketStore) SendBuffers(sid SocketID, buffers [][]byte) (ok bool) {
	return s.store.sendBuffers(sid, buffers)
}

func (s *adapterSocketStore) Get(sid SocketID) (socket adapter.Socket, ok bool) {
	return s.store.get(sid)
}

func (s *adapterSocketStore) GetAll() []adapter.Socket {
	_sockets := s.store.getAll()
	sockets := make([]adapter.Socket, len(_sockets))
	for i := range sockets {
		sockets[i] = _sockets[i]
	}
	return sockets
}

func (s *adapterSocketStore) Remove(sid SocketID) {
	s.store.remove(sid)
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

func (e *eventHandlerStore) on(eventName string, handler *eventHandler) {
	e.mu.Lock()
	handlers, _ := e.events[eventName]
	handlers = append(handlers, handler)
	e.events[eventName] = handlers
	e.mu.Unlock()
}

func (e *eventHandlerStore) once(eventName string, handler *eventHandler) {
	e.mu.Lock()
	handlers, _ := e.eventsOnce[eventName]
	handlers = append(handlers, handler)
	e.eventsOnce[eventName] = handlers
	e.mu.Unlock()
}

func (e *eventHandlerStore) off(eventName string, handler ...reflect.Value) {
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

	events, ok := e.events[eventName]
	if ok {
		for i, event := range events {
			for _, h := range handler {
				ep := event.rv.Pointer()
				hp := h.Pointer()
				if ep == hp {
					events = remove(events, i)
				}
			}
		}
		if len(events) == 0 {
			delete(e.events, eventName)
		} else {
			e.events[eventName] = events
		}
	}

	eventsOnce, ok := e.eventsOnce[eventName]
	if ok {
		for i, event := range eventsOnce {
			for _, h := range handler {
				ep := event.rv.Pointer()
				hp := h.Pointer()
				if ep == hp {
					eventsOnce = remove(eventsOnce, i)
				}
			}
		}
		if len(eventsOnce) == 0 {
			delete(e.eventsOnce, eventName)
		} else {
			e.eventsOnce[eventName] = eventsOnce
		}
	}
}

func (e *eventHandlerStore) offAll() {
	e.mu.Lock()
	defer e.mu.Unlock()

	for k := range e.events {
		delete(e.events, k)
	}

	for k := range e.eventsOnce {
		delete(e.eventsOnce, k)
	}
}

func (e *eventHandlerStore) getAll(eventName string) (handlers []*eventHandler) {
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
