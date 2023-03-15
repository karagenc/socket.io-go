package sio

import (
	"time"

	"github.com/tomruk/socket.io-go/adapter"
)

type (
	SocketID = adapter.SocketID
	Room     = adapter.Room
)

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

	// Emit a message.
	// If you want to emit a binary data, use sio.Binary instead of []byte.
	Emit(eventName string, v ...any)

	// Return an emitter with timeout set.
	Timeout(timeout time.Duration) Emitter

	// Register an event handler.
	OnEvent(eventName string, handler any)

	// Register a one-time event handler.
	// The handler will run once and will be removed afterwards.
	OnceEvent(eventName string, handler any)

	// Remove an event handler.
	//
	// If you want to remove all handlers of a particular event,
	// provide the eventName and leave the handler nil.
	//
	// Otherwise, provide both the eventName and handler arguments.
	OffEvent(eventName string, handler ...any)

	// Remove all event handlers.
	// Including special event handlers (connect, disconnect, disconnecting, etc.).
	OffAll()
}
