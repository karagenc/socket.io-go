package sio

import (
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/tomruk/socket.io-go/parser"
)

type AdapterCreator func(namespace *Namespace, socketStore *NamespaceSocketStore) Adapter

// A public ID, sent by the server at the beginning of
// the Socket.IO session and which can be used for private messaging.
type SocketID string

type Room string

type Adapter interface {
	ServerCount() int
	Close()

	AddAll(sid SocketID, rooms []Room)
	Delete(sid SocketID, room Room)
	DeleteAll(sid SocketID)

	Broadcast(header *parser.PacketHeader, buffers [][]byte, opts *BroadcastOptions)

	BroadcastWithAck(packetID string, header *parser.PacketHeader, buffers [][]byte, opts *BroadcastOptions, ackHandler *ackHandler)

	// The return value 'sids' is a thread safe mapset.Set.
	Sockets(rooms mapset.Set[Room]) (sids mapset.Set[SocketID])
	// The return value 'rooms' is a thread safe mapset.Set.
	SocketRooms(sid SocketID) (rooms mapset.Set[Room], ok bool)

	FetchSockets(opts *BroadcastOptions) (sockets []AdapterSocket)

	AddSockets(opts *BroadcastOptions, rooms ...Room)
	DelSockets(opts *BroadcastOptions, rooms ...Room)
	DisconnectSockets(opts *BroadcastOptions, close bool)

	// Save the client session in order to restore it upon reconnection.
	// TODO: Keep this pointer or not?
	PersistSession(session *SessionToPersist)

	// Restore the session and find the packets that were missed by the client.
	//
	// Returns nil if there is no session or session has expired.
	RestoreSession(pid PrivateSessionID, offset string) *SessionToPersist
}

type AdapterSocket interface {
	ServerSocket
}

// A private ID, sent by the server at the beginning of
// the Socket.IO session and used for connection state recovery upon reconnection.
type PrivateSessionID string

type SessionToPersist struct {
	SID SocketID
	PID PrivateSessionID

	Rooms []Room

	Data [][][]byte
}
