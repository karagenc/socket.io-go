package sio

import "sync"

type clientSocketStore struct {
	sockets map[string]*clientSocket
	mu      sync.Mutex
}

func newClientSocketStore() *clientSocketStore {
	return &clientSocketStore{
		sockets: make(map[string]*clientSocket),
	}
}

func (s *clientSocketStore) Get(namespace string) (ss *clientSocket, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ss, ok = s.sockets[namespace]
	return
}

func (s *clientSocketStore) GetAll() (sockets []*clientSocket) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sockets = make([]*clientSocket, len(s.sockets))
	i := 0
	for _, ss := range s.sockets {
		sockets[i] = ss
		i++
	}
	return
}

func (s *clientSocketStore) Set(ss *clientSocket) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sockets[ss.namespace] = ss
}

func (s *clientSocketStore) Remove(namespace string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sockets, namespace)
}

func (s *clientSocketStore) DisconnectAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, socket := range s.sockets {
		socket.Disconnect()
	}
}
