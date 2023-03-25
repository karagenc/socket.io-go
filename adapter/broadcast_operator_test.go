package adapter

import (
	"testing"

	"github.com/stretchr/testify/require"
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
			require.Contains(t, sids, SocketID("s1"))
			require.NotContains(t, sids, SocketID("s2"))
			require.Contains(t, sids, SocketID("s3"))
		})

		t.Run("FetchSockets", func(t *testing.T) {
			sockets := b.FetchSockets()
			require.Contains(t, sockets, s1)
			require.Contains(t, sockets, s2)
			require.Contains(t, sockets, s3)
		})

		t.Run("SocketsJoin and SocketsLeave", func(t *testing.T) {
			b.To("r2").SocketsJoin("r3", "r4")

			require.Contains(t, s1.Rooms, Room("r3"))
			require.NotContains(t, s2.Rooms, Room("r3"))
			require.Contains(t, s3.Rooms, Room("r3"))

			require.Contains(t, s1.Rooms, Room("r4"))
			require.NotContains(t, s2.Rooms, Room("r4"))
			require.Contains(t, s3.Rooms, Room("r4"))

			b.To("r2").SocketsLeave("r3", "r4")

			require.NotContains(t, s1.Rooms, Room("r3"))
			require.NotContains(t, s2.Rooms, Room("r3"))
			require.NotContains(t, s3.Rooms, Room("r3"))

			require.NotContains(t, s1.Rooms, Room("r4"))
			require.NotContains(t, s2.Rooms, Room("r4"))
			require.NotContains(t, s3.Rooms, Room("r4"))
		})

		t.Run("DisconnectSockets", func(t *testing.T) {
			b.To("r2").DisconnectSockets(true)

			require.False(t, s1.Connected)
			require.True(t, s2.Connected)
			require.False(t, s3.Connected)
		})
	})

	t.Run("to and in", func(t *testing.T) {
		bn := b.To("r1", "r2", "r3")
		require.True(t, b != bn)
		require.True(t, b.rooms != bn.rooms)
		require.True(t, bn.rooms.Contains("r1", "r2", "r3"))

		bn = b.In("r1", "r2", "r3")
		require.True(t, b != bn)
		require.True(t, b.rooms != bn.rooms)
		require.True(t, bn.rooms.Contains("r1", "r2", "r3"))
	})

	t.Run("except", func(t *testing.T) {
		bn := b.Except("r1", "r2", "r3")
		require.True(t, b != bn)
		require.True(t, b.exceptRooms != bn.exceptRooms)
		require.True(t, bn.exceptRooms.Contains("r1", "r2", "r3"))
	})

	t.Run("compress", func(t *testing.T) {
		bn := b.Compress(true)
		require.True(t, b != bn)
		require.True(t, bn.flags.Compress)

		bn = b.Compress(false)
		require.True(t, b != bn)
		require.False(t, bn.flags.Compress)
	})

	t.Run("local", func(t *testing.T) {
		bn := b.Local()
		require.True(t, b != bn)
		require.True(t, bn.flags.Local)
	})
}
