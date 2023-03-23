package sio

import (
	"sync"

	"github.com/tomruk/socket.io-go/adapter"
)

type namespaceSocketStore struct {
	sockets map[SocketID]ServerSocket
	mu      sync.Mutex
}

func newNamespaceSocketStore() *namespaceSocketStore {
	return &namespaceSocketStore{
		sockets: make(map[SocketID]ServerSocket),
	}
}

// Send Engine.IO packets to a specific socket.
func (s *namespaceSocketStore) sendBuffers(sid SocketID, buffers [][]byte) (ok bool) {
	_socket, ok := s.get(sid)
	if !ok {
		return false
	}
	socket := _socket.(*serverSocket)
	socket.conn.sendBuffers(buffers...)
	return true
}

func (s *namespaceSocketStore) get(sid SocketID) (so ServerSocket, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	so, ok = s.sockets[sid]
	return so, ok
}

func (s *namespaceSocketStore) getAll() []ServerSocket {
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

func (s *namespaceSocketStore) set(so ServerSocket) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sockets[so.ID()] = so
}

func (s *namespaceSocketStore) remove(sid SocketID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sockets, sid)
}

// This is to ensure we have a socket store with a
// right function signature that matches with adapter's
// `SocketStore`.
type adapterSocketStore struct {
	store *namespaceSocketStore
}

func newAdapterSocketStore(store *namespaceSocketStore) *adapterSocketStore {
	return &adapterSocketStore{
		store: store,
	}
}

// Send Engine.IO packets to a specific socket.
func (s *adapterSocketStore) SendBuffers(sid SocketID, buffers [][]byte) (ok bool) {
	return s.store.sendBuffers(sid, buffers)
}

func (s *adapterSocketStore) Get(sid SocketID) (so adapter.Socket, ok bool) {
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
