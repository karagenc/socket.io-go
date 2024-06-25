package websocket

import (
	"context"
	"net/http"

	"github.com/tomruk/socket.io-go/internal/sync"

	"github.com/tomruk/socket.io-go/engine.io/parser"
	"github.com/tomruk/socket.io-go/engine.io/transport"
	"nhooyr.io/websocket"
)

type ServerTransport struct {
	readLimit      int64
	supportsBinary bool
	acceptOptions  *websocket.AcceptOptions

	ctx  context.Context
	conn *websocket.Conn

	callbacks *transport.Callbacks
	once      sync.Once
}

func NewServerTransport(
	callbacks *transport.Callbacks,
	maxBufferSize int64,
	supportsBinary bool,
	acceptOptions *websocket.AcceptOptions,
) *ServerTransport {
	return &ServerTransport{
		readLimit:      maxBufferSize,
		supportsBinary: supportsBinary,
		callbacks:      callbacks,
		acceptOptions:  acceptOptions,
	}
}

func (t *ServerTransport) Name() string { return "websocket" }

func (t *ServerTransport) QueuedPackets() []*parser.Packet {
	// There's no queue on WebSocket. Packets are directly sent.
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

func (t *ServerTransport) Handshake(handshakePacket *parser.Packet, w http.ResponseWriter, r *http.Request) (sid string, err error) {
	t.ctx = r.Context()
	t.conn, err = websocket.Accept(w, r, t.acceptOptions)
	if err != nil {
		return
	}
	if t.readLimit != 0 {
		t.conn.SetReadLimit(t.readLimit)
	}
	// sid is only for webtransport
	return "", t.writeHandshakePacket(handshakePacket)
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

func (t *ServerTransport) PostHandshake(_ *parser.Packet) {
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
	mt, r, err := t.conn.Reader(t.ctx)
	if err != nil {
		return nil, err
	}
	return parser.Decode(r, mt == websocket.MessageBinary)
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
