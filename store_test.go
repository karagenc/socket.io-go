package sio

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/tomruk/socket.io-go/internal/sync"

	"github.com/stretchr/testify/require"
	"github.com/tomruk/socket.io-go/parser"
)

func TestClientSocketStore(t *testing.T) {
	store := newClientSocketStore()
	manager := NewManager("http://asdf.jkl", nil)
	main := manager.Socket("/", nil).(*clientSocket)

	store.set(main)
	s, ok := store.get("/")
	require.True(t, ok)
	require.True(t, main == s)

	foo := manager.Socket("/foo", nil).(*clientSocket)
	store.set(foo)

	sockets := store.getAll()
	require.Equal(t, 2, len(sockets))
	require.Contains(t, sockets, main)
	require.Contains(t, sockets, foo)
	// We used to this, but maps are not ordered, so we do the above test.
	// require.True(t, main == sockets[0])
	// require.True(t, foo == sockets[1])

	store.remove("/foo")
	sockets = store.getAll()
	require.Equal(t, 1, len(sockets))
	require.True(t, main == sockets[0])
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
		t.Logf("New connection to `/` with sid: %s", _socket.ID())
		socket = _socket.(*serverSocket)
		store.set(socket)
		socketTW.Done()
	})

	manager.Socket("/", nil).Connect()
	timedout := socketTW.WaitTimeout(t, defaultTestWaitTimeout)
	if timedout {
		return
	}

	require.Equal(t, 1, len(store.socketsByID))
	require.Equal(t, 1, len(store.socketsByNsp))

	s, ok := store.getByID(socket.ID())
	require.True(t, ok)
	require.True(t, socket == s)

	s, ok = store.getByNsp("/")
	require.True(t, ok)
	require.True(t, socket == s)

	sockets := store.getAll()
	require.Equal(t, 1, len(sockets))
	require.Contains(t, sockets, socket)

	sockets = store.getAndRemoveAll()
	require.Equal(t, 1, len(sockets))
	require.Contains(t, sockets, socket)
	require.Equal(t, 0, len(store.socketsByID))
	require.Equal(t, 0, len(store.socketsByNsp))

	socketTW.Add(1)
	server.Of("/asdf").OnConnection(func(_socket ServerSocket) {
		t.Logf("New connection to `/asdf` with sid: %s", _socket.ID())
		socket = _socket.(*serverSocket)
		store.set(socket)
		socketTW.Done()
	})

	//manager = NewManager(httpServer.URL, nil)
	manager.Socket("/asdf", nil).Connect()
	timedout = socketTW.WaitTimeout(t, defaultTestWaitTimeout)
	if timedout {
		return
	}

	require.Equal(t, 1, len(store.socketsByID))
	require.Equal(t, 1, len(store.socketsByNsp))

	s, ok = store.getByNsp("/asdf")
	require.True(t, ok)
	require.True(t, socket == s)

	store.removeByID(socket.ID())
	require.Equal(t, 0, len(store.socketsByID))
	require.Equal(t, 0, len(store.socketsByNsp))
}

func TestNamespaceStore(t *testing.T) {
	store := newNspStore()
	server, _, _ := newTestServerAndClient(t, nil, nil)

	main := server.Of("/")
	asdf := server.Of("/asdf")
	store.set(main)
	store.set(asdf)

	require.Equal(t, 2, store.len())
	n, ok := store.get("/")
	require.True(t, ok)
	require.True(t, n == main)

	n, ok = store.get("/asdf")
	require.True(t, ok)
	require.True(t, n == asdf)

	store.remove("/asdf")
	n, ok = store.get("/asdf")
	require.False(t, ok)
	require.True(t, n == nil)

	n, created := store.getOrCreate("/jkl", server, server.adapterCreator, server.parserCreator)
	require.True(t, created)
	require.Equal(t, "/jkl", n.Name())

	n, created = store.getOrCreate("/", server, server.adapterCreator, server.parserCreator)
	require.False(t, created)
	require.True(t, n == main)
}

func TestNamespaceSocketStore(t *testing.T) {
	store := newNspSocketStore()
	server, _, manager := newTestServerAndClient(t, nil, nil)
	tw := newTestWaiter(2)

	var (
		main, asdf ServerSocket
	)

	server.Of("/").OnConnection(func(socket ServerSocket) {
		main = socket
		tw.Done()
	})

	server.Of("/asdf").OnConnection(func(socket ServerSocket) {
		asdf = socket
		tw.Done()
	})

	manager.Socket("/", nil).Connect()
	manager.Socket("/asdf", nil).Connect()
	timedout := tw.WaitTimeout(t, defaultTestWaitTimeout)
	if timedout {
		return
	}

	store.set(main)
	store.set(asdf)
	require.Equal(t, 2, len(store.sockets))

	s, ok := store.get(main.ID())
	require.True(t, ok)
	require.True(t, s == main)

	s, ok = store.get(asdf.ID())
	require.True(t, ok)
	require.True(t, s == asdf)

	store.remove(asdf.ID())
	s, ok = store.get("/asdf")
	require.False(t, ok)
	require.True(t, s == nil)

	sockets := store.getAll()
	require.Equal(t, 1, len(sockets))
	require.True(t, sockets[0] == main)

	// There is no such socket.
	ok = store.sendBuffers("", nil)
	require.False(t, ok)

	tw.Add(1)
	manager.Socket("/", nil).OnEvent("hi", func(message string) {
		require.Equal(t, "I am Groot", message)
		tw.Done()
	})

	_main := main.(*serverSocket)
	_, buffers := mustCreateEventPacket(_main, "hi", []any{"I am Groot"})
	store.sendBuffers(main.ID(), buffers)

	tw.WaitTimeout(t, defaultTestWaitTimeout)
}

func mustCreateEventPacket(socket *serverSocket, eventName string, _v []any) (header *parser.PacketHeader, buffers [][]byte) {
	header = &parser.PacketHeader{
		Type:      parser.PacketTypeEvent,
		Namespace: socket.nsp.Name(),
	}

	if IsEventReservedForServer(eventName) {
		panic(fmt.Errorf("sio: Emit: attempted to emit a reserved event: `%s`", eventName))
	}

	v := make([]any, 0, len(_v)+1)
	v = append(v, eventName)
	v = append(v, _v...)

	var err error
	buffers, err = socket.parser.Encode(header, &v)
	if err != nil {
		panic(err)
	}
	return
}

func TestHandlerStore(t *testing.T) {
	type testFn func()

	t.Run("on and off", func(t *testing.T) {
		store := newHandlerStore[*testFn]()
		count := 0
		var f testFn = func() {
			count++
		}
		store.on(&f)

		all := store.getAll()
		c := all[0]
		(*c)()
		require.Equal(t, 1, count)

		store.off(&f)
		all = store.getAll()
		require.Equal(t, 0, len(all))
	})

	t.Run("forEach", func(t *testing.T) {
		store := newHandlerStore[*testFn]()
		count := 0
		var f testFn = func() {
			count++
		}
		store.on(&f)
		store.once(&f)

		store.forEach(func(handler *testFn) {
			(*handler)()
		}, false)
		store.offAll()

		wg := sync.WaitGroup{}
		wg.Add(2)
		count = 0
		f = func() {
			count++
			wg.Done()
		}
		store.on(&f)
		store.once(&f)

		store.forEach(func(handler *testFn) {
			(*handler)()
		}, true)

		wg.Wait()
		require.Equal(t, 2, count)

		store.offAll()
		store.forEach(func(handler *testFn) {
			t.Fatal("This function should not be run")
		}, false)
	})

	t.Run("once", func(t *testing.T) {
		store := newHandlerStore[*testFn]()
		count := 0
		var f testFn = func() {
			count++
		}
		store.once(&f)

		all := store.getAll()
		c := all[0]
		(*c)()
		require.Equal(t, 1, count)

		all = store.getAll()
		require.Equal(t, 0, len(all))

		store.once(&f)
		store.off(&f)

		all = store.getAll()
		require.Equal(t, 0, len(all))
	})

	t.Run("offAll", func(t *testing.T) {
		store := newHandlerStore[*testFn]()
		var f testFn = func() {}

		store.on(&f)
		store.once(&f)
		store.offAll()

		all := store.getAll()
		require.Equal(t, 0, len(all))
	})

	t.Run("subevents", func(t *testing.T) {
		store := newHandlerStore[*testFn]()
		var f testFn = func() {}

		store.onSubEvent(&f)
		require.True(t, store.subs[0] == &f)

		all := store.getAll()
		require.True(t, all[0] == &f)

		store.offSubEvents()
		require.Equal(t, 0, len(store.subs))
		all = store.getAll()
		require.Equal(t, 0, len(all))
	})
}

func TestEventHandlerStore(t *testing.T) {
	t.Run("on and off", func(t *testing.T) {
		store := newEventHandlerStore()
		sum := 0
		f := func(x, y int) {
			sum = x + y
		}

		h, err := newEventHandler(f)
		if err != nil {
			t.Fatal(err)
		}
		store.on("sum", h)

		all := store.getAll("sum")
		require.Equal(t, 1, len(all))
		c := all[0]

		x := 6
		y := 3
		_, err = c.call(reflect.ValueOf(x), reflect.ValueOf(y))
		if err != nil {
			t.Fatal(err)
		}

		require.Equal(t, 9, sum)

		store.off("sum")
		all = store.getAll("sum")
		require.Equal(t, 0, len(all))
	})

	t.Run("once", func(t *testing.T) {
		store := newEventHandlerStore()
		f := func() {}
		h, err := newEventHandler(f)
		if err != nil {
			t.Fatal(err)
		}
		store.once("ff", h)

		all := store.getAll("ff")
		require.Equal(t, 1, len(all))
		c := all[0]
		require.True(t, c == h)

		all = store.getAll("ff")
		require.Equal(t, 0, len(all))

		store.once("ff", h)
		store.off("ff")

		all = store.getAll("ff")
		require.Equal(t, 0, len(all))
	})

	t.Run("off", func(t *testing.T) {
		store := newEventHandlerStore()
		f1 := func() {}
		h1, err := newEventHandler(f1)
		if err != nil {
			t.Fatal(err)
		}

		f2 := func() {}
		h2, err := newEventHandler(f2)
		if err != nil {
			t.Fatal(err)
		}

		store.on("ff", h1)
		store.once("ff", h2)

		store.off("ff", reflect.ValueOf(f1), reflect.ValueOf(f2))

		require.Equal(t, 0, len(store.events))
		require.Equal(t, 0, len(store.eventsOnce))

		store.on("ff", h1)
		store.on("ff", h2)
		store.once("ff", h1)
		store.once("ff", h2)

		store.off("ff", reflect.ValueOf(f1))

		require.Equal(t, 1, len(store.events))
		require.Equal(t, 1, len(store.eventsOnce))

		store.offAll() // Cleanup
	})

	t.Run("offAll", func(t *testing.T) {
		store := newEventHandlerStore()
		f := func() {}

		h, err := newEventHandler(f)
		if err != nil {
			t.Fatal(err)
		}

		store.on("ff", h)
		store.once("ff", h)
		store.offAll()

		all := store.getAll("ff")
		require.Equal(t, 0, len(all))
	})
}
