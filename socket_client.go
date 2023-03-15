package sio

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

	Volatile() Emitter
}

type ClientEvents interface {
	OnConnect(f ClientSocketConnectFunc)

	OnceConnect(f ClientSocketConnectFunc)

	OffConnect(f ...ClientSocketConnectFunc)

	OnConnectError(f ClientSocketConnectErrorFunc)

	OnceConnectError(f ClientSocketConnectErrorFunc)

	OffConnectError(f ...ClientSocketConnectErrorFunc)

	OnDisconnect(f ClientSocketDisconnectFunc)

	OnceDisconnect(f ClientSocketDisconnectFunc)

	OffDisconnect(f ...ClientSocketDisconnectFunc)
}
