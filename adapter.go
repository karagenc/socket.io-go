package sio

import "sync"

type AdapterCreator func(namespace *Namespace) Adapter

type Adapter interface {
	Close()

	AddAll(sid string, rooms []string)
	Delete(sid string, room string)
	DeleteAll(sid string)

	Broadcast()

	Sockets(rooms []string) (sids []string)
	SocketRooms()
	FetchSockets()
	AddSockets()
	DelSockets()

	DisconnectSockets()
	ComputeExceptSIDs()
}

// This is the default in-memory adapter of Socket.IO.
// Have a look at: https://github.com/socketio/socket.io-adapter
type inMemoryAdapter struct {
	mu        sync.Mutex
	namespace *Namespace
	rooms     stringMapStringSlice
	sids      stringMapStringSlice
}

// This is the equivalent of the container type as defined in socket.io-adapter:
//
// public rooms: Map<Room, Set<SocketId>> = new Map();
//
// public sids: Map<SocketId, Set<Room>> = new Map();
//
type stringMapStringSlice map[string][]string

func (m stringMapStringSlice) Has(key string) bool {
	_, ok := m[key]
	return ok
}

func (m stringMapStringSlice) Get(key string) []string {
	s, _ := m[key]
	return s
}

func (m stringMapStringSlice) Set(key string, s []string) {
	m[key] = s
}

func (m stringMapStringSlice) Add(key, value string) (alreadyExists bool) {
	arr, _ := m[key]
	for _, a := range arr {
		if a == value {
			return true
		}
	}
	arr = append(arr, value)
	m[key] = arr
	return false
}

func (m stringMapStringSlice) Delete(key string) {
	delete(m, key)
}

func (m stringMapStringSlice) DeleteItem(key, value string) (deleted bool) {
	values, _ := m[key]

	remove := func(slice []string, s int) []string {
		return append(slice[:s], slice[s+1:]...)
	}

	for i, v := range values {
		if v == value {
			values = remove(values, i)
			deleted = true
			break
		}
	}

	m[key] = values
	return
}

func newInMemoryAdapter(namespace *Namespace) Adapter {
	return &inMemoryAdapter{
		namespace: namespace,
		rooms:     make(stringMapStringSlice),
		sids:      make(stringMapStringSlice),
	}
}

func (a *inMemoryAdapter) Close() {}

func (a *inMemoryAdapter) AddAll(sid string, rooms []string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	for _, room := range rooms {
		a.sids.Add(sid, room)

		if !a.rooms.Has(room) {
			a.rooms.Set(room, nil)
			// TODO: this.emit("create-room", room);
		}

		alreadyExists := a.rooms.Add(room, sid)
		if !alreadyExists {
			// TODO: this.emit("join-room", room);
		}
	}
}

func (a *inMemoryAdapter) Delete(sid string, room string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.sids.Has(sid) {
		a.sids.DeleteItem(sid, room)
	}

	a.delete(sid, room)
}

func (a *inMemoryAdapter) delete(sid string, room string) {
	if a.rooms.Has(room) {
		deleted := a.rooms.DeleteItem(room, sid)
		if deleted {
			// TODO: this.emit("leave-room", room, id);
		}
		sids := a.rooms.Get(room)
		if len(sids) == 0 {
			a.rooms.Delete(room)
			// TODO: this.emit("delete-room", room);
		}
	}
}

func (a *inMemoryAdapter) DeleteAll(sid string) {
	if !a.sids.Has(sid) {
		return
	}

	rooms := a.sids.Get(sid)
	for _, room := range rooms {
		a.delete(sid, room)
	}

	a.sids.Delete(sid)
}

func (a *inMemoryAdapter) Sockets(rooms []string) (sids []string) {
	if len(rooms) > 0 {
		for _, room := range rooms {
			if !a.rooms.Has(room) {
				continue
			}

			sids := a.rooms.Get(room)
			for _, sid := range sids {
				if a.namespace.sockets.has(id) {
					sids.add(id)
				}
			}
		}
	} else {

	}

	return
}
