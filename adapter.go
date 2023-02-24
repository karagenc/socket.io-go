package sio

import mapset "github.com/deckarep/golang-set/v2"

type AdapterCreator func(namespace *Namespace, socketStore *NamespaceSocketStore) Adapter

// A public ID, sent by the server at the beginning of
// the Socket.IO session and which can be used for private messaging.
type SocketID string

// A private ID, sent by the server at the beginning of
// the Socket.IO session and used for connection state recovery upon reconnection.
type PrivateSessionID string

type Room string

type Adapter interface {
	Close()

	AddAll(sid SocketID, rooms []Room)
	Delete(sid SocketID, room Room)
	DeleteAll(sid SocketID)

	Broadcast(buffers [][]byte, opts *BroadcastOptions)
	BroadcastWithAck(packetID string, buffers [][]byte, opts *BroadcastOptions, ackHandler *ackHandler)

	// The return value 'sids' is a thread safe mapset.Set.
	Sockets(rooms mapset.Set[Room]) (sids mapset.Set[SocketID])
	// The return value 'rooms' is a thread safe mapset.Set.
	SocketRooms(sid SocketID) (rooms mapset.Set[Room], ok bool)

	FetchSockets(opts *BroadcastOptions) (sockets []AdapterSocket)

	AddSockets(opts *BroadcastOptions, rooms ...Room)
	DelSockets(opts *BroadcastOptions, rooms ...Room)
	DisconnectSockets(opts *BroadcastOptions, close bool)
}

type AdapterSocket interface {
	ServerSocket
}
