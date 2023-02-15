package sio

import (
	mapset "github.com/deckarep/golang-set/v2"
)

type AdapterCreator func(namespace *Namespace) Adapter

type Adapter interface {
	Close()

	AddAll(sid string, rooms []string)
	Delete(sid string, room string)
	DeleteAll(sid string)

	Broadcast(buffers [][]byte, opts *BroadcastOptions)

	// The return value 'sids' is a thread safe mapset.Set.
	Sockets(rooms mapset.Set[string]) (sids mapset.Set[string])
	// The return value 'rooms' is a thread safe mapset.Set.
	SocketRooms(sid string) (rooms mapset.Set[string], ok bool)

	AddSockets(opts *BroadcastOptions, rooms ...string)
	DelSockets(opts *BroadcastOptions, rooms ...string)
	DisconnectSockets(opts *BroadcastOptions, close bool)
}
