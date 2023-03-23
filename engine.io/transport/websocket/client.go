package websocket

import (
	"context"
	"net/url"
	"strconv"
	"sync"

	"github.com/tomruk/socket.io-go/engine.io/parser"
	"github.com/tomruk/socket.io-go/engine.io/transport"
	"nhooyr.io/websocket"
)

type ClientTransport struct {
	sid string

	protocolVersion int
	url             *url.URL
	requestHeader   *transport.RequestHeader

	dialOptions *websocket.DialOptions
	conn        *websocket.Conn
	writeMu     sync.Mutex

	callbacks *transport.Callbacks

	once sync.Once
}

func NewClientTransport(
	callbacks *transport.Callbacks,
	sid string,
	protocolVersion int,
	url url.URL,
	requestHeader *transport.RequestHeader,
	dialOptions *websocket.DialOptions,
) *ClientTransport {
	return &ClientTransport{
		sid: sid,

		protocolVersion: protocolVersion,
		url:             &url,
		requestHeader:   requestHeader,

		callbacks: callbacks,

		dialOptions: dialOptions,
	}
}

func (t *ClientTransport) Name() string { return "websocket" }

func (t *ClientTransport) Handshake() (hr *parser.HandshakeResponse, err error) {
	q := t.url.Query()
	q.Set("transport", "websocket")
	q.Set("EIO", strconv.Itoa(t.protocolVersion))

	if t.sid != "" {
		q.Set("sid", t.sid)
	}

	t.url.RawQuery = q.Encode()

	switch t.url.Scheme {
	case "https":
		t.url.Scheme = "wss"
	case "http":
		t.url.Scheme = "ws"
	}

	t.conn, _, err = websocket.Dial(context.Background(), t.url.String(), t.dialOptions)
	if err != nil {
		return
	}

	// If sid is set this means that we have already connected and
	// we're using this transport for upgrade purposes.

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
		p, err := t.nextPacket()
		if err != nil {
			t.close(err)
			return
		}

		t.callbacks.OnPacket(p)
	}
}

func (t *ClientTransport) nextPacket() (*parser.Packet, error) {
	mt, r, err := t.conn.Reader(context.Background())
	if err != nil {
		return nil, err
	}
	return parser.Decode(r, mt == websocket.MessageBinary)
}

func (t *ClientTransport) Send(packets ...*parser.Packet) {
	// WriteMessage must not be called concurrently.
	t.writeMu.Lock()
	defer t.writeMu.Unlock()

	for _, packet := range packets {
		err := t.send(packet)
		if err != nil {
			t.close(err)
			break
		}
	}
}

func (t *ClientTransport) send(packet *parser.Packet) error {
	var mt websocket.MessageType
	if packet.IsBinary {
		mt = websocket.MessageBinary
	} else {
		mt = websocket.MessageText
	}

	w, err := t.conn.Writer(context.Background(), mt)
	if err != nil {
		return err
	}
	defer w.Close()
	return packet.Encode(w, true)
}

func (t *ClientTransport) Discard() {
	t.once.Do(func() {
		if t.conn != nil {
			t.conn.Close(websocket.StatusNormalClosure, "")
		}
	})
}

func (t *ClientTransport) close(err error) {
	t.once.Do(func() {
		status := websocket.CloseStatus(err)
		if status == -1 {
			err = nil
		}
		for _, expected := range expectedCloseCodes {
			if status == expected {
				err = nil
				break
			}
		}

		defer t.callbacks.OnClose(t.Name(), err)

		if t.conn != nil {
			t.conn.Close(websocket.StatusNormalClosure, "")
		}
	})
}

func (t *ClientTransport) Close() {
	t.close(nil)
}
