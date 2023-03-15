package sio

import mapset "github.com/deckarep/golang-set/v2"

type ServerSocket interface {
	Socket
	ServerSocketEvents

	// Retrieves the underlying Server.
	Server() *Server

	// Retrieves the Namespace this socket is connected to.
	Namespace() *Namespace

	// Register a middleware for events.
	//
	// Function signature must be same as with On and Once:
	// func(eventName string, v ...any) error
	Use(f any)

	// Join room(s)
	Join(room ...Room)
	// Leave a room
	Leave(room Room)
	// Get a set of all rooms socket was joined to.
	Rooms() mapset.Set[Room]

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
	// the event data will only be broadcast to the current node.
	Local() *BroadcastOperator

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

type ServerSocketEvents interface {
	OnError(f ServerSocketErrorFunc)

	OnceError(f ServerSocketErrorFunc)

	OffError(f ...ServerSocketErrorFunc)

	OnDisconnecting(f ServerSocketDisconnectingFunc)

	OnceDisconnecting(f ServerSocketDisconnectingFunc)

	OffDisconnecting(f ...ServerSocketDisconnectingFunc)

	OnDisconnect(f ServerSocketDisconnectFunc)

	OnceDisconnect(f ServerSocketDisconnectFunc)

	OffDisconnect(f ...ServerSocketDisconnectFunc)
}
