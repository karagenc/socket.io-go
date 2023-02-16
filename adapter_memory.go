package sio

import (
	"sync"

	mapset "github.com/deckarep/golang-set/v2"
)

// This is the equivalent of the default in-memory adapter of Socket.IO.
// Have a look at: https://github.com/socketio/socket.io-adapter
type inMemoryAdapter struct {
	mu    sync.Mutex
	rooms map[string]mapset.Set[string]
	sids  map[string]mapset.Set[string]

	namespace *Namespace
	sockets   *NamespaceSocketStore
}

func newInMemoryAdapter(namespace *Namespace, socketStore *NamespaceSocketStore) Adapter {
	return &inMemoryAdapter{
		rooms:     make(map[string]mapset.Set[string]),
		sids:      make(map[string]mapset.Set[string]),
		namespace: namespace,
		sockets:   socketStore,
	}
}

func (a *inMemoryAdapter) Close() {}

func (a *inMemoryAdapter) AddAll(sid string, rooms []string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	for _, room := range rooms {
		s, ok := a.sids[sid]
		if ok {
			s.Add(room)
		}

		r, ok := a.rooms[room]
		if !ok {
			r = mapset.NewThreadUnsafeSet[string]()
			a.rooms[room] = r
		}
		if !r.Contains(sid) {
			r.Add(sid)
		}
	}
}

func (a *inMemoryAdapter) Delete(sid string, room string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	s, ok := a.sids[sid]
	if ok {
		s.Remove(room)
	}

	a.delete(sid, room)
}

func (a *inMemoryAdapter) delete(sid string, room string) {
	r, ok := a.rooms[room]
	if ok {
		r.Remove(sid)
		if r.Cardinality() == 0 {
			delete(a.rooms, room)
		}
	}
}

func (a *inMemoryAdapter) DeleteAll(sid string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	s, ok := a.sids[sid]
	if !ok {
		return
	}

	s.Each(func(room string) bool {
		a.delete(sid, room)
		return false
	})

	delete(a.sids, sid)
}

func (a *inMemoryAdapter) Broadcast(buffers [][]byte, opts *BroadcastOptions) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.apply(opts, func(socket *serverSocket) {
		a.sockets.SendBuffers(socket.ID(), buffers)
	})
}

func (a *inMemoryAdapter) BroadcastWithAck(packetID string, buffers [][]byte, opts *BroadcastOptions, ackHandler *ackHandler) {
	a.apply(opts, func(socket *serverSocket) {
		a.sockets.SetAck(socket.ID(), ackHandler)
		a.sockets.SendBuffers(socket.ID(), buffers)
	})
}

// The return value 'sids' must be a thread safe mapset.Set.
func (a *inMemoryAdapter) Sockets(rooms mapset.Set[string]) (sids mapset.Set[string]) {
	a.mu.Lock()
	defer a.mu.Unlock()

	sids = mapset.NewSet[string]()
	opts := NewBroadcastOptions()
	opts.Rooms = rooms

	a.apply(opts, func(socket *serverSocket) {
		sids.Add(socket.ID())
	})
	return
}

// The return value 'rooms' must be a thread safe mapset.Set.
func (a *inMemoryAdapter) SocketRooms(sid string) (rooms mapset.Set[string], ok bool) {
	a.mu.Lock()
	defer a.mu.Unlock()

	s, ok := a.sids[sid]
	if !ok {
		return nil, false
	}

	rooms = mapset.NewSet[string]()
	s.Each(func(room string) bool {
		rooms.Add(room)
		return false
	})
	return
}

func (a *inMemoryAdapter) FetchSockets(opts *BroadcastOptions) (sockets []*serverSocket) {
	a.apply(opts, func(socket *serverSocket) {
		sockets = append(sockets, socket)
	})
	return
}

func (a *inMemoryAdapter) AddSockets(opts *BroadcastOptions, rooms ...string) {
	a.apply(opts, func(socket *serverSocket) {
		socket.Join(rooms...)
	})
}

func (a *inMemoryAdapter) DelSockets(opts *BroadcastOptions, rooms ...string) {
	a.apply(opts, func(socket *serverSocket) {
		for _, room := range rooms {
			socket.Leave(room)
		}
	})
}

func (a *inMemoryAdapter) DisconnectSockets(opts *BroadcastOptions, close bool) {
	a.apply(opts, func(socket *serverSocket) {
		socket.Disconnect(close)
	})
}

func (a *inMemoryAdapter) apply(opts *BroadcastOptions, callback func(socket *serverSocket)) {
	a.mu.Lock()
	defer a.mu.Unlock()

	exceptSids := a.computeExceptSids(opts.Except)

	// If a room was specificed in opts.Rooms,
	// we only use sockets in those rooms.
	// Otherwise (within else), any socket will be used.
	if opts.Rooms.Cardinality() > 0 {
		ids := mapset.NewThreadUnsafeSet[string]()
		opts.Rooms.Each(func(room string) bool {
			r, ok := a.rooms[room]
			if !ok {
				return false
			}

			r.Each(func(sid string) bool {
				if ids.Contains(sid) || exceptSids.Contains(sid) {
					return false
				}
				socket, ok := a.sockets.Get(sid)
				if ok {
					callback(socket)
					ids.Add(sid)
				}
				return false
			})
			return false
		})
	} else {
		for sid := range a.sids {
			if exceptSids.Contains(sid) {
				continue
			}
			socket, ok := a.sockets.Get(sid)
			if ok {
				callback(socket)
			}
		}
	}
}

// Beware that the return value 'exceptSids' is thread unsafe.
func (a *inMemoryAdapter) computeExceptSids(exceptRooms mapset.Set[string]) (exceptSids mapset.Set[string]) {
	exceptSids = mapset.NewThreadUnsafeSet[string]()
	if exceptRooms.Cardinality() > 0 {
		exceptRooms.Each(func(room string) bool {
			r, ok := a.rooms[room]
			if ok {
				r.Each(func(sid string) bool {
					exceptSids.Add(sid)
					return false
				})
			}
			return false
		})
	}
	return
}
