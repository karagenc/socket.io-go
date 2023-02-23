package sio

import "sync"

type serverSocketStore struct {
	sockets map[string]*serverSocket
	mu      sync.Mutex
}

func newServerSocketStore() *serverSocketStore {
	return &serverSocketStore{
		sockets: make(map[string]*serverSocket),
	}
}

func (s *serverSocketStore) Get(sid string) (socket *serverSocket, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	socket, ok = s.sockets[sid]
	return
}

func (s *serverSocketStore) GetAll() (sockets []*serverSocket) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sockets = make([]*serverSocket, len(s.sockets))
	i := 0
	for _, socket := range s.sockets {
		sockets[i] = socket
		i++
	}
	return
}

func (s *serverSocketStore) GetAndRemoveAll() (sockets []*serverSocket) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sockets = make([]*serverSocket, len(s.sockets))
	i := 0
	for _, socket := range s.sockets {
		sockets[i] = socket
		i++
	}
	s.sockets = make(map[string]*serverSocket)
	return
}

func (s *serverSocketStore) Set(socket *serverSocket) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sockets[socket.ID()] = socket
}

func (s *serverSocketStore) Remove(sid string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sockets, sid)
}
