package sio

import "sync"

type Namespace struct {
	name    string
	sockets *namespaceSocketStore
	adapter Adapter
}

type namespaceSocketStore struct {
	sockets map[string]*serverSocket
	mu      sync.Mutex
}

func newNamespaceSocketStore() *namespaceSocketStore {
	return &namespaceSocketStore{
		sockets: make(map[string]*serverSocket),
	}
}

func (s *namespaceSocketStore) Get(sid string) (so *serverSocket, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	so, ok = s.sockets[sid]
	return so, ok
}

func (s *namespaceSocketStore) Set(so *serverSocket) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sockets[so.ID()] = so
}

func (s *namespaceSocketStore) Remove(sid string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sockets, sid)
}

func newNamespace(name string, adapterCreator AdapterCreator) *Namespace {
	nsp := &Namespace{
		name:    name,
		sockets: newNamespaceSocketStore(),
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
