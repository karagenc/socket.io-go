package sio

import "sync"

type NamespaceSocketStore struct {
	sockets map[SocketID]ServerSocket
	mu      sync.Mutex
}

func newNamespaceSocketStore() *NamespaceSocketStore {
	return &NamespaceSocketStore{
		sockets: make(map[SocketID]ServerSocket),
	}
}

func (s *NamespaceSocketStore) Get(sid SocketID) (so ServerSocket, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	so, ok = s.sockets[sid]
	return so, ok
}

// Send Engine.IO packets to a specific socket.
func (s *NamespaceSocketStore) SendBuffers(sid SocketID, buffers [][]byte) (ok bool) {
	_socket, ok := s.Get(sid)
	if !ok {
		return false
	}
	socket := _socket.(*serverSocket)
	socket.conn.sendBuffers(buffers...)
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

func (s *NamespaceSocketStore) Set(so ServerSocket) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sockets[so.ID()] = so
}

func (s *NamespaceSocketStore) Remove(sid SocketID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sockets, sid)
}
