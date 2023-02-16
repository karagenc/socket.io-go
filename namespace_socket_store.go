package sio

import "sync"

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

func (s *NamespaceSocketStore) SetAck(sid string, ackHandler *ackHandler) (ok bool) {
	socket, ok := s.Get(sid)
	if !ok {
		return false
	}
	socket.setAck(ackHandler)
	return true
}

func (s *NamespaceSocketStore) GetAll() []ServerSocket {
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
