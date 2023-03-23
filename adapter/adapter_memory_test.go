package adapter

import (
	"testing"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/stretchr/testify/assert"
	"github.com/tomruk/socket.io-go/parser"
	jsonparser "github.com/tomruk/socket.io-go/parser/json"
	"github.com/tomruk/socket.io-go/parser/json/serializer/stdjson"
)

func TestInMemoryAdapterAddDelete(t *testing.T) {
	adapter := newTestInMemoryAdapter()
	adapter.AddAll("s1", []Room{"r1", "r2"})
	adapter.AddAll("s2", []Room{"r2", "r3"})

	_, ok := adapter.rooms["r1"]
	assert.True(t, ok)
	_, ok = adapter.rooms["r2"]
	assert.True(t, ok)
	_, ok = adapter.rooms["r3"]
	assert.True(t, ok)
	_, ok = adapter.rooms["r4"]
	assert.False(t, ok)

	_, ok = adapter.sids["s1"]
	assert.True(t, ok)
	_, ok = adapter.sids["s2"]
	assert.True(t, ok)
	_, ok = adapter.sids["s3"]
	assert.False(t, ok)

	adapter.Delete("s1", "r1")
	_, ok = adapter.rooms["r1"]
	assert.False(t, ok)

	adapter.DeleteAll("s2")
	_, ok = adapter.sids["s2"]
	assert.False(t, ok)
	_, ok = adapter.rooms["r2"]
	assert.True(t, ok)
	_, ok = adapter.rooms["r3"]
	assert.False(t, ok)
}

func TestSockets(t *testing.T) {
	adapter := newTestInMemoryAdapter()
	store := adapter.sockets.(*testSocketStore)
	store.Set(newTestSocket("s1"))
	store.Set(newTestSocket("s2"))
	store.Set(newTestSocket("s3"))

	adapter.AddAll("s1", []Room{"r1", "r2"})
	adapter.AddAll("s2", []Room{"r2", "r3"})
	adapter.AddAll("s3", []Room{"r3"})

	sockets := adapter.Sockets(mapset.NewThreadUnsafeSet[Room]())
	assert.Equal(t, 3, sockets.Cardinality())

	sockets = adapter.Sockets(mapset.NewThreadUnsafeSet[Room]("r2"))
	assert.Equal(t, 2, sockets.Cardinality())
	sockets = adapter.Sockets(mapset.NewThreadUnsafeSet[Room]("r4"))
	assert.Equal(t, 0, sockets.Cardinality())
}

func TestSocketRooms(t *testing.T) {
	adapter := newTestInMemoryAdapter()
	adapter.AddAll("s1", []Room{"r1", "r2"})
	adapter.AddAll("s2", []Room{"r2", "r3"})
	adapter.AddAll("s3", []Room{"r3"})

	rooms, ok := adapter.SocketRooms("s2")
	assert.True(t, ok)
	assert.Equal(t, 2, rooms.Cardinality())
	_, ok = adapter.SocketRooms("s4")
	assert.False(t, ok)
}

func TestExcludeSockets(t *testing.T) {
	adapter := newTestInMemoryAdapter()
	adapter.AddAll("s1", []Room{"r1"})
	adapter.AddAll("s2", nil)
	adapter.AddAll("s3", []Room{"r1"})

	store := adapter.sockets.(*testSocketStore)
	store.Set(newTestSocket("s1"))
	store.Set(newTestSocket("s2"))
	store.Set(newTestSocket("s3"))

	header := parser.PacketHeader{}
	opts := NewBroadcastOptions()
	opts.Except.Add("r1")
	v := []any{"123"}
	ids := []SocketID{}

	store.sendBuffers = func(sid SocketID, buffers [][]byte) (ok bool) {
		assert.Equal(t, 1, len(buffers))
		assert.Equal(t, `0["123"]`, string(buffers[0]))
		ids = append(ids, sid)
		return
	}

	adapter.Broadcast(&header, v, opts)

	assert.Equal(t, 1, len(ids))
	assert.Equal(t, SocketID("s2"), ids[0])
}

func TestExcludeSocketsWhenBroadcastingToRooms(t *testing.T) {
	adapter := newTestInMemoryAdapter()
	adapter.AddAll("s1", []Room{"r1", "r2"})
	adapter.AddAll("s2", []Room{"r2"})
	adapter.AddAll("s3", []Room{"r1"})

	store := adapter.sockets.(*testSocketStore)
	store.Set(newTestSocket("s1"))
	store.Set(newTestSocket("s2"))
	store.Set(newTestSocket("s3"))

	header := parser.PacketHeader{}
	opts := NewBroadcastOptions()
	opts.Rooms.Add("r1")
	opts.Except.Add("r2")
	v := []any{"123"}
	ids := []SocketID{}

	store.sendBuffers = func(sid SocketID, buffers [][]byte) (ok bool) {
		assert.Equal(t, 1, len(buffers))
		assert.Equal(t, `0["123"]`, string(buffers[0]))
		ids = append(ids, sid)
		return
	}

	adapter.Broadcast(&header, v, opts)

	assert.Equal(t, 1, len(ids))
	assert.Equal(t, SocketID("s3"), ids[0])
}

func TestFetchSockets(t *testing.T) {
	adapter := newTestInMemoryAdapter()
	adapter.AddAll("s1", []Room{"s1"})
	adapter.AddAll("s2", []Room{"s2"})
	adapter.AddAll("s3", []Room{"s3"})

	store := adapter.sockets.(*testSocketStore)
	store.Set(newTestSocket("s1"))
	store.Set(newTestSocket("s2"))
	store.Set(newTestSocket("s3"))

	sockets := adapter.FetchSockets(NewBroadcastOptions())

	assert.Equal(t, 3, len(sockets))
}

func TestAddAndDelSockets(t *testing.T) {
	adapter := newTestInMemoryAdapter()
	adapter.AddAll("s1", []Room{"s1"})
	adapter.AddAll("s2", []Room{"s2"})

	socket1 := newTestSocket("s1")
	socket2 := newTestSocket("s2")
	store := adapter.sockets.(*testSocketStore)
	store.Set(socket1)
	store.Set(socket2)

	opts := NewBroadcastOptions()
	opts.Rooms.Add("s1")
	adapter.AddSockets(opts, "r1", "r2", "r3")

	assert.Equal(t, 4, len(socket1.Rooms))
	assert.Equal(t, Room("s1"), socket1.Rooms[0])
	assert.Equal(t, Room("r1"), socket1.Rooms[1])
	assert.Equal(t, Room("r2"), socket1.Rooms[2])
	assert.Equal(t, Room("r3"), socket1.Rooms[3])

	assert.Equal(t, 1, len(socket2.Rooms))
	assert.Equal(t, Room("s2"), socket2.Rooms[0])

	opts = NewBroadcastOptions()
	opts.Rooms.Add("s1")
	opts.Rooms.Add("s2")
	adapter.DelSockets(opts, "r3", "r2", "s2")

	assert.Equal(t, 2, len(socket1.Rooms))
	assert.Equal(t, Room("s1"), socket1.Rooms[0])
	assert.Equal(t, Room("r1"), socket1.Rooms[1])

	assert.Equal(t, 0, len(socket2.Rooms))
}

func TestDisconnectSockets(t *testing.T) {
	adapter := newTestInMemoryAdapter()
	adapter.AddAll("s1", []Room{"s1"})
	adapter.AddAll("s2", []Room{"s2"})

	socket1 := newTestSocket("s1")
	socket2 := newTestSocket("s2")
	store := adapter.sockets.(*testSocketStore)
	store.Set(socket1)
	store.Set(socket2)

	opts := NewBroadcastOptions()
	opts.Rooms.Add("s1")
	adapter.DisconnectSockets(opts, true)

	assert.False(t, socket1.Connected)
	assert.True(t, socket2.Connected)
}

func TestReturnMatchingSocketsWithinRoom(t *testing.T) {
	adapter := newTestInMemoryAdapter()
	adapter.AddAll("s1", []Room{"r1", "r2"})
	adapter.AddAll("s2", []Room{"r1"})
	adapter.AddAll("s3", []Room{"r2"})

	store := adapter.sockets.(*testSocketStore)
	store.Set(newTestSocket("s1"))
	store.Set(newTestSocket("s2"))
	store.Set(newTestSocket("s3"))

	opts := NewBroadcastOptions()
	opts.Rooms.Add("r1")
	opts.Except.Add("r2")
	sockets := adapter.FetchSockets(opts)

	assert.Equal(t, 1, len(sockets))
	assert.Equal(t, SocketID("s2"), sockets[0].ID())
}

func newTestInMemoryAdapter() *inMemoryAdapter {
	creator := NewInMemoryAdapterCreator()
	return creator(newTestSocketStore(), jsonparser.NewCreator(0, stdjson.New())).(*inMemoryAdapter)
}
