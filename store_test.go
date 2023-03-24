package sio

import (
	"fmt"
	"testing"
	"time"

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

func TestServerSocketStore(t *testing.T) {
	store := newServerSocketStore()
	server, _, manager := newTestServerAndClient(t, nil, nil)

	var (
		socket   *serverSocket
		socketTW = newTestWaiter(0)
	)

	socketTW.Add(1)
	server.Of("/").OnConnection(func(_socket ServerSocket) {
		fmt.Printf("New connection to `/` with sid: %s\n", _socket.ID())
		_socket.OnError(func(err error) {
			t.Fatal(err)
		})
		socket = _socket.(*serverSocket)
		store.set(socket)
		socketTW.Done()
	})

	manager.Socket("/", nil).Connect()
	timedout := socketTW.WaitTimeout(t, 15*time.Second)
	if timedout {
		return
	}

	assert.Equal(t, 1, len(store.socketsByID))
	assert.Equal(t, 1, len(store.socketsByNamespace))

	s, ok := store.getByID(socket.ID())
	if !assert.True(t, ok) {
		return
	}
	assert.True(t, socket == s)

	s, ok = store.getByNsp("/")
	if !assert.True(t, ok) {
		return
	}
	assert.True(t, socket == s)

	sockets := store.getAll()
	if !assert.Equal(t, 1, len(sockets)) {
		return
	}
	assert.Contains(t, sockets, socket)

	sockets = store.getAndRemoveAll()
	if !assert.Equal(t, 1, len(sockets)) {
		return
	}
	assert.Contains(t, sockets, socket)
	assert.Equal(t, 0, len(store.socketsByID))
	assert.Equal(t, 0, len(store.socketsByNamespace))

	socketTW.Add(1)
	server.Of("/asdf").OnConnection(func(_socket ServerSocket) {
		fmt.Printf("New connection to `/asdf` with sid: %s\n", _socket.ID())
		_socket.OnError(func(err error) {
			t.Fatal(err)
		})
		socket = _socket.(*serverSocket)
		store.set(socket)
		socketTW.Done()
	})

	//manager = NewManager(httpServer.URL, nil)
	manager.Socket("/asdf", nil).Connect()
	timedout = socketTW.WaitTimeout(t, 15*time.Second)
	if timedout {
		return
	}

	assert.Equal(t, 1, len(store.socketsByID))
	assert.Equal(t, 1, len(store.socketsByNamespace))

	s, ok = store.getByNsp("/asdf")
	if !assert.True(t, ok) {
		return
	}
	assert.True(t, socket == s)

	store.removeByID(socket.ID())
	assert.Equal(t, 0, len(store.socketsByID))
	assert.Equal(t, 0, len(store.socketsByNamespace))
}

func TestNamespaceStore(t *testing.T) {
	store := newNamespaceStore()
	server, _, _ := newTestServerAndClient(t, nil, nil)

	main := server.Of("/")
	asdf := server.Of("/asdf")
	store.set(main)
	store.set(asdf)

	assert.Equal(t, 2, store.len())
	n, ok := store.get("/")
	assert.True(t, ok)
	assert.True(t, n == main)

	n, ok = store.get("/asdf")
	assert.True(t, ok)
	assert.True(t, n == asdf)

	store.remove("/asdf")
	n, ok = store.get("/asdf")
	assert.False(t, ok)
	assert.True(t, n == nil)

	n, created := store.getOrCreate("/jkl", server, server.adapterCreator, server.parserCreator)
	assert.True(t, created)
	assert.Equal(t, "/jkl", n.Name())

	n, created = store.getOrCreate("/", server, server.adapterCreator, server.parserCreator)
	assert.False(t, created)
	assert.True(t, n == main)
}
