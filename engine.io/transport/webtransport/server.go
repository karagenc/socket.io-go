package webtransport

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/quic-go/webtransport-go"
	"github.com/tomruk/socket.io-go/internal/sync"

	"github.com/tomruk/socket.io-go/engine.io/parser"
	"github.com/tomruk/socket.io-go/engine.io/transport"
)

type ServerTransport struct {
	readLimit int64

	ctx           context.Context
	server        *webtransport.Server
	conn          *webtransport.Session
	stream        webtransport.Stream
	limitedReader *limitedReader
	sendMu        sync.Mutex

	callbacks *transport.Callbacks
	once      sync.Once
}

func NewServerTransport(
	callbacks *transport.Callbacks,
	maxBufferSize int,
	server *webtransport.Server,
) *ServerTransport {
	return &ServerTransport{
		readLimit: int64(maxBufferSize),
		server:    server,
		callbacks: callbacks,
	}
}

func (t *ServerTransport) Name() string { return "webtransport" }

func (t *ServerTransport) QueuedPackets() []*parser.Packet {
	// There's no queue on WebTransport. Packets are directly sent.
	return nil
}

func (t *ServerTransport) Send(packets ...*parser.Packet) {
	for _, packet := range packets {
		err := t.send(packet)
		if err != nil {
			t.close(err)
			break
		}
	}
}

func (t *ServerTransport) send(packet *parser.Packet) error {
	t.sendMu.Lock()
	defer t.sendMu.Unlock()
	return send(t.stream, packet)
}

func (t *ServerTransport) Handshake(_ *parser.Packet, w http.ResponseWriter, r *http.Request) (sid string, err error) {
	t.ctx = r.Context()
	t.conn, err = t.server.Upgrade(w, r)
	if err != nil {
		return "", err
	}
	t.stream, err = t.conn.AcceptStream(t.ctx)
	if err != nil {
		return "", err
	}
	t.limitedReader = newLimitedReader(t.stream, t.readLimit)

	packet, err := t.nextPacket()
	if err != nil {
		return "", err
	}
	if packet.Type != parser.PacketTypeOpen {
		return "", fmt.Errorf("packet type of OPEN expected")
	}
	if len(packet.Data) > 0 {
		data := new(clientOpenPacketData)
		err = json.Unmarshal(packet.Data, &data)
		if err != nil {
			return "", err
		}
		sid = data.SID
	}
	return
}

func (t *ServerTransport) writeHandshakePacket(packet *parser.Packet) error {
	if packet != nil {
		return send(t.stream, packet)
	}
	return nil
}

func (t *ServerTransport) PostHandshake(handshakePacket *parser.Packet) {
	err := t.writeHandshakePacket(handshakePacket)
	if err != nil {
		t.close(err)
		return
	}

	for {
		packet, err := t.nextPacket()
		if err != nil {
			t.close(err)
			return
		}
		t.callbacks.OnPacket(packet)
	}
}

func (t *ServerTransport) nextPacket() (*parser.Packet, error) {
	return nextPacket(t.limitedReader)
}

func (t *ServerTransport) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusBadRequest)
}

func (t *ServerTransport) Discard() {
	t.once.Do(func() {
		if t.stream != nil {
			t.stream.Close()
		}
	})
}

func (t *ServerTransport) close(err error) {
	t.once.Do(func() {
		defer t.callbacks.OnClose(t.Name(), err)

		if t.stream != nil {
			t.stream.Close()
		}
	})
}

func (t *ServerTransport) Close() {
	t.close(nil)
}
