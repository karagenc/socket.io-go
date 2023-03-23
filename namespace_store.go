package sio

import (
	"sync"

	"github.com/tomruk/socket.io-go/adapter"
	"github.com/tomruk/socket.io-go/parser"
)

// We could've used mapset instead of this,
// but mapset doesn't have an equivalent of the GetOrCreate method.
type namespaceStore struct {
	nsps map[string]*Namespace
	mu   sync.Mutex
}

func newNamespaceStore() *namespaceStore {
	return &namespaceStore{
		nsps: make(map[string]*Namespace),
	}
}

func (s *namespaceStore) getOrCreate(
	name string,
	server *Server,
	adapterCreator adapter.Creator,
	parserCreator parser.Creator,
) (namespace *Namespace, created bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var ok bool
	namespace, ok = s.nsps[name]
	if !ok {
		namespace = newNamespace(name, server, adapterCreator, parserCreator)
		s.nsps[namespace.Name()] = namespace
		created = true
	}
	return
}

func (s *namespaceStore) len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.nsps)
}

func (s *namespaceStore) set(nsp *Namespace) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nsps[nsp.Name()] = nsp
}

func (s *namespaceStore) get(name string) (nsp *Namespace, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	nsp, ok = s.nsps[name]
	return
}

func (s *namespaceStore) remove(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.nsps, name)
}
