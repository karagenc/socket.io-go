package websocket

import (
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/tomruk/socket.io-go/engine.io/parser"
	"github.com/tomruk/socket.io-go/engine.io/transport"
)

type ServerTransport struct {
	readLimit      int64
	supportsBinary bool

	upgrader *websocket.Upgrader
	conn     *websocket.Conn
	writeMu  sync.Mutex

	callbacks *transport.Callbacks

	once sync.Once
}

func NewServerTransport(callbacks *transport.Callbacks, maxBufferSize int, supportsBinary bool, upgrader *websocket.Upgrader) *ServerTransport {
	return &ServerTransport{
		readLimit:      int64(maxBufferSize),
		supportsBinary: supportsBinary,

		callbacks: callbacks,

		upgrader: upgrader,
	}
}

func (t *ServerTransport) Name() string {
	return "websocket"
}

func (t *ServerTransport) Callbacks() *transport.Callbacks {
	return t.callbacks
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
		var mt int
		if p.IsBinary {
			mt = websocket.BinaryMessage
		} else {
			mt = websocket.TextMessage
		}

		w, err := t.conn.NextWriter(mt)
		if err != nil {
			t.close(err)
			break
		}

		err = p.Encode(w, true)
		if err != nil {
			t.close(err)
			break
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

	return t.writeHandshakePacket(handshakePacket)
}

func (t *ServerTransport) writeHandshakePacket(packet *parser.Packet) error {
	if packet != nil {
		w, err := t.conn.NextWriter(websocket.TextMessage)
		if err != nil {
			t.conn.Close()
			return err
		}

		err = packet.Encode(w, t.supportsBinary)
		if err != nil {
			t.conn.Close()
			return err
		}
	}
	return nil
}

func (t *ServerTransport) PostHandshake() {
	for {
		mt, r, err := t.conn.NextReader()
		if err != nil {
			t.close(err)
			break
		}

		p, err := parser.Decode(r, mt == websocket.BinaryMessage)
		if err != nil {
			t.close(err)
			break
		}

		t.callbacks.OnPacket(p)
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

		defer t.callbacks.OnClose(t.Name(), err)

		if t.conn != nil {
			t.conn.Close()
		}
	})
}

func (t *ServerTransport) Close() {
	t.close(nil)
}
