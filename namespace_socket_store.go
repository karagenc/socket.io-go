package sio

import "sync"

type NamespaceSocketStore struct {
	sockets map[SocketID]*serverSocket
	mu      sync.Mutex
}

func newNamespaceSocketStore() *NamespaceSocketStore {
	return &NamespaceSocketStore{
		sockets: make(map[SocketID]*serverSocket),
	}
}

func (s *NamespaceSocketStore) Get(sid SocketID) (so *serverSocket, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	so, ok = s.sockets[sid]
	return so, ok
}

// Send Engine.IO packets to a specific socket.
func (s *NamespaceSocketStore) SendBuffers(sid SocketID, buffers [][]byte) (ok bool) {
	socket, ok := s.Get(sid)
	if !ok {
		return false
	}
	socket.conn.sendBuffers(buffers...)
	return true
}

func (s *NamespaceSocketStore) SetAck(sid SocketID, ackHandler *ackHandler) (ok bool) {
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

func (s *NamespaceSocketStore) Remove(sid SocketID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sockets, sid)
}
