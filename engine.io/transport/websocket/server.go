package websocket

import (
	"context"
	"net/http"
	"sync"

	"github.com/tomruk/socket.io-go/engine.io/parser"
	"github.com/tomruk/socket.io-go/engine.io/transport"
	"nhooyr.io/websocket"
)

type ServerTransport struct {
	readLimit      int64
	supportsBinary bool

	acceptOptions *websocket.AcceptOptions

	conn    *websocket.Conn
	ctx     context.Context
	writeMu sync.Mutex

	callbacks *transport.Callbacks

	once sync.Once
}

func NewServerTransport(
	callbacks *transport.Callbacks,
	maxBufferSize int,
	supportsBinary bool,
	acceptOptions *websocket.AcceptOptions,
) *ServerTransport {
	return &ServerTransport{
		readLimit:      int64(maxBufferSize),
		supportsBinary: supportsBinary,

		callbacks: callbacks,

		acceptOptions: acceptOptions,
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

	for _, packet := range packets {
		err := t.send(packet)
		if err != nil {
			t.close(err)
			break
		}
	}
}

func (t *ServerTransport) send(packet *parser.Packet) error {
	var mt websocket.MessageType
	if packet.IsBinary {
		mt = websocket.MessageBinary
	} else {
		mt = websocket.MessageText
	}

	w, err := t.conn.Writer(t.ctx, mt)
	if err != nil {
		return err
	}
	defer w.Close()
	return packet.Encode(w, true)
}

func (t *ServerTransport) Handshake(handshakePacket *parser.Packet, w http.ResponseWriter, r *http.Request) (err error) {
	t.ctx = r.Context()
	t.conn, err = websocket.Accept(w, r, t.acceptOptions)
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
		w, err := t.conn.Writer(t.ctx, websocket.MessageText)
		if err != nil {
			t.close(err)
			return err
		}
		defer w.Close()

		err = packet.Encode(w, t.supportsBinary)
		if err != nil {
			t.close(err)
			return err
		}
	}
	return nil
}

func (t *ServerTransport) PostHandshake() {
	for {
		mt, r, err := t.conn.Reader(t.ctx)
		if err != nil {
			t.close(err)
			break
		}

		p, err := parser.Decode(r, mt == websocket.MessageBinary)
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
			t.conn.Close(websocket.StatusNormalClosure, "")
		}
	})
}

var expectedCloseCodes = []websocket.StatusCode{
	websocket.StatusNormalClosure,
	websocket.StatusGoingAway,
	websocket.StatusNoStatusRcvd,
	websocket.StatusAbnormalClosure,
}

func (t *ServerTransport) close(err error) {
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

func (t *ServerTransport) Close() {
	t.close(nil)
}
