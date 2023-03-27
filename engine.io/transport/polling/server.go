package polling

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/tomruk/socket.io-go/internal/sync"

	"github.com/tomruk/socket.io-go/engine.io/parser"
	"github.com/tomruk/socket.io-go/engine.io/transport"
)

type ServerTransport struct {
	maxHTTPBufferSize int64

	pq          *pollQueue
	pollTimeout time.Duration

	callbacks *transport.Callbacks

	once sync.Once
}

func NewServerTransport(callbacks *transport.Callbacks, maxBufferSize int, pollTimeout time.Duration) *ServerTransport {
	return &ServerTransport{
		maxHTTPBufferSize: int64(maxBufferSize),

		pq:          newPollQueue(),
		pollTimeout: pollTimeout,

		callbacks: callbacks,
	}
}

func (t *ServerTransport) Name() string { return "polling" }

func (t *ServerTransport) Send(packets ...*parser.Packet) {
	t.pq.add(packets...)
}

func (t *ServerTransport) QueuedPackets() []*parser.Packet {
	return t.pq.get()
}

func (t *ServerTransport) Handshake(handshakePacket *parser.Packet, w http.ResponseWriter, r *http.Request) error {
	if handshakePacket != nil {
		t.Send(handshakePacket)
	}
	t.ServeHTTP(w, r)
	return nil
}

func (t *ServerTransport) PostHandshake() {
	// Only for websocket. Do nothing.
}

func (t *ServerTransport) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		t.handlePollRequest(w, r)
	case "POST":
		t.handleDataRequest(w, r)
	}
}

func (t *ServerTransport) setHeaders(w http.ResponseWriter, r *http.Request) {
	wh := w.Header()
	userAgent := r.UserAgent()
	if strings.Contains(userAgent, ";MSIE") || strings.Contains(userAgent, "Trident/") {
		wh.Set("X-XSS-Protection", "0")
	}
}

func (t *ServerTransport) writeJSONPBody(w io.Writer, jsonp string, packets []*parser.Packet) error {
	head := []byte("___eio[" + jsonp + "](\"")
	foot := []byte("\");")

	_, err := w.Write(head)
	if err != nil {
		return err
	}

	buf := bytes.Buffer{}
	buf.Grow(parser.EncodedPayloadsLen(packets...))

	err = parser.EncodePayloads(&buf, packets...)
	if err != nil {
		return err
	}
	template.JSEscape(w, buf.Bytes())

	_, err = w.Write(foot)
	return err
}

func (t *ServerTransport) handlePollRequest(w http.ResponseWriter, r *http.Request) {
	packets := t.pq.poll(t.pollTimeout)

	jsonp := r.URL.Query().Get("j")
	wh := w.Header()
	t.setHeaders(w, r)

	// If this is not a JSON-P request
	if jsonp == "" {
		wh.Set("Content-Type", "text/plain; charset=UTF-8")
		wh.Set("Content-Length", strconv.Itoa(parser.EncodedPayloadsLen(packets...)))
		w.WriteHeader(200)

		err := parser.EncodePayloads(w, packets...)
		if err != nil {
			t.close(err)
			return
		}
	} else {
		buf := bytes.Buffer{}
		err := t.writeJSONPBody(&buf, jsonp, packets)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			t.close(err)
			return
		}

		wh.Set("Content-Type", "text/javascript; charset=UTF-8")
		wh.Set("Content-Length", strconv.Itoa(buf.Len()))
		w.WriteHeader(200)

		_, err = w.Write(buf.Bytes())
		if err != nil {
			t.close(err)
			return
		}
	}
}

var (
	slashReplacer = strings.NewReplacer("\\n", "\n", "\\\\n", "\\n")
	ok            = []byte("ok")
)

func (t *ServerTransport) handleDataRequest(w http.ResponseWriter, r *http.Request) {
	if t.maxHTTPBufferSize > 0 && r.ContentLength > t.maxHTTPBufferSize {
		defer t.close(fmt.Errorf("polling: maxHTTPBufferSize (MaxBufferSize) exceeded"))
		w.WriteHeader(http.StatusBadRequest)
		r.Close = true
		r.Body.Close()
		return
	}

	var (
		packets []*parser.Packet
		jsonp   = r.URL.Query().Get("j")
		err     error
	)

	// If this is not a JSON-P request
	if jsonp == "" {
		packets, err = parser.DecodePayloads(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			t.close(err)
			return
		}
	} else {
		err = r.ParseForm()
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			t.close(err)
			return
		}

		d := r.PostForm.Get("d")
		d = slashReplacer.Replace(d)
		buf := bytes.NewBuffer([]byte(d))

		packets, err = parser.DecodePayloads(buf)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			t.close(err)
			return
		}
	}

	t.callbacks.OnPacket(packets...)

	t.setHeaders(w, r)
	wh := w.Header()

	// text/html is required instead of text/plain to avoid an
	// unwanted download dialog on certain user-agents (GH-43)
	wh.Set("Content-Type", "text/html")
	wh.Set("Content-Length", "2")
	w.WriteHeader(200)
	w.Write(ok)
}

func (t *ServerTransport) Discard() {
	t.once.Do(func() {
		// Send a NOOP packet to force a poll cycle.
		p, err := parser.NewPacket(parser.PacketTypeNoop, false, nil)
		if err == nil {
			go t.Send(p)
		}
	})
}

func (t *ServerTransport) close(err error) {
	t.once.Do(func() {
		defer t.callbacks.OnClose(t.Name(), err)

		p, err := parser.NewPacket(parser.PacketTypeClose, false, nil)
		if err == nil {
			go t.Send(p)
		}
	})
}

func (t *ServerTransport) Close() {
	t.close(nil)
}
