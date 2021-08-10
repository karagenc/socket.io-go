package sio

type Socket interface {
	// Socket ID. For client socket, this can be empty if the client hasn't connected yet.
	ID() string

	// Client only.
	Connect() error

	// Emit message.
	Emit(v ...interface{})
}
