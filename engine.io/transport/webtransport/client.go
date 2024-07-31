package webtransport

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/karagenc/socket.io-go/internal/sync"
	"github.com/quic-go/webtransport-go"

	"github.com/karagenc/socket.io-go/engine.io/parser"
	"github.com/karagenc/socket.io-go/engine.io/transport"
)

type ClientTransport struct {
	sid             string
	protocolVersion int
	url             *url.URL
	requestHeader   *transport.RequestHeader

	dialer *webtransport.Dialer
	stream webtransport.Stream
	sendMu sync.Mutex

	callbacks *transport.Callbacks
	once      sync.Once
}

func NewClientTransport(
	callbacks *transport.Callbacks,
	sid string,
	protocolVersion int,
	url url.URL,
	requestHeader *transport.RequestHeader,
	dialer *webtransport.Dialer,
) *ClientTransport {
	return &ClientTransport{
		sid: sid,

		protocolVersion: protocolVersion,
		url:             &url,
		requestHeader:   requestHeader,

		callbacks: callbacks,

		dialer: dialer,
	}
}

func (t *ClientTransport) Name() string { return "webtransport" }

func (t *ClientTransport) Handshake() (hr *parser.HandshakeResponse, err error) {
	switch t.url.Scheme {
	case "wss":
		t.url.Scheme = "https"
	case "ws":
		t.url.Scheme = "http"
	}

	_, session, err := t.dialer.Dial(context.Background(), t.url.String(), t.requestHeader.Header())
	if err != nil {
		return nil, err
	}

	t.stream, err = session.OpenStream()
	if err != nil {
		return nil, err
	}

	var data []byte
	if t.sid != "" {
		data, err = json.Marshal(clientOpenPacketData{
			SID: t.sid,
		})
		if err != nil {
			return nil, err
		}
	}
	packet, err := parser.NewPacket(parser.PacketTypeOpen, false, data)
	if err != nil {
		return nil, err
	}
	err = send(t.stream, packet)
	if err != nil {
		return nil, err
	}

	// If sid is set this means that we have already connected, and
	// we're using this transport for upgrade purposes.
	//
	// If sid is not set, we should receive the OPEN packet and return the values decoded from it.
	if t.sid == "" {
		p, err := t.nextPacket()
		if err != nil {
			return nil, err
		}

		hr, err = parser.ParseHandshakeResponse(p)
		if err != nil {
			return nil, err
		}
		t.sid = hr.SID
	}
	return
}

func (t *ClientTransport) Run() {
	for {
		packet, err := t.nextPacket()
		if err != nil {
			t.close(err)
			return
		}
		t.callbacks.OnPacket(packet)
	}
}

func (t *ClientTransport) nextPacket() (*parser.Packet, error) {
	return nextPacket(t.stream)
}

func (t *ClientTransport) Send(packets ...*parser.Packet) {
	for _, packet := range packets {
		err := t.send(packet)
		if err != nil {
			t.close(err)
			break
		}
	}
}

func (t *ClientTransport) send(packet *parser.Packet) error {
	t.sendMu.Lock()
	defer t.sendMu.Unlock()
	return send(t.stream, packet)
}

func (t *ClientTransport) Discard() {
	t.once.Do(func() {
		if t.stream != nil {
			t.stream.Close()
		}
	})
}

func (t *ClientTransport) close(err error) {
	t.once.Do(func() {
		defer t.callbacks.OnClose(t.Name(), err)

		if t.stream != nil {
			t.stream.Close()
		}
	})
}

func (t *ClientTransport) Close() {
	t.close(nil)
}
