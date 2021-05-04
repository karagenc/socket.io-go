package polling

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/tomruk/socket.io-go/engine.io/parser"
)

type ClientTransport struct {
	sid string

	protocolVersion int
	url             *url.URL
	requestHeader   http.Header

	httpContext context.Context
	httpCancel  context.CancelFunc
	httpClient  *http.Client

	initialPacket *parser.Packet

	callbackMu sync.Mutex
	onPacket   func(p *parser.Packet)
	onClose    func(name string, err error)

	once sync.Once
}

func NewClientTransport(protocolVersion int, url url.URL, requestHeader http.Header, httpClient *http.Client) *ClientTransport {
	ctx, cancel := context.WithCancel(context.Background())

	return &ClientTransport{
		protocolVersion: protocolVersion,
		url:             &url,
		requestHeader:   requestHeader,

		httpContext: ctx,
		httpCancel:  cancel,
		httpClient:  httpClient,
	}
}

func (t *ClientTransport) Name() string {
	return "polling"
}

func (t *ClientTransport) SetCallbacks(onPacket func(p *parser.Packet), onClose func(transportName string, err error)) {
	t.callbackMu.Lock()
	t.onPacket = onPacket
	t.onClose = onClose
	t.callbackMu.Unlock()
}

func (t *ClientTransport) Handshake() (hr *parser.HandshakeResponse, err error) {
	packets, err := t.poll()
	if err != nil {
		return nil, err
	}

	if len(packets) < 1 {
		err = fmt.Errorf("expected at least 1 packet")
		return
	}

	p := packets[0]
	hr, err = parser.ParseHandshakeResponse(p)
	if err != nil {
		return nil, err
	}

	t.sid = hr.SID
	pingInterval := hr.GetPingInterval()
	pingTimeout := hr.GetPingTimeout()

	// If this is a http.Transport, set the timeout
	ht, ok := t.httpClient.Transport.(*http.Transport)
	if ok {
		// Maximum time to wait for a HTTP response.
		ht.ResponseHeaderTimeout = pingInterval + pingTimeout
		// Add a reasonable time (10 seconds) so that even if PollTimeout is reached, we can still read the HTTP response.
		ht.ResponseHeaderTimeout += 10 * time.Second
	}

	if len(packets) == 2 {
		// Save the initial packet. It will be handled later on.
		t.initialPacket = packets[1]
	}

	return
}

func (t *ClientTransport) Run() {
	t.callbackMu.Lock()
	onPacket := t.onPacket
	t.callbackMu.Unlock()

	if t.initialPacket != nil {
		onPacket(t.initialPacket)
		// Set to nil for garbage collection.
		t.initialPacket = nil
	}

	for {
		packets, err := t.poll()
		if err != nil {
			t.close(err)
			break
		}

		t.callbackMu.Lock()
		onPacket := t.onPacket
		t.callbackMu.Unlock()

		for _, p := range packets {
			onPacket(p)
		}
	}
}

func (t *ClientTransport) newRequest(method string, body io.Reader, contentLength int) (*http.Request, error) {
	req, err := http.NewRequestWithContext(t.httpContext, method, t.url.String(), body)
	if err != nil {
		return nil, err
	}

	if body != nil {
		req.Header.Set("Content-Type", "text/plain; charset=UTF-8")
		req.Header.Set("Content-Length", strconv.Itoa(contentLength))
	}

	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Encoding", "gzip")

	for k, v := range t.requestHeader {
		for _, s := range v {
			req.Header.Set(k, s)
		}
	}

	q := req.URL.Query()
	q.Set("transport", "polling")
	q.Set("EIO", strconv.Itoa(t.protocolVersion))

	if t.sid != "" {
		q.Set("sid", t.sid)
	}

	req.URL.RawQuery = q.Encode()
	return req, nil
}

func (t *ClientTransport) poll() ([]*parser.Packet, error) {
	req, err := t.newRequest("GET", nil, 0)
	if err != nil {
		return nil, err
	}

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("non-200 HTTP response received")
	}

	r, err := compressedReader(resp)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	return parser.DecodePayloads(r)
}

func (t *ClientTransport) Send(packets ...*parser.Packet) {
	buf := bytes.Buffer{}
	buf.Grow(parser.EncodedPayloadsLen(packets...))

	err := parser.EncodePayloads(&buf, packets...)
	if err != nil {
		t.close(err)
		return
	}

	req, err := t.newRequest("POST", &buf, buf.Len())
	if err != nil {
		t.close(err)
		return
	}

	resp, err := t.httpClient.Do(req)
	if err != nil {
		t.close(err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.close(fmt.Errorf("non-200 HTTP response received"))
		return
	}

	r, err := compressedReader(resp)
	if err != nil {
		t.close(err)
		return
	}
	defer r.Close()

	// Rapidly read the response body without an heap allocation.
	var respBody [2]byte
	r.Read(respBody[:])

	rWeOk := respBody[0] == 'o' && respBody[1] == 'k'
	if !rWeOk {
		t.close(fmt.Errorf("invalid response received"))
		return
	}
}

func (t *ClientTransport) Discard() {
	t.once.Do(func() {
		t.httpCancel()
	})
}

func (t *ClientTransport) close(err error) {
	t.once.Do(func() {
		t.callbackMu.Lock()
		onClose := t.onClose
		t.callbackMu.Unlock()

		defer onClose(t.Name(), err)
		defer t.httpCancel()

		p, err := parser.NewPacket(parser.PacketTypeClose, false, nil)
		if err == nil {
			go t.Send(p)
		}
	})
}

func (t *ClientTransport) Close() {
	t.close(nil)
}
