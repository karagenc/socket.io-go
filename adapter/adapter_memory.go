package adapter

import (
	"fmt"

	"github.com/karagenc/socket.io-go/internal/sync"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/karagenc/socket.io-go/parser"
)

// This is the equivalent of the default in-memory adapter of Socket.IO.
// Have a look at: https://github.com/socketio/socket.io-adapter
type inMemoryAdapter struct {
	mu    sync.Mutex
	rooms map[Room]mapset.Set[SocketID]
	sids  map[SocketID]mapset.Set[Room]

	sockets SocketStore

	parser parser.Parser
}

func NewInMemoryAdapterCreator() Creator {
	return func(socketStore SocketStore, parserCreator parser.Creator) Adapter {
		return &inMemoryAdapter{
			rooms:   make(map[Room]mapset.Set[SocketID]),
			sids:    make(map[SocketID]mapset.Set[Room]),
			sockets: socketStore,
			parser:  parserCreator(),
		}
	}
}

func (a *inMemoryAdapter) ServerCount() int { return 1 }

func (a *inMemoryAdapter) Close() {}

func (a *inMemoryAdapter) AddAll(sid SocketID, rooms []Room) {
	a.mu.Lock()
	defer a.mu.Unlock()

	_, ok := a.sids[sid]
	if !ok {
		a.sids[sid] = mapset.NewThreadUnsafeSet[Room]()
	}

	for _, room := range rooms {
		s := a.sids[sid]
		s.Add(room)

		r, ok := a.rooms[room]
		if !ok {
			r = mapset.NewThreadUnsafeSet[SocketID]()
			a.rooms[room] = r
		}
		if !r.Contains(sid) {
			r.Add(sid)
		}
	}
}

func (a *inMemoryAdapter) Delete(sid SocketID, room Room) {
	a.mu.Lock()
	defer a.mu.Unlock()

	s, ok := a.sids[sid]
	if ok {
		s.Remove(room)
	}

	a.delete(sid, room)
}

func (a *inMemoryAdapter) delete(sid SocketID, room Room) {
	r, ok := a.rooms[room]
	if ok {
		r.Remove(sid)
		if r.Cardinality() == 0 {
			delete(a.rooms, room)
		}
	}
}

func (a *inMemoryAdapter) DeleteAll(sid SocketID) {
	a.mu.Lock()
	defer a.mu.Unlock()

	s, ok := a.sids[sid]
	if !ok {
		return
	}

	s.Each(func(room Room) bool {
		a.delete(sid, room)
		return false
	})

	delete(a.sids, sid)
}

func (a *inMemoryAdapter) Broadcast(header *parser.PacketHeader, v []any, opts *BroadcastOptions) {
	buffers, err := a.parser.Encode(header, &v)
	if err != nil {
		panic(fmt.Errorf("sio: %w", err))
	}

	a.apply(opts, func(socket Socket) {
		a.sockets.SendBuffers(socket.ID(), buffers)
	})
}

// The return value 'sids' must be a thread safe mapset.Set.
func (a *inMemoryAdapter) Sockets(rooms mapset.Set[Room]) (sids mapset.Set[SocketID]) {
	a.mu.Lock()
	sids = mapset.NewSet[SocketID]()
	opts := NewBroadcastOptions()
	opts.Rooms = rooms
	a.mu.Unlock()

	a.apply(opts, func(socket Socket) {
		sids.Add(socket.ID())
	})
	return
}

// The return value 'rooms' must be a thread safe mapset.Set.
func (a *inMemoryAdapter) SocketRooms(sid SocketID) (rooms mapset.Set[Room], ok bool) {
	a.mu.Lock()
	defer a.mu.Unlock()

	s, ok := a.sids[sid]
	if !ok {
		return nil, false
	}

	rooms = mapset.NewSet[Room]()
	s.Each(func(room Room) bool {
		rooms.Add(room)
		return false
	})
	return
}

func (a *inMemoryAdapter) FetchSockets(opts *BroadcastOptions) (sockets []Socket) {
	a.apply(opts, func(socket Socket) {
		sockets = append(sockets, socket)
	})
	return
}

func (a *inMemoryAdapter) AddSockets(opts *BroadcastOptions, rooms ...Room) {
	a.apply(opts, func(socket Socket) {
		socket.Join(rooms...)
	})
}

func (a *inMemoryAdapter) DelSockets(opts *BroadcastOptions, rooms ...Room) {
	a.apply(opts, func(socket Socket) {
		for _, room := range rooms {
			socket.Leave(room)
		}
	})
}

func (a *inMemoryAdapter) DisconnectSockets(opts *BroadcastOptions, close bool) {
	a.apply(opts, func(socket Socket) {
		socket.Disconnect(close)
	})
}

func (a *inMemoryAdapter) apply(opts *BroadcastOptions, callback func(socket Socket)) {
	a.mu.Lock()

	exceptSids := a.computeExceptSids(opts.Except)

	// If a room was specificed in opts.Rooms,
	// we only use sockets in those rooms.
	// Otherwise (within else), any socket will be used.
	if opts.Rooms.Cardinality() > 0 {
		ids := mapset.NewThreadUnsafeSet[SocketID]()
		opts.Rooms.Each(func(room Room) bool {
			r, ok := a.rooms[room]
			if !ok {
				return false
			}

			r.Each(func(sid SocketID) bool {
				if ids.Contains(sid) || exceptSids.Contains(sid) {
					return false
				}
				socket, ok := a.sockets.Get(sid)
				if ok {
					a.mu.Unlock()
					callback(socket)
					a.mu.Lock()
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
				a.mu.Unlock()
				callback(socket)
				a.mu.Lock()
			}
		}
	}
	a.mu.Unlock()
}

// Beware that the return value 'exceptSids' is thread unsafe.
func (a *inMemoryAdapter) computeExceptSids(exceptRooms mapset.Set[Room]) (exceptSids mapset.Set[SocketID]) {
	exceptSids = mapset.NewThreadUnsafeSet[SocketID]()

	if exceptRooms.Cardinality() > 0 {
		exceptRooms.Each(func(room Room) bool {
			r, ok := a.rooms[room]
			if ok {
				r.Each(func(sid SocketID) bool {
					exceptSids.Add(sid)
					return false
				})
			}
			return false
		})
	}
	return
}

func (a *inMemoryAdapter) ServerSideEmit(header *parser.PacketHeader, v []any) {}

func (a *inMemoryAdapter) PersistSession(session *SessionToPersist) {}

func (a *inMemoryAdapter) RestoreSession(pid PrivateSessionID, offset string) (*SessionToPersist, bool) {
	return nil, false
}
