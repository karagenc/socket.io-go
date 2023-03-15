package adapter

type Socket interface {
	ID() SocketID

	// Join room(s)
	Join(room ...Room)
	// Leave a room
	Leave(room Room)

	// Emit a message.
	// If you want to emit a binary data, use sio.Binary instead of []byte.
	Emit(eventName string, v ...any)

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
