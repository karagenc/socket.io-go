package eio

import (
	"strconv"
	"testing"

	"github.com/tomruk/socket.io-go/internal/sync"

	"github.com/stretchr/testify/require"
)

func TestSocketStore(t *testing.T) {
	store := newSocketStore()

	const max = 100
	count := 0
	countMu := new(sync.Mutex)

	onClose := func(sid string) {
		countMu.Lock()
		count++
		countMu.Unlock()
	}

	for i := 0; i < max; i++ {
		sid := strconv.Itoa(i)
		ft := newTestServerTransport()
		c := ft.callbacks
		socket := newServerSocket(sid, nil, ft, c, 0, 0, NewNoopDebugger(), onClose)

		ok := store.set(socket.ID(), socket)
		require.True(t, ok)
	}

	all := store.getAll()
	require.Equal(t, max, len(all))

	for _, socket1 := range all {
		require.NotNil(t, socket1)

		sid := socket1.ID()
		socket2, ok := store.get(sid)

		require.NotNil(t, socket2)
		require.True(t, socket1 == socket2)
		require.True(t, ok)

		require.Equal(t, sid, socket2.ID())

		exists := store.exists(sid)
		require.True(t, exists)
	}

	store.closeAll()

	countMu.Lock()
	require.Equal(t, max, count, "all sockets should be closed")
	countMu.Unlock()

	for i := 0; i < max; i++ {
		sid := strconv.Itoa(i)

		store.delete(sid)
		exists := store.exists(sid)
		require.False(t, exists)
	}
}
