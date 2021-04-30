package websocket

import (
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/tomruk/socket.io-go/engine.io/parser"
)

type ServerTransport struct {
	readLimit      int64
	supportsBinary bool

	upgrader *websocket.Upgrader
	conn     *websocket.Conn
	writeMu  sync.Mutex

	callbackMu sync.Mutex
	onPacket   func(p *parser.Packet)
	onClose    func(name string, err error)

	once sync.Once
}

func NewServerTransport(maxBufferSize int, supportsBinary bool, upgrader *websocket.Upgrader) *ServerTransport {
	return &ServerTransport{
		readLimit:      int64(maxBufferSize),
		supportsBinary: supportsBinary,

		upgrader: upgrader,
	}
}

func (t *ServerTransport) Name() string {
	return "websocket"
}

func (t *ServerTransport) SetCallbacks(onPacket func(p *parser.Packet), onClose func(transportName string, err error)) {
	t.callbackMu.Lock()
	t.onPacket = onPacket
	t.onClose = onClose
	t.callbackMu.Unlock()
}

func (t *ServerTransport) QueuedPackets() []*parser.Packet {
	// There's no queue on WebSocket. Packets are directly sent.
	return nil
}

func (t *ServerTransport) Send(packets ...*parser.Packet) {
	// WriteMessage must not be called concurrently.
	t.writeMu.Lock()
	defer t.writeMu.Unlock()

	for _, p := range packets {
		var (
			mt   int
			data = p.Build(t.supportsBinary)
		)

		if p.IsBinary {
			mt = websocket.BinaryMessage
		} else {
			mt = websocket.TextMessage
		}

		err := t.conn.WriteMessage(mt, data)
		if err != nil {
			t.close(err)
		}
	}
}

func (t *ServerTransport) Handshake(handshakePacket *parser.Packet, w http.ResponseWriter, r *http.Request) (err error) {
	t.conn, err = t.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	if t.readLimit != 0 {
		t.conn.SetReadLimit(t.readLimit)
	}

	if handshakePacket != nil {
		built := handshakePacket.Build(t.supportsBinary)
		err = t.conn.WriteMessage(websocket.TextMessage, built)
		if err != nil {
			t.conn.Close()
			return
		}
	}
	return
}

func (t *ServerTransport) PostHandshake() {
	for {
		mt, data, err := t.conn.ReadMessage()
		if err != nil {
			t.close(err)
			return
		}

		p, err := parser.Parse(data, mt == websocket.BinaryMessage)
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

func (t *ServerTransport) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusBadRequest)
}

func (t *ServerTransport) Discard() {
	t.once.Do(func() {
		if t.conn != nil {
			t.conn.Close()
		}
	})
}

var expectedCloseCodes = []int{
	websocket.CloseNormalClosure,
	websocket.CloseGoingAway,
	websocket.CloseNoStatusReceived,
	websocket.CloseAbnormalClosure,
}

func (t *ServerTransport) close(err error) {
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

func (t *ServerTransport) Close() {
	t.close(nil)
}
