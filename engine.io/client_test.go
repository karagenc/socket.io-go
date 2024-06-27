package eio

import (
	"bytes"
	"errors"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/tomruk/socket.io-go/engine.io/parser"
	"github.com/tomruk/socket.io-go/internal/utils"
)

type testDialOptions struct {
	testWaitUpgrade bool
}

func testDial(t *testing.T, rawURL string, callbacks *Callbacks, config *ClientConfig, options *testDialOptions) *clientSocket {
	if options == nil {
		options = new(testDialOptions)
	}
	if config == nil {
		config = new(ClientConfig)
	}
	if os.Getenv("EIO_DEBUGGER_PRINT") == "1" {
		config.Debugger = NewPrintDebugger()
	}
	s, err := dial(rawURL, callbacks, config, options.testWaitUpgrade)
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

func testSendReceive(t *testing.T, transports []string) {
	tw := utils.NewTestWaiter(0)
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

	io, close := newTestServer(t, onSocket, nil, nil)
	ts := httptest.NewServer(io)

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

	socket := testDial(t, ts.URL, callbacks, &ClientConfig{Transports: transports}, nil)
	send(socket)

	tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
	close()
	ts.Close()
}

func TestCommon(t *testing.T) {
	t.Run("should send and receive with transport set to polling", func(t *testing.T) {
		testSendReceive(t, []string{"polling"})
	})

	t.Run("should send and receive with transport set to websocket", func(t *testing.T) {
		testSendReceive(t, []string{"websocket"})
	})

	t.Run("should send and receive with transport set to polling and websocket", func(t *testing.T) {
		testSendReceive(t, []string{"polling", "websocket"})
	})
}

func TestClient(t *testing.T) {
	t.Run("should emit error if `UpgradeTimeout` is set and is exceeded", func(t *testing.T) {
		tw := utils.NewTestWaiter(2)

		onSocket := func(socket ServerSocket) *Callbacks {
			return &Callbacks{
				OnPacket: func(packets ...*parser.Packet) {
					defer tw.Done()
					// Receive packet as normal while upgrading.
					assert.Equal(t, 1, len(packets))
					packet := packets[0]
					assert.Equal(t, packet.Type, parser.PacketTypeMessage)
					assert.Equal(t, packet.Data, []byte("123456"))
				},
			}
		}

		io, close := newTestServer(t, onSocket, nil, &testServerOptions{
			testWaitUpgrade: true,
		})
		ts := httptest.NewServer(io)

		callbacks := &Callbacks{
			OnError: func(err error) {
				assert.True(t, errors.Is(err, errUpgradeTimeoutExceeded))
				tw.Done()
			},
		}
		socket := testDial(t, ts.URL, callbacks, &ClientConfig{
			UpgradeDone: func(transportName string) {
				t.Fatalf("transport upgraded to: %s", transportName)
			},
			UpgradeTimeout: 10 * time.Millisecond,
		}, nil)
		assert.Equal(t, "polling", socket.TransportName())

		socket.Send(mustCreatePacket(t, parser.PacketTypeMessage, false, []byte("123456")))

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		close()
		ts.Close()
	})

	t.Run("`Close` should not block", func(t *testing.T) {
		tw := utils.NewTestWaiter(1)

		onSocket := func(socket ServerSocket) *Callbacks {
			defer tw.Done()
			return nil
		}
		io, close := newTestServer(t, onSocket, nil, nil)
		ts := httptest.NewServer(io)

		socket := testDial(t, ts.URL, nil, &ClientConfig{Transports: []string{"websocket"}}, nil)
		// This test is to check if the socket.Close is blocking.
		socket.Close()

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		close()
		ts.Close()
	})

	t.Run("`Discard` should not block", func(t *testing.T) {
		tw := utils.NewTestWaiter(1)

		onSocket := func(socket ServerSocket) *Callbacks {
			defer tw.Done()
			return nil
		}
		io, close := newTestServer(t, onSocket, nil, nil)
		ts := httptest.NewServer(io)

		socket := testDial(t, ts.URL, nil, &ClientConfig{Transports: []string{"websocket"}}, nil)
		socket.transportMu.Lock()
		socket.transport.Discard()
		socket.transportMu.Unlock()

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		close()
		ts.Close()
	})

	t.Run("`Close` should not block with transport set to polling", func(t *testing.T) {
		tw := utils.NewTestWaiter(1)

		onSocket := func(socket ServerSocket) *Callbacks {
			defer tw.Done()
			return nil
		}
		io, close := newTestServer(t, onSocket, nil, nil)
		ts := httptest.NewServer(io)

		socket := testDial(t, ts.URL, nil, &ClientConfig{Transports: []string{"polling"}}, nil)
		// This test is to check if the socket.Close is blocking.
		socket.Close()

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		close()
		ts.Close()
	})

	t.Run("`Discard` should not block with transport set to polling", func(t *testing.T) {
		tw := utils.NewTestWaiter(1)

		onSocket := func(socket ServerSocket) *Callbacks {
			defer tw.Done()
			return nil
		}
		io, close := newTestServer(t, onSocket, nil, nil)
		ts := httptest.NewServer(io)

		socket := testDial(t, ts.URL, nil, &ClientConfig{Transports: []string{"polling"}}, nil)
		// This test is to check if the socket.transport.Discard is blocking.
		socket.transportMu.Lock()
		socket.transport.Discard()
		socket.transportMu.Unlock()

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		close()
		ts.Close()
	})

	t.Run("ping timeout and ping interval should be set", func(t *testing.T) {
		tw := utils.NewTestWaiter(1)

		const (
			pingInterval = 20 * time.Second
			pingTimeout  = 8 * time.Second
		)

		onSocket := func(socket ServerSocket) *Callbacks {
			assert.Equal(t, pingInterval, socket.PingInterval())
			assert.Equal(t, pingTimeout, socket.PingTimeout())
			tw.Done()
			return nil
		}
		io, close := newTestServer(t, onSocket, &ServerConfig{PingInterval: pingInterval, PingTimeout: pingTimeout}, nil)
		ts := httptest.NewServer(io)

		socket := testDial(t, ts.URL, nil, nil, nil)
		assert.Equal(t, pingInterval, socket.PingInterval())
		assert.Equal(t, pingTimeout, socket.PingTimeout())

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		close()
		ts.Close()
	})

	t.Run("should upgrade", func(t *testing.T) {
		tw := utils.NewTestWaiter(1)

		io, close := newTestServer(t, nil, nil, nil)
		ts := httptest.NewServer(io)
		transports := []string{"polling", "websocket"}

		upgradeDone := func(transportName string) {
			defer tw.Done()

			if transportName != "websocket" {
				t.Error("transport should have been upgraded to websocket")
			}
		}

		socket := testDial(t, ts.URL, nil, &ClientConfig{Transports: transports, UpgradeDone: upgradeDone}, nil)
		upgrades := socket.Upgrades()
		assert.Equal(t, 1, len(upgrades))
		assert.Equal(t, "websocket", upgrades[0])

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		close()
		ts.Close()
	})

	t.Run("should merge packets", func(t *testing.T) {
		tw := utils.NewTestWaiter(2)

		var (
			checkIndex  = 0
			testPackets = []*parser.Packet{
				mustCreatePacket(t, parser.PacketTypeMessage, false, []byte("12345678")),
				mustCreatePacket(t, parser.PacketTypeMessage, false, []byte("87654321")),
			}
		)

		io, close := newTestServer(t, func(socket ServerSocket) *Callbacks {
			return &Callbacks{
				OnPacket: func(packets ...*parser.Packet) {
					for _, packet := range packets {
						if packet.Type != parser.PacketTypeMessage {
							continue
						}
						defer tw.Done()
						testPacket := testPackets[checkIndex]
						assert.Equal(t, testPacket.IsBinary, packet.IsBinary)
						assert.Equal(t, testPacket.Data, packet.Data)
						checkIndex++
					}
				},
			}
		}, &ServerConfig{
			MaxBufferSize: 9,
		}, nil)
		ts := httptest.NewServer(io)

		socket := testDial(t, ts.URL, nil, &ClientConfig{
			Transports: []string{"polling"},
		}, nil)
		socket.Send(testPackets...)

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		close()
		ts.Close()
	})
}
