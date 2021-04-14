package polling

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/tomruk/socket.io-go/engine.io/parser"
)

type ServerTransport struct {
	maxHTTPBufferSize int64

	pq          *pollQueue
	pollTimeout time.Duration

	callbackMu sync.Mutex
	onPacket   func(p *parser.Packet)
	onClose    func(name string, err error)

	once sync.Once
}

func NewServer(maxBufferSize int, pollTimeout time.Duration) *ServerTransport {
	return &ServerTransport{
		maxHTTPBufferSize: int64(maxBufferSize),

		pq:          newPollQueue(),
		pollTimeout: pollTimeout,
	}
}

func (t *ServerTransport) Name() string {
	return "polling"
}

func (t *ServerTransport) SetCallbacks(onPacket func(p *parser.Packet), onClose func(transportName string, err error)) {
	t.callbackMu.Lock()
	t.onPacket = onPacket
	t.onClose = onClose
	t.callbackMu.Unlock()
}

func (t *ServerTransport) SendPacket(p *parser.Packet) {
	t.pq.Add(p)
}

func (t *ServerTransport) QueuedPackets() []*parser.Packet {
	return t.pq.Get()
}

func (t *ServerTransport) Handshake(handshakePacket *parser.Packet, w http.ResponseWriter, r *http.Request) error {
	if handshakePacket != nil {
		t.SendPacket(handshakePacket)
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

func (t *ServerTransport) createJSONPBody(jsonp string, body []byte) []byte {
	head := "___eio[" + jsonp + "](\""
	foot := "\");"
	js := bytes.Buffer{}
	js.Grow(len(head) + len(body) + len(foot))

	js.WriteString(head)
	template.JSEscape(&js, body)
	js.WriteString(foot)

	return js.Bytes()
}

func (t *ServerTransport) handlePollRequest(w http.ResponseWriter, r *http.Request) {
	packets := t.pq.Poll(t.pollTimeout)

	body := parser.EncodePayloads(packets...)
	jsonp := r.URL.Query().Get("j")
	wh := w.Header()
	t.setHeaders(w, r)

	// If this is not a JSON-P request
	if jsonp == "" {
		wh.Set("Content-Type", "text/plain; charset=UTF-8")
		wh.Set("Content-Length", strconv.Itoa(len(body)))
		w.WriteHeader(200)
		w.Write(body)
	} else {
		body = t.createJSONPBody(jsonp, body)

		wh.Set("Content-Type", "text/javascript; charset=UTF-8")
		wh.Set("Content-Length", strconv.Itoa(len(body)))
		w.WriteHeader(200)
		w.Write(body)
	}
}

var (
	slashReplacer = strings.NewReplacer("\\n", "\n", "\\\\n", "\\n")
	ok            = []byte("ok")
)

const maxBufferSizeExceeded = "maxHTTPBufferSize (MaxBufferSize) exceeded"

func (t *ServerTransport) handleDataRequest(w http.ResponseWriter, r *http.Request) {
	if t.maxHTTPBufferSize > 0 && r.ContentLength > t.maxHTTPBufferSize {
		defer t.close(fmt.Errorf(maxBufferSizeExceeded))
		http.Error(w, maxBufferSizeExceeded, http.StatusBadRequest)
		r.Close = true
		r.Body.Close()
		return
	}

	var (
		jsonp = r.URL.Query().Get("j")
		data  []byte
		err   error
	)

	// If this is not a JSON-P request
	if jsonp == "" {
		data, err = io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
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
		data = []byte(slashReplacer.Replace(d))
	}

	packets, err := parser.DecodePayloads(data)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		t.close(err)
		return
	}

	t.callbackMu.Lock()
	onPacket := t.onPacket
	t.callbackMu.Unlock()

	for _, p := range packets {
		onPacket(p)
	}

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
			go t.SendPacket(p)
		}
	})
}

func (t *ServerTransport) close(err error) {
	t.once.Do(func() {
		t.callbackMu.Lock()
		onClose := t.onClose
		t.callbackMu.Unlock()

		defer onClose(t.Name(), err)

		p, err := parser.NewPacket(parser.PacketTypeClose, false, nil)
		if err == nil {
			go t.SendPacket(p)
		}
	})
}

func (t *ServerTransport) Close() {
	t.close(nil)
}
