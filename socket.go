package sio

import mapset "github.com/deckarep/golang-set/v2"

type Socket interface {
	// Socket ID. For client socket, this may return an empty string if the client hasn't connected yet.
	ID() SocketID

	// Is the socket (currently) connected?
	Connected() bool

	// Whether the connection state was recovered after a
	// temporary disconnection. In that case, any missed packets
	// will be transmitted, the data attribute
	// and the rooms will be restored.
	Recovered() bool

	// Register an event handler.
	On(eventName string, handler interface{})

	// Register a one-time event handler.
	// The handler will run once and will be removed afterwards.
	Once(eventName string, handler interface{})

	// Remove an event handler.
	//
	// If you want to remove all handlers of a particular event,
	// provide the eventName and leave the handler nil.
	//
	// Otherwise, provide both the eventName and handler arguments.
	Off(eventName string, handler interface{})

	// Remove all event handlers.
	// Including special event handlers such as: connect, connect_error, disconnect, disconnecting.
	OffAll()

	// Emit a message.
	// If you want to emit a binary data, use sio.Binary instead of []byte.
	Emit(eventName string, v ...interface{})
}

type ServerSocket interface {
	Socket

	// Retrieves the underlying Server.
	Server() *Server

	// Retrieves the Namespace this socket is connected to.
	Namespace() *Namespace

	// Register a middleware for events.
	//
	// Function signature must be same as with On and Once:
	// func(eventName string, v ...interface{}) error
	Use(f interface{})

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

type ClientSocket interface {
	Socket

	// Whether the socket will try to reconnect when its Client (manager) connects or reconnects.
	Active() bool

	// Connect the socket.
	Connect()

	// Disconnect the socket (a DISCONNECT packet will be sent).
	Disconnect()

	// Retrieves the Manager.
	Manager() *Manager

	// Setting the authentication data is optional and if used, it must be a JSON object (struct or map).
	// Non-JSON-object authentication data will not accepted, and panic will occur.
	SetAuth(v any)
	// Get the authentication data that was set by `SetAuth`.
	// As you might have guessed, returns nil if authentication data was not set before.
	Auth() (v any)
}
