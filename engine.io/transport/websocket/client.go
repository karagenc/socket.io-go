package websocket

import (
	"net/http"
	"net/url"
	"strconv"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/tomruk/socket.io-go/engine.io/parser"
)

type ClientTransport struct {
	sid string

	protocolVersion int
	url             *url.URL
	requestHeader   http.Header

	dialer  *websocket.Dialer
	conn    *websocket.Conn
	writeMu sync.Mutex

	callbackMu sync.Mutex
	onPacket   func(p *parser.Packet)
	onClose    func(name string, err error)

	once sync.Once
}

func NewClientTransport(sid string, protocolVersion int, url url.URL, requestHeader http.Header, dialer *websocket.Dialer) *ClientTransport {
	return &ClientTransport{
		sid: sid,

		protocolVersion: protocolVersion,
		url:             &url,
		requestHeader:   requestHeader,

		dialer: dialer,
	}
}

func (t *ClientTransport) Name() string {
	return "websocket"
}

func (t *ClientTransport) SetCallbacks(onPacket func(p *parser.Packet), onClose func(transportName string, err error)) {
	t.callbackMu.Lock()
	t.onPacket = onPacket
	t.onClose = onClose
	t.callbackMu.Unlock()
}

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

	t.conn, _, err = t.dialer.Dial(t.url.String(), t.requestHeader)
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

		t.callbackMu.Lock()
		onPacket := t.onPacket
		t.callbackMu.Unlock()
		onPacket(p)
	}
}

func (t *ClientTransport) nextPacket() (*parser.Packet, error) {
	mt, b, err := t.conn.ReadMessage()
	if err != nil {
		return nil, err
	}
	return parser.Parse(b, mt == websocket.BinaryMessage)
}

func (t *ClientTransport) SendPacket(p *parser.Packet) {
	var (
		mt   int
		data = p.Build(true)
	)

	if p.IsBinary {
		mt = websocket.BinaryMessage
	} else {
		mt = websocket.TextMessage
	}

	// WriteMessage must not be called concurrently.
	t.writeMu.Lock()
	defer t.writeMu.Unlock()

	err := t.conn.WriteMessage(mt, data)
	if err != nil {
		t.close(err)
	}
}

func (t *ClientTransport) Discard() {
	t.once.Do(func() {
		if t.conn != nil {
			t.conn.Close()
		}
	})
}

func (t *ClientTransport) close(err error) {
	t.once.Do(func() {
		if websocket.IsCloseError(err, expectedCloseCodes...) {
			err = nil
		}

		t.callbackMu.Lock()
		onClose := t.onClose
		t.callbackMu.Unlock()

		defer onClose(t.Name(), err)

		if t.conn != nil {
			t.conn.Close()
		}
	})
}

func (t *ClientTransport) Close() {
	t.close(nil)
}
