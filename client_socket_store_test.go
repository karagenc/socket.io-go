package sio

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClientSocketStore(t *testing.T) {
	store := newClientSocketStore()
	manager := NewManager("http://asdf.jkl", nil)
	main := manager.Socket("/", nil).(*clientSocket)

	store.Set(main)
	s, ok := store.Get("/")
	if !assert.True(t, ok) {
		return
	}
	assert.True(t, main == s)

	foo := manager.Socket("/foo", nil).(*clientSocket)
	store.Set(foo)

	sockets := store.GetAll()
	if !assert.Equal(t, 2, len(sockets)) {
		return
	}
	assert.True(t, main == sockets[0])
	assert.True(t, foo == sockets[1])

	store.Remove("/foo")
	sockets = store.GetAll()
	if !assert.Equal(t, 1, len(sockets)) {
		return
	}
	assert.True(t, main == sockets[0])
}
