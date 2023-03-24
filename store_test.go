package sio

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClientSocketStore(t *testing.T) {
	store := newClientSocketStore()
	manager := NewManager("http://asdf.jkl", nil)
	main := manager.Socket("/", nil).(*clientSocket)

	store.set(main)
	s, ok := store.get("/")
	if !assert.True(t, ok) {
		return
	}
	assert.True(t, main == s)

	foo := manager.Socket("/foo", nil).(*clientSocket)
	store.set(foo)

	sockets := store.getAll()
	if !assert.Equal(t, 2, len(sockets)) {
		return
	}
	assert.Contains(t, sockets, main)
	assert.Contains(t, sockets, foo)
	// We used to this, but maps are not ordered, so we do the above test.
	// assert.True(t, main == sockets[0])
	// assert.True(t, foo == sockets[1])

	store.remove("/foo")
	sockets = store.getAll()
	if !assert.Equal(t, 1, len(sockets)) {
		return
	}
	assert.True(t, main == sockets[0])
}
