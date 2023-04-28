package sio

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClientAck(t *testing.T) {
	server, _, manager := newTestServerAndClient(t, nil, nil)
	socket := manager.Socket("/", nil)
	socket.Connect()
	tw := newTestWaiter(5)

	socket.OnConnect(func() {
		for i := 0; i < 5; i++ {
			t.Log("Emitting to server")
			socket.Emit("ack", "hello", func(reply string) {
				defer tw.Done()
				t.Logf("Ack received. Value: `%s`", reply)
				assert.Equal(t, "hi", reply)
			})
		}
	})

	server.OnConnection(func(socket ServerSocket) {
		socket.OnEvent("ack", func(message string, ack func(reply string)) {
			t.Logf("Message for the `ack` event: %s", message)
			assert.Equal(t, "hello", message)
			ack("hi")
		})
	})
	tw.WaitTimeout(t, defaultTestWaitTimeout)
}

func TestAuth(t *testing.T) {
	_, _, manager := newTestServerAndClient(t, nil, nil)
	socket := manager.Socket("/", nil).(*clientSocket)

	type S struct {
		Num int
	}
	s := &S{
		Num: 500,
	}

	err := socket.setAuth(s)
	if err != nil {
		t.Fatal(err)
	}

	s, ok := socket.Auth().(*S)
	require.True(t, ok)
	require.Equal(t, s.Num, 500)

	err = socket.setAuth("Donkey")
	require.NotNil(t, err)

	require.PanicsWithError(t, "sio: SetAuth: non-JSON data cannot be accepted. please provide a struct or map", func() {
		socket.SetAuth("Donkey")
	})
}

func TestConnectToANamespaceAfterConnectionEstablished(t *testing.T) {
	_, _, manager := newTestServerAndClient(
		t,
		&ServerConfig{
			AcceptAnyNamespace: true,
		},
		nil,
	)
	tw := newTestWaiter(1)
	socket := manager.Socket("/", nil)

	socket.OnConnect(func() {
		t.Log("/ connected")
		asdf := manager.Socket("/asdf", nil)
		asdf.OnConnect(func() {
			t.Log("/asdf connected")
			tw.Done()
		})
		asdf.Connect()
	})
	socket.Connect()

	tw.WaitTimeout(t, defaultTestWaitTimeout)
}

func TestConnectToANewNamespaceAfterConnectionGetsClosed(t *testing.T) {
	_, _, manager := newTestServerAndClient(
		t,
		&ServerConfig{
			AcceptAnyNamespace: true,
		},
		nil,
	)
	socket := manager.Socket("/", nil)
	tw := newTestWaiter(1)

	socket.OnConnect(func() {
		t.Log("/ connected")
		socket.Disconnect()
	})
	socket.OnDisconnect(func(reason Reason) {
		t.Logf("/ disconnected with reason: %s", reason)
		asdf := manager.Socket("/asdf", nil)
		asdf.OnConnect(func() {
			t.Log("/asdf connected")
			tw.Done()
		})
		t.Log("/asdf is connecting")
		asdf.Connect()
	})
	socket.Connect()

	tw.WaitTimeout(t, defaultTestWaitTimeout)
}

func TestManagerOpenWithoutSocket(t *testing.T) {
	server, _, manager := newTestServerAndClient(
		t,
		&ServerConfig{
			AcceptAnyNamespace: true,
			ConnectTimeout:     1000 * time.Millisecond,
		},
		nil,
	)
	tw := newTestWaiter(2)

	server.OnAnyConnection(func(namespace string, socket ServerSocket) {
		t.Fatalf("Connection to `%s` was received. This shouldn't have happened", namespace)
	})

	manager.OnOpen(func() {
		t.Log("Manager connection is established")
		tw.Done()
	})
	manager.OnClose(func(reason Reason, err error) {
		assert.Equal(t, Reason("transport close"), reason)
		tw.Done()
	})
	manager.Open()

	tw.WaitTimeout(t, defaultTestWaitTimeout)
}

func TestReconnectByDefault(t *testing.T) {
	server, _, manager := newTestServerAndClient(
		t,
		nil,
		nil,
	)
	tw := newTestWaiter(1)
	socket := manager.Socket("/", nil)

	server.OnConnection(func(socket ServerSocket) {
		s := socket.(*serverSocket)
		// Abruptly close the connection.
		s.conn.eio.Close()
	})
	manager.OnReconnect(func(attempt uint32) {
		socket.Disconnect()
		tw.Done()
	})

	socket.Connect()
	tw.WaitTimeout(t, defaultTestWaitTimeout)
}
