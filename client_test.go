package sio

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
	if !assert.True(t, ok) {
		return
	}
	assert.Equal(t, s.Num, 500)

	err = socket.setAuth("Donkey")
	if err != nil {
		t.Fatal(err)
	}

	assert.PanicsWithError(t, "sio: SetAuth: non-JSON data cannot be accepted. please provide a struct or map", func() {
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
	socket.Connect()

	socket.OnConnect(func() {
		t.Log("/ connected")
		asdf := manager.Socket("/asdf", nil)
		asdf.OnConnect(func() {
			t.Log("/asdf connected")
			tw.Done()
		})
		asdf.Connect()
	})

	tw.WaitTimeout(t, defaultTestWaitTimeout)
	tw.Wait()
}
