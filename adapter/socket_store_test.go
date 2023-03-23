package adapter

import (
	"sync"
)

type testSocketStore struct {
	sockets     map[SocketID]Socket
	mu          sync.Mutex
	sendBuffers func(sid SocketID, buffers [][]byte) (ok bool)
}

var _ SocketStore = newTestSocketStore()

func newTestSocketStore() *testSocketStore {
	return &testSocketStore{
		sockets:     make(map[SocketID]Socket),
		sendBuffers: func(sid SocketID, buffers [][]byte) (ok bool) { return },
	}
}

func (s *testSocketStore) SendBuffers(sid SocketID, buffers [][]byte) (ok bool) {
	return s.sendBuffers(sid, buffers)
}

func (s *testSocketStore) Get(sid SocketID) (so Socket, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	so, ok = s.sockets[sid]
	return so, ok
}

func (s *testSocketStore) GetAll() []Socket {
	s.mu.Lock()
	defer s.mu.Unlock()

	sockets := make([]Socket, len(s.sockets))
	i := 0
	for _, s := range s.sockets {
		sockets[i] = s
		i++
	}
	return sockets
}

func (s *testSocketStore) Set(so Socket) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sockets[so.ID()] = so
}

func (s *testSocketStore) Remove(sid SocketID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sockets, sid)
}
