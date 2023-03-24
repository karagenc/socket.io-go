package adapter

import (
	"sync"
)

type TestSocketStore struct {
	sockets     map[SocketID]Socket
	mu          sync.Mutex
	sendBuffers func(sid SocketID, buffers [][]byte) (ok bool)
}

var _ SocketStore = NewTestSocketStore()

func NewTestSocketStore() *TestSocketStore {
	return &TestSocketStore{
		sockets:     make(map[SocketID]Socket),
		sendBuffers: func(sid SocketID, buffers [][]byte) (ok bool) { return true },
	}
}

func (s *TestSocketStore) SendBuffers(sid SocketID, buffers [][]byte) (ok bool) {
	return s.sendBuffers(sid, buffers)
}

func (s *TestSocketStore) SetSendBuffers(sendBuffers func(sid SocketID, buffers [][]byte) (ok bool)) {
	s.sendBuffers = sendBuffers
}

func (s *TestSocketStore) Get(sid SocketID) (so Socket, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	so, ok = s.sockets[sid]
	return so, ok
}

func (s *TestSocketStore) GetAll() []Socket {
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

func (s *TestSocketStore) Set(so Socket) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sockets[so.ID()] = so
}

func (s *TestSocketStore) Remove(sid SocketID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sockets, sid)
}

type TestSocket struct {
	id SocketID

	Rooms     []Room
	Connected bool
}

var _ Socket = NewTestSocket("")

func NewTestSocket(id SocketID) *TestSocket {
	return &TestSocket{
		id:        id,
		Connected: true,
		Rooms:     []Room{Room(id)},
	}
}

func (s *TestSocket) ID() SocketID { return s.id }

func (s *TestSocket) Join(room ...Room) {
	s.Rooms = append(s.Rooms, room...)
}

func (s *TestSocket) Leave(room Room) {
	remove := func(slice []Room, s int) []Room {
		return append(slice[:s], slice[s+1:]...)
	}
	for i, r := range s.Rooms {
		if r == room {
			s.Rooms = remove(s.Rooms, i)
		}
	}
}

func (s *TestSocket) Emit(eventName string, v ...any) {}

func (s *TestSocket) To(room ...Room) *BroadcastOperator { return nil }

func (s *TestSocket) In(room ...Room) *BroadcastOperator { return nil }

func (s *TestSocket) Except(room ...Room) *BroadcastOperator { return nil }

func (s *TestSocket) Broadcast() *BroadcastOperator { return nil }

func (s *TestSocket) Disconnect(close bool) {
	s.Connected = false
}
