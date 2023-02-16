package sio

type Socket interface {
	// Socket ID. For client socket, this may return an empty string if the client hasn't connected yet.
	ID() string

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

	Join(room ...string)
	Leave(room string)

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

	Connect()

	// Retrieves the underlying Client.
	//
	// It is called Manager in official implementation of Socket.IO: https://github.com/socketio/socket.io-client/blob/4.1.3/lib/manager.ts#L295
	Client() *Client

	// Setting the authentication data is optional and if used, it must be a JSON object (struct or map).
	// Non-JSON-object authentication data will not accepted, and panic will occur.
	SetAuth(v any)
	// Get the authentication data that was set by `SetAuth`.
	// As you might have guessed, returns nil if authentication data was not set before.
	Auth() (v any)

	// Disconnect the Socket (a DISCONNECT packet will be sent).
	Disconnect()
}
