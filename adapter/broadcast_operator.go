package adapter

import mapset "github.com/deckarep/golang-set/v2"

type BroadcastOptions struct {
	Rooms  mapset.Set[Room]
	Except mapset.Set[Room]
	Flags  BroadcastFlags
}

type BroadcastFlags struct {
	// This flag is unused at the moment, but for compatibility with the socket.io API, it stays here.
	Compress bool

	Local bool
}

func NewBroadcastOptions() *BroadcastOptions {
	return &BroadcastOptions{
		Rooms:  mapset.NewSet[Room](),
		Except: mapset.NewSet[Room](),
	}
}

type BroadcastOperator interface {
	// Emits an event to all choosen clients.
	Emit(eventName string, _v ...any)

	// Sets a modifier for a subsequent event emission that the event
	// will only be broadcast to clients that have joined the given room.
	//
	// To emit to multiple rooms, you can call To several times.
	To(room ...Room) BroadcastOperator

	// Alias of To(...)
	In(room ...Room) BroadcastOperator

	// Sets a modifier for a subsequent event emission that the event
	// will only be broadcast to clients that have not joined the given rooms.
	Except(room ...Room) BroadcastOperator

	// Compression flag is unused at the moment, thus setting this will have no effect on compression.
	Compress(compress bool) BroadcastOperator

	// Sets a modifier for a subsequent event emission that the event data will only be broadcast to the current node (when scaling to multiple nodes).
	//
	// See: https://socket.io/docs/v4/using-multiple-nodes
	Local() BroadcastOperator

	// Returns the matching socket instances. This method works across a cluster of several Socket.IO servers.
	FetchSockets() []Socket

	// Makes the matching socket instances join the specified rooms.
	SocketsJoin(room ...Room)

	// Makes the matching socket instances leave the specified rooms.
	SocketsLeave(room ...Room)

	// Makes the matching socket instances disconnect from the namespace.
	//
	// If value of close is true, closes the underlying connection. Otherwise, it just disconnects the namespace.
	DisconnectSockets(close bool)
}
