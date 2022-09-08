package sio

import (
	"encoding/json"
	"reflect"
	"sync"
	"time"

	"github.com/tomruk/socket.io-go/parser"
)

type Namespace struct {
	name   string
	server *Server

	sockets *NamespaceSocketStore

	middlewareFuncs   []MiddlewareFunction
	middlewareFuncsMu sync.RWMutex

	adapter Adapter
	parser  parser.Parser

	emitter *eventEmitter
}

type namespaceStore struct {
	nsps map[string]*Namespace
	mu   sync.Mutex
}

func newNamespaceStore() *namespaceStore {
	return &namespaceStore{
		nsps: make(map[string]*Namespace),
	}
}

func (s *namespaceStore) Set(nsp *Namespace) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nsps[nsp.Name()] = nsp
}

func (s *namespaceStore) Get(name string) (nsp *Namespace, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	nsp, ok = s.nsps[name]
	return
}

func (s *namespaceStore) GetOrCreate(name string, server *Server, adapterCreator AdapterCreator, parserCreator parser.Creator) *Namespace {
	s.mu.Lock()
	defer s.mu.Unlock()
	nsp, ok := s.nsps[name]
	if !ok {
		nsp = newNamespace(name, server, adapterCreator, parserCreator)
		s.nsps[nsp.Name()] = nsp
	}
	return nsp
}

func (s *namespaceStore) Remove(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.nsps, name)
}

type NamespaceSocketStore struct {
	sockets map[string]*serverSocket
	mu      sync.Mutex
}

func newNamespaceSocketStore() *NamespaceSocketStore {
	return &NamespaceSocketStore{
		sockets: make(map[string]*serverSocket),
	}
}

func (s *NamespaceSocketStore) Get(sid string) (so *serverSocket, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	so, ok = s.sockets[sid]
	return so, ok
}

// Send Engine.IO packets to a specific socket.
func (s *NamespaceSocketStore) SendBuffers(sid string, buffers [][]byte) (ok bool) {
	socket, ok := s.Get(sid)
	if !ok {
		return false
	}
	socket.conn.sendBuffers(buffers...)
	return true
}

func (s *NamespaceSocketStore) GetAll() []Socket {
	s.mu.Lock()
	defer s.mu.Unlock()

	sockets := make([]Socket, len(s.sockets))
	i := 0
	for _, s := range s.sockets {
		sockets[i] = s
		i++
	}
	return sockets
}

func (s *NamespaceSocketStore) Set(so *serverSocket) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sockets[so.ID()] = so
}

func (s *NamespaceSocketStore) Remove(sid string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sockets, sid)
}

func newNamespace(name string, server *Server, adapterCreator AdapterCreator, parserCreator parser.Creator) *Namespace {
	nsp := &Namespace{
		name:    name,
		server:  server,
		sockets: newNamespaceSocketStore(),
		parser:  parserCreator(),
		emitter: newEventEmitter(),
	}
	nsp.adapter = adapterCreator(nsp)
	return nsp
}

func (n *Namespace) Name() string {
	return n.name
}

func (n *Namespace) Adapter() Adapter {
	return n.adapter
}

func (n *Namespace) SocketStore() *NamespaceSocketStore {
	return n.sockets
}

func (n *Namespace) Sockets() []Socket {
	return n.sockets.GetAll()
}

func (n *Namespace) Emit(eventName string, v ...interface{}) {
	newBroadcastOperator(n.Name(), n.adapter, n.parser).Emit(eventName, v...)
}

func (n *Namespace) To(room ...string) *broadcastOperator {
	return newBroadcastOperator(n.Name(), n.adapter, n.parser).To(room...)
}

func (n *Namespace) In(room ...string) *broadcastOperator {
	return newBroadcastOperator(n.Name(), n.adapter, n.parser).In(room...)
}

func (n *Namespace) Except(room ...string) *broadcastOperator {
	return newBroadcastOperator(n.Name(), n.adapter, n.parser).Except(room...)
}

func (n *Namespace) Compress(compress bool) *broadcastOperator {
	return newBroadcastOperator(n.Name(), n.adapter, n.parser).Compress(compress)
}

func (n *Namespace) Local() *broadcastOperator {
	return newBroadcastOperator(n.Name(), n.adapter, n.parser).Local()
}

func (n *Namespace) AllSockets() (sids []string) {
	return newBroadcastOperator(n.Name(), n.adapter, n.parser).AllSockets()
}

type MiddlewareFunction func(socket Socket, handshake *Handshake) error

func (n *Namespace) Use(f MiddlewareFunction) {
	n.middlewareFuncsMu.Lock()
	defer n.middlewareFuncsMu.Unlock()
	n.middlewareFuncs = append(n.middlewareFuncs, f)
}

func (n *Namespace) add(c *serverConn, auth json.RawMessage) (*serverSocket, error) {
	handshake := &Handshake{
		Time: time.Now(),
		Auth: auth,
	}

	socket, err := newServerSocket(n.server, c, n, c.parser)
	if err != nil {
		return nil, err
	}

	n.middlewareFuncsMu.RLock()
	defer n.middlewareFuncsMu.RUnlock()

	for _, f := range n.middlewareFuncs {
		err := f(socket, handshake)
		if err != nil {
			return nil, err
		}
	}

	n.adapter.AddAll(socket.ID(), []string{socket.ID()})
	n.sockets.Set(socket)
	go n.onSocket(socket)

	return socket, nil
}

func (n *Namespace) onSocket(socket Socket) {
	connectHandlers := n.emitter.GetHandlers("connect")
	connectionHandlers := n.emitter.GetHandlers("connection")

	callHandler := func(handler *eventHandler) {
		_, err := handler.Call(reflect.ValueOf(socket))
		if err != nil {
			// TODO: n.onError(err)???
			return
		}
	}

	for _, handler := range connectHandlers {
		go callHandler(handler)
	}
	for _, handler := range connectionHandlers {
		go callHandler(handler)
	}
}

func (n *Namespace) remove(socket *serverSocket) {
	n.sockets.Remove(socket.ID())
}

func (n *Namespace) On(eventName string, handler interface{}) {
	n.checkHandler(eventName, handler)
	n.emitter.On(eventName, handler)
}

func (n *Namespace) Once(eventName string, handler interface{}) {
	n.checkHandler(eventName, handler)
	n.emitter.Once(eventName, handler)
}

func (n *Namespace) checkHandler(eventName string, handler interface{}) {
	switch eventName {
	case "":
		fallthrough
	case "connect":
		fallthrough
	case "connection":
		checkNamespaceHandler(eventName, handler)
	}
}

func (n *Namespace) Off(eventName string, handler interface{}) {
	n.emitter.Off(eventName, handler)
}

func (n *Namespace) OffAll() {
	n.emitter.OffAll()
}
