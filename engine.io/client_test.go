package eio

import (
	"bytes"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/tomruk/socket.io-go/engine.io/parser"
)

func testDial(t *testing.T, rawURL string, callbacks *Callbacks, config *ClientConfig) *clientSocket {
	s, err := Dial(rawURL, callbacks, config)
	if err != nil {
		t.Fatal(err)
	}
	socket := s.(*clientSocket)
	return socket
}

func mustCreatePacket(t *testing.T, packetType parser.PacketType, isBinary bool, data []byte) *parser.Packet {
	p, err := parser.NewPacket(packetType, isBinary, data)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func createTestPackets(t *testing.T) []*parser.Packet {
	return []*parser.Packet{
		// Common packets
		mustCreatePacket(t, parser.PacketTypeOpen, false, nil),
		mustCreatePacket(t, parser.PacketTypeClose, false, nil),
		mustCreatePacket(t, parser.PacketTypePing, false, []byte("testing123")),
		mustCreatePacket(t, parser.PacketTypePong, false, []byte("testing123")),
		mustCreatePacket(t, parser.PacketTypeMessage, false, []byte("testing123")),
		mustCreatePacket(t, parser.PacketTypeMessage, true, []byte{0x0, 0x1, 0x2, 0x3}),
		mustCreatePacket(t, parser.PacketTypeUpgrade, false, nil),
		mustCreatePacket(t, parser.PacketTypeNoop, false, nil),

		// Non UTF-8 packets

		// Turkish
		mustCreatePacket(t, parser.PacketTypeMessage, false, []byte("çÇöÖğĞüÜşŞ")),
		mustCreatePacket(t, parser.PacketTypeMessage, true, []byte("çÇöÖğĞüÜşŞ")),

		// Russian
		mustCreatePacket(t, parser.PacketTypeMessage, false, []byte("АаБбВвГгДдЕеЁёЖжЗзИиЙйКкЛлМмНнОоПпРрСсТтУуФфХхЦцЧчШшЩщЪъЫыЬьЭэЮюЯя")),
		mustCreatePacket(t, parser.PacketTypeMessage, true, []byte("АаБбВвГгДдЕеЁёЖжЗзИиЙйКкЛлМмНнОоПпРрСсТтУуФфХхЦцЧчШшЩщЪъЫыЬьЭэЮюЯя")),

		// Chinese
		mustCreatePacket(t, parser.PacketTypeMessage, false, []byte("石室詩士施氏，嗜獅，誓食十獅。")),
		mustCreatePacket(t, parser.PacketTypeMessage, true, []byte("石室詩士施氏，嗜獅，誓食十獅。")),
	}
}

func TestPolling(t *testing.T) {
	testSendReceive(t, []string{"polling"})
}

func TestWebSocket(t *testing.T) {
	testSendReceive(t, []string{"websocket"})
}

func TestPollingAndWebSocket(t *testing.T) {
	testSendReceive(t, []string{"polling", "websocket"})
}

func testSendReceive(t *testing.T, transports []string) {
	tw := newTestWaiter(0)

	test := createTestPackets(t)

	check := func(data []byte, isBinary bool) bool {
		for _, p := range test {
			if p.Type == parser.PacketTypeMessage && p.IsBinary == isBinary {
				if bytes.Equal(p.Data, data) {
					return true
				}
			}
		}
		return false
	}

	send := func(socket Socket) {
		for _, p := range test {
			if p.Type == parser.PacketTypeMessage {
				socket.SendMessage(p.Data, p.IsBinary)
			}
		}
	}

	for _, p := range test {
		if p.Type == parser.PacketTypeMessage {
			tw.Add(2) // Wait for both server and client.
		}
	}

	onSocket := func(socket Socket) *Callbacks {
		callbacks := &Callbacks{
			OnMessage: func(data []byte, isBinary bool) {
				defer tw.Done()

				ok := check(data, isBinary)
				if !ok {
					t.Error("server: invalid message received")
				}
			},
			OnError: func(err error) {
				t.Errorf("unexpected error: %v", err)
			},
		}

		go func() {
			send(socket)
		}()

		return callbacks
	}

	io := NewServer(onSocket, nil)

	err := io.Run()
	if err != nil {
		t.Fatal(err)
	}

	s := httptest.NewServer(io)

	callbacks := &Callbacks{
		OnError: func(err error) {
			t.Errorf("unexpected error: %v", err)
		},
		OnMessage: func(data []byte, isBinary bool) {
			defer tw.Done()

			ok := check(data, isBinary)
			if !ok {
				t.Error("client: invalid message received")
			}
		},
	}

	socket := testDial(t, s.URL, callbacks, &ClientConfig{Transports: transports})

	send(socket)

	tw.WaitTimeout(t, time.Second*3)
}

func TestClientWebSocketClose(t *testing.T) {
	tw := newTestWaiter(1)

	onSocket := func(socket Socket) *Callbacks {
		defer tw.Done()
		return nil
	}

	io := NewServer(onSocket, nil)

	err := io.Run()
	if err != nil {
		t.Fatal(err)
	}

	s := httptest.NewServer(io)

	socket := testDial(t, s.URL, nil, &ClientConfig{Transports: []string{"websocket"}})
	// This test is to check if the socket.Close is blocking.
	socket.Close()

	tw.WaitTimeout(t, time.Second*3)
}

func TestClientWebSocketDiscard(t *testing.T) {
	tw := newTestWaiter(1)

	onSocket := func(socket Socket) *Callbacks {
		defer tw.Done()
		return nil
	}

	io := NewServer(onSocket, nil)

	err := io.Run()
	if err != nil {
		t.Fatal(err)
	}

	s := httptest.NewServer(io)

	socket := testDial(t, s.URL, nil, &ClientConfig{Transports: []string{"websocket"}})
	// This test is to check if the socket.t.Discard is blocking.
	socket.tMu.Lock()
	socket.t.Discard()
	socket.tMu.Unlock()

	tw.WaitTimeout(t, time.Second*3)
}

func TestClientPollingClose(t *testing.T) {
	tw := newTestWaiter(1)

	onSocket := func(socket Socket) *Callbacks {
		defer tw.Done()
		return nil
	}

	io := NewServer(onSocket, nil)

	err := io.Run()
	if err != nil {
		t.Fatal(err)
	}

	s := httptest.NewServer(io)

	socket := testDial(t, s.URL, nil, &ClientConfig{Transports: []string{"polling"}})
	// This test is to check if the socket.Close is blocking.
	socket.Close()

	tw.WaitTimeout(t, time.Second*3)
}

func TestClientPollingDiscard(t *testing.T) {
	tw := newTestWaiter(1)

	onSocket := func(socket Socket) *Callbacks {
		defer tw.Done()
		return nil
	}

	io := NewServer(onSocket, nil)

	err := io.Run()
	if err != nil {
		t.Fatal(err)
	}

	s := httptest.NewServer(io)

	socket := testDial(t, s.URL, nil, &ClientConfig{Transports: []string{"polling"}})
	// This test is to check if the socket.t.Discard is blocking.
	socket.tMu.Lock()
	socket.t.Discard()
	socket.tMu.Unlock()

	tw.WaitTimeout(t, time.Second*3)
}

func TestPingTimeoutAndPingInterval(t *testing.T) {
	tw := newTestWaiter(1)

	const (
		pingInterval = 20 * time.Second
		pingTimeout  = 8 * time.Second
	)

	onSocket := func(socket Socket) *Callbacks {
		defer tw.Done()

		assert.Equal(t, pingInterval, socket.PingInterval())
		assert.Equal(t, pingTimeout, socket.PingTimeout())

		return nil
	}

	io := NewServer(onSocket, &ServerConfig{PingInterval: pingInterval, PingTimeout: pingTimeout})

	err := io.Run()
	if err != nil {
		t.Fatal(err)
	}

	s := httptest.NewServer(io)

	socket := testDial(t, s.URL, nil, nil)

	assert.Equal(t, pingInterval, socket.PingInterval())
	assert.Equal(t, pingTimeout, socket.PingTimeout())

	tw.WaitTimeout(t, time.Second*3)
}

func TestUpgrade(t *testing.T) {
	tw := newTestWaiter(1)

	io := NewServer(nil, nil)

	err := io.Run()
	if err != nil {
		t.Fatal(err)
	}

	s := httptest.NewServer(io)
	transports := []string{"polling", "websocket"}

	upgradeDone := func(transportName string) {
		defer tw.Done()

		if transportName != "websocket" {
			t.Error("transport should have been upgraded to websocket")
		}
	}

	socket := testDial(t, s.URL, nil, &ClientConfig{Transports: transports, UpgradeDone: upgradeDone})
	upgrades := socket.Upgrades()

	if !assert.Equal(t, 1, len(upgrades)) {
		return
	}

	assert.Equal(t, "websocket", upgrades[0])

	tw.WaitTimeout(t, time.Second*3)
}
