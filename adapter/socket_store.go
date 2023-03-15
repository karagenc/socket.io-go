package adapter

type SocketStore interface {
	Get(sid SocketID) (so Socket, ok bool)

	// Send Engine.IO packets to a specific socket.
	SendBuffers(sid SocketID, buffers [][]byte)

	GetAll() []Socket

	Set(so Socket)

	Remove(sid SocketID)
}
