package eio

import (
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
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
		ft := newFakeServerTransport()
		socket := newServerSocket(sid, nil, ft, 0, 0, NewNoopDebugger(), onClose)

		ok := store.set(socket.ID(), socket)
		assert.True(t, ok)
	}

	all := store.getAll()
	assert.Equal(t, max, len(all))

	for _, socket1 := range all {
		if !assert.NotNil(t, socket1) {
			break
		}

		sid := socket1.ID()
		socket2, ok := store.get(sid)

		if socket1 != socket2 {
			t.Fatal("socket1 and socket2 should be the same one")
		}

		assert.NotNil(t, socket2)
		assert.True(t, ok)

		assert.Equal(t, sid, socket2.ID())

		exists := store.exists(sid)
		assert.True(t, exists)
	}

	store.closeAll()

	countMu.Lock()
	assert.Equal(t, max, count, "all sockets should be closed")
	countMu.Unlock()

	for i := 0; i < max; i++ {
		sid := strconv.Itoa(i)

		store.delete(sid)
		exists := store.exists(sid)
		assert.False(t, exists)
	}
}
