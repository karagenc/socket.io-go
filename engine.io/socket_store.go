package eio

import (
	"sync"
)

type socketStore struct {
	sockets map[string]*serverSocket
	mu      sync.RWMutex
}

func newSocketStore() *socketStore {
	return &socketStore{
		sockets: make(map[string]*serverSocket),
	}
}

func (s *socketStore) Get(sid string) (socket *serverSocket, ok bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	socket, ok = s.sockets[sid]
	return
}

func (s *socketStore) Set(sid string, socket *serverSocket) (ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, exists := s.sockets[sid]
	if exists {
		return false
	}

	s.sockets[sid] = socket
	return true
}

func (s *socketStore) Delete(sid string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sockets, sid)
}

func (s *socketStore) Exists(sid string) (exists bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, exists = s.sockets[sid]
	return
}

func (s *socketStore) GetAll() (sockets []*serverSocket) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, socket := range s.sockets {
		sockets = append(sockets, socket)
	}
	return
}

func (s *socketStore) CloseAll() {
	sockets := s.GetAll()
	for _, socket := range sockets {
		socket.Close()
	}
}
