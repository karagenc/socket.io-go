package adapter

type SocketStore interface {
	// Send Engine.IO packets to a specific socket.
	SendBuffers(sid SocketID, buffers [][]byte) (ok bool)

	Get(sid SocketID) (so Socket, ok bool)
	GetAll() []Socket

	Remove(sid SocketID)
}
