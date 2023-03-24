package adapter

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBroadcastOperator(t *testing.T) {
	adapter := newTestInMemoryAdapter()
	adapter.AddAll("s1", []Room{"r1", "r2"})
	adapter.AddAll("s2", []Room{"r1"})
	adapter.AddAll("s3", []Room{"r2"})

	store := adapter.sockets.(*TestSocketStore)
	s1 := NewTestSocket("s1")
	s2 := NewTestSocket("s2")
	s3 := NewTestSocket("s3")
	store.Set(s1)
	store.Set(s2)
	store.Set(s3)

	b := NewBroadcastOperator("/", adapter, func(s string) bool { return false })

	t.Run("adapter", func(t *testing.T) {
		t.Run("Emit", func(t *testing.T) {
			var sids []SocketID
			store.sendBuffers = func(sid SocketID, buffers [][]byte) (ok bool) {
				sids = append(sids, sid)
				return true
			}
			b.To("r2").Emit("hi", "I am Groot")
			assert.Contains(t, sids, SocketID("s1"))
			assert.NotContains(t, sids, SocketID("s2"))
			assert.Contains(t, sids, SocketID("s3"))
		})

		t.Run("FetchSockets", func(t *testing.T) {
			sockets := b.FetchSockets()
			assert.Contains(t, sockets, s1)
			assert.Contains(t, sockets, s2)
			assert.Contains(t, sockets, s3)
		})

		t.Run("SocketsJoin and SocketsLeave", func(t *testing.T) {
			b.To("r2").SocketsJoin("r3", "r4")

			assert.Contains(t, s1.Rooms, Room("r3"))
			assert.NotContains(t, s2.Rooms, Room("r3"))
			assert.Contains(t, s3.Rooms, Room("r3"))

			assert.Contains(t, s1.Rooms, Room("r4"))
			assert.NotContains(t, s2.Rooms, Room("r4"))
			assert.Contains(t, s3.Rooms, Room("r4"))

			b.To("r2").SocketsLeave("r3", "r4")

			assert.NotContains(t, s1.Rooms, Room("r3"))
			assert.NotContains(t, s2.Rooms, Room("r3"))
			assert.NotContains(t, s3.Rooms, Room("r3"))

			assert.NotContains(t, s1.Rooms, Room("r4"))
			assert.NotContains(t, s2.Rooms, Room("r4"))
			assert.NotContains(t, s3.Rooms, Room("r4"))
		})

		t.Run("DisconnectSockets", func(t *testing.T) {
			b.To("r2").DisconnectSockets(true)

			assert.False(t, s1.Connected)
			assert.True(t, s2.Connected)
			assert.False(t, s3.Connected)
		})
	})

	t.Run("to and in", func(t *testing.T) {
		bn := b.To("r1", "r2", "r3")
		assert.True(t, b != bn)
		assert.True(t, b.rooms != bn.rooms)
		assert.True(t, bn.rooms.Contains("r1", "r2", "r3"))

		bn = b.In("r1", "r2", "r3")
		assert.True(t, b != bn)
		assert.True(t, b.rooms != bn.rooms)
		assert.True(t, bn.rooms.Contains("r1", "r2", "r3"))
	})

	t.Run("except", func(t *testing.T) {
		bn := b.Except("r1", "r2", "r3")
		assert.True(t, b != bn)
		assert.True(t, b.exceptRooms != bn.exceptRooms)
		assert.True(t, bn.exceptRooms.Contains("r1", "r2", "r3"))
	})

	t.Run("compress", func(t *testing.T) {
		bn := b.Compress(true)
		assert.True(t, b != bn)
		assert.True(t, bn.flags.Compress)

		bn = b.Compress(false)
		assert.True(t, b != bn)
		assert.False(t, bn.flags.Compress)
	})

	t.Run("local", func(t *testing.T) {
		bn := b.Local()
		assert.True(t, b != bn)
		assert.True(t, bn.flags.Local)
	})
}
