package sio

import (
	"sync"

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