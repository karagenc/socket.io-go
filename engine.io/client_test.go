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

	send := func(socket ServerSocket) {
		for _, p := range test {
			if p.Type == parser.PacketTypeMessage {
				socket.Send(p)
			}
		}
	}

	for _, p := range test {
		if p.Type == parser.PacketTypeMessage {
			tw.Add(2) // Wait for both server and client.
		}
	}

	onSocket := func(socket ServerSocket) *Callbacks {
		callbacks := &Callbacks{
			OnPacket: func(packets ...*parser.Packet) {
				for _, packet := range packets {
					if packet.Type == parser.PacketTypeMessage {
						defer tw.Done()

						ok := check(packet.Data, packet.IsBinary)
						if !ok {
							t.Error("server: invalid message received")
						}
					}
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
		OnPacket: func(packets ...*parser.Packet) {
			for _, packet := range packets {
				if packet.Type == parser.PacketTypeMessage {
					defer tw.Done()

					ok := check(packet.Data, packet.IsBinary)
					if !ok {
						t.Error("client: invalid message received")
					}
				}
			}
		},
	}

	socket := testDial(t, s.URL, callbacks, &ClientConfig{Transports: transports})

	send(socket)

	tw.WaitTimeout(t, defaultTestWaitTimeout)
}

func TestClientWebSocketClose(t *testing.T) {
	tw := newTestWaiter(1)

	onSocket := func(socket ServerSocket) *Callbacks {
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

	tw.WaitTimeout(t, defaultTestWaitTimeout)
}

func TestClientWebSocketDiscard(t *testing.T) {
	tw := newTestWaiter(1)

	onSocket := func(socket ServerSocket) *Callbacks {
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
	// This test is to check if the socket.transport.Discard is blocking.
	socket.transportMu.Lock()
	socket.transport.Discard()
	socket.transportMu.Unlock()

	tw.WaitTimeout(t, defaultTestWaitTimeout)
}

func TestClientPollingClose(t *testing.T) {
	tw := newTestWaiter(1)

	onSocket := func(socket ServerSocket) *Callbacks {
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

	tw.WaitTimeout(t, defaultTestWaitTimeout)
}

func TestClientPollingDiscard(t *testing.T) {
	tw := newTestWaiter(1)

	onSocket := func(socket ServerSocket) *Callbacks {
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
	// This test is to check if the socket.transport.Discard is blocking.
	socket.transportMu.Lock()
	socket.transport.Discard()
	socket.transportMu.Unlock()

	tw.WaitTimeout(t, defaultTestWaitTimeout)
}

func TestPingTimeoutAndPingInterval(t *testing.T) {
	tw := newTestWaiter(1)

	const (
		pingInterval = 20 * time.Second
		pingTimeout  = 8 * time.Second
	)

	onSocket := func(socket ServerSocket) *Callbacks {
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

	tw.WaitTimeout(t, defaultTestWaitTimeout)
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

	tw.WaitTimeout(t, defaultTestWaitTimeout)
}
