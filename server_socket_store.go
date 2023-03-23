package sio

import "sync"

type serverSocketStore struct {
	socketsByID        map[SocketID]*serverSocket
	socketsByNamespace map[string]*serverSocket
	mu                 sync.Mutex
}

func newServerSocketStore() *serverSocketStore {
	return &serverSocketStore{
		socketsByID:        make(map[SocketID]*serverSocket),
		socketsByNamespace: make(map[string]*serverSocket),
	}
}

func (s *serverSocketStore) getByID(sid SocketID) (socket *serverSocket, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	socket, ok = s.socketsByID[sid]
	return
}

func (s *serverSocketStore) getByNsp(nsp string) (socket *serverSocket, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	socket, ok = s.socketsByNamespace[nsp]
	return
}

func (s *serverSocketStore) getAll() (sockets []*serverSocket) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sockets = make([]*serverSocket, len(s.socketsByID))
	i := 0
	for _, socket := range s.socketsByID {
		sockets[i] = socket
		i++
	}
	return
}

func (s *serverSocketStore) getAndRemoveAll() (sockets []*serverSocket) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sockets = make([]*serverSocket, len(s.socketsByID))
	i := 0
	for _, socket := range s.socketsByID {
		sockets[i] = socket
		i++
	}
	s.socketsByID = make(map[SocketID]*serverSocket)
	s.socketsByNamespace = make(map[string]*serverSocket)
	return
}

func (s *serverSocketStore) set(socket *serverSocket) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.socketsByID[socket.ID()] = socket
	s.socketsByNamespace[socket.nsp.Name()] = socket
}

func (s *serverSocketStore) removeByID(sid SocketID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	socket, ok := s.socketsByID[sid]
	if ok {
		delete(s.socketsByID, sid)
		delete(s.socketsByNamespace, socket.nsp.Name())
	}
}
