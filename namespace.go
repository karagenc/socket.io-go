package sio

import (
	"sync"

	eioparser "github.com/tomruk/socket.io-go/engine.io/parser"
)

type Namespace struct {
	name    string
	sockets *NamespaceSocketStore
	adapter Adapter
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
func (s *NamespaceSocketStore) Packet(sid string, packets ...*eioparser.Packet) (ok bool) {
	socket, ok := s.Get(sid)
	if !ok {
		return false
	}
	socket.packet(packets...)
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

func (n *Namespace) SocketStore() *NamespaceSocketStore {
	return n.sockets
}

func (n *Namespace) Sockets() []Socket {
	return n.sockets.GetAll()
}

func (n *Namespace) Emit(v ...interface{}) {

}
