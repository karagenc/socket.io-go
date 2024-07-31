package utils

import (
	"net/http"
	"time"

	"github.com/karagenc/socket.io-go/engine.io/parser"
	"github.com/karagenc/socket.io-go/engine.io/transport"
)

type TestSocket struct {
	id       string
	Closed   bool
	SendFunc func(packets ...*parser.Packet)
}

func NewTestSocket(id string) *TestSocket {
	return &TestSocket{
		id:       id,
		SendFunc: func(packets ...*parser.Packet) {},
	}
}

// Session ID (sid)
func (s *TestSocket) ID() string { return s.id }

func (s *TestSocket) PingInterval() time.Duration { return time.Second * 20 }
func (s *TestSocket) PingTimeout() time.Duration  { return time.Second * 25 }

// Name of the current transport
func (s *TestSocket) TransportName() string { return "polling" }

func (s *TestSocket) Send(packets ...*parser.Packet) { s.SendFunc(packets...) }

func (s *TestSocket) Close() { s.Closed = true }

type TestServerTransport struct {
	Callbacks *transport.Callbacks
}

func NewTestServerTransport() *TestServerTransport {
	return &TestServerTransport{Callbacks: transport.NewCallbacks()}
}

func (t *TestServerTransport) Name() string { return "fake" }

func (t *TestServerTransport) Handshake(
	_ *parser.Packet,
	w http.ResponseWriter,
	r *http.Request,
) (string, error) {
	return "", nil
}

func (t *TestServerTransport) PostHandshake(handshakePacket *parser.Packet) {}

func (t *TestServerTransport) ServeHTTP(w http.ResponseWriter, r *http.Request) {}

func (t *TestServerTransport) QueuedPackets() []*parser.Packet { return nil }

func (t *TestServerTransport) Send(packets ...*parser.Packet) {}

func (t *TestServerTransport) Discard() {}
func (t *TestServerTransport) Close()   {}
