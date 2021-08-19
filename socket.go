package sio

type Socket interface {
	// Socket ID. For client socket, this can be empty if the client hasn't connected yet.
	ID() string

	// Client only.
	Connect()

	// Client only.
	//
	// Retrieves the underlying Client.
	//
	// It is called Manager in official implementation of Socket.IO: https://github.com/socketio/socket.io-client/blob/4.1.3/lib/manager.ts#L295
	IO() *Client

	// Client only.
	//
	// This is a concurrenct storage that stores the authentication data.
	//
	// Setting the authentication data is optional and if used, it must be a JSON object (struct or map).
	// Non-JSON-object authentication data is not accepted by Socket.IO.
	Auth() *Auth

	OnConnect(handler ConnectCallback)

	OnceConnect(handler ConnectCallback)

	OffConnect(handler ConnectCallback)

	// Client only.
	OnConnectError(handler ConnectErrorCallback)

	// Client only.
	OnceConnectError(handler ConnectErrorCallback)

	// Client only.
	OffConnectError(handler ConnectErrorCallback)

	OnDisconnect(handler DisconnectCallback)

	OnceDisconnect(handler DisconnectCallback)

	OffDisconnect(handler DisconnectCallback)

	// Server only.
	OnDisconnecting(handler DisconnectingCallback)

	// Server only.
	OnceDisconnecting(handler DisconnectingCallback)

	// Server only.
	OffDisconnecting(handler DisconnectingCallback)

	// Register an event handler.
	OnEvent(eventName string, handler interface{})

	// Register a one-time event handler.
	// The handler will run once and will be removed afterwards.
	OnceEvent(eventName string, handler interface{})

	// Remove an event handler.
	//
	// If you want to remove all handlers of a particular event,
	// provide the eventName and leave the handler nil.
	//
	// Otherwise, provide both the eventName and handler arguments.
	OffEvent(eventName string, handler interface{})

	// Remove all event handlers.
	// Including special event handlers such as: connect, connect_error, disconnect, disconnecting.
	OffAll()

	// Emit a message.
	// If you want to emit a binary data, use sio.Binary instead of []byte.
	Emit(v ...interface{})
}
