package sio

import (
	"time"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/tomruk/socket.io-go/parser"
)

type AdapterCreator func(namespace *Namespace, socketStore *NamespaceSocketStore, parserCreator parser.Creator) Adapter

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

	Broadcast(header *parser.PacketHeader, v []interface{}, opts *BroadcastOptions)

	// The return value 'sids' is a thread safe mapset.Set.
	Sockets(rooms mapset.Set[Room]) (sids mapset.Set[SocketID])
	// The return value 'rooms' is a thread safe mapset.Set.
	SocketRooms(sid SocketID) (rooms mapset.Set[Room], ok bool)

	FetchSockets(opts *BroadcastOptions) (sockets []AdapterSocket)

	AddSockets(opts *BroadcastOptions, rooms ...Room)
	DelSockets(opts *BroadcastOptions, rooms ...Room)
	DisconnectSockets(opts *BroadcastOptions, close bool)

	ServerSideEmit(header *parser.PacketHeader, v []interface{})

	// Save the client session in order to restore it upon reconnection.
	PersistSession(session *SessionToPersist)

	// Restore the session and find the packets that were missed by the client.
	//
	// Returns nil if there is no session or session has expired.
	RestoreSession(pid PrivateSessionID, offset string) *SessionToPersist
}

// This is the equivalent of RemoteSocket
//
// See: https://github.com/socketio/socket.io/blob/7952312911e439f1e794760b50054565ece72845/lib/broadcast-operator.ts#L471
type AdapterSocket interface {
	ID() SocketID

	// Join room(s)
	Join(room ...Room)
	// Leave a room
	Leave(room Room)

	// Emit a message.
	// If you want to emit a binary data, use sio.Binary instead of []byte.
	Emit(eventName string, v ...interface{})

	// Sets a modifier for a subsequent event emission that the event
	// will only be broadcast to clients that have joined the given room.
	//
	// To emit to multiple rooms, you can call To several times.
	To(room ...Room) *BroadcastOperator

	// Alias of To(...)
	In(room ...Room) *BroadcastOperator

	// Sets a modifier for a subsequent event emission that the event
	// will only be broadcast to clients that have not joined the given rooms.
	Except(room ...Room) *BroadcastOperator

	// Sets a modifier for a subsequent event emission that
	// the event data will only be broadcast to every sockets but the sender.
	Broadcast() *BroadcastOperator

	// Disconnect from namespace.
	//
	// If `close` is true, all namespaces are going to be disconnected (a DISCONNECT packet will be sent),
	// and the underlying Engine.IO connection will be terminated.
	//
	// If `close` is false, only the current namespace will be disconnected (a DISCONNECT packet will be sent),
	// and the underlying Engine.IO connection will be kept open.
	Disconnect(close bool)
}

// A private ID, sent by the server at the beginning of
// the Socket.IO session and used for connection state recovery upon reconnection.
type PrivateSessionID string

type SessionToPersist struct {
	SID SocketID
	PID PrivateSessionID

	Rooms []Room

	MissedPackets []*PersistedPacket
}

type PersistedPacket struct {
	ID        string
	EmittedAt time.Time
	Opts      *BroadcastOptions

	Header *parser.PacketHeader
	Data   []interface{}
}

func (p *PersistedPacket) HasExpired(maxDisconnectDuration time.Duration) bool {
	return time.Now().Before(p.EmittedAt.Add(maxDisconnectDuration))
}
