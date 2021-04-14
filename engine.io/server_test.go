package eio

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/tomruk/socket.io-go/engine.io/parser"
	"github.com/tomruk/socket.io-go/internal/json"
)

// This is a sync.WaitGroup with a WaitTimeout function. Use this for testing purposes.
type testWaiter struct {
	wg *sync.WaitGroup
}

func newTestWaiter(delta int) *testWaiter {
	wg := new(sync.WaitGroup)
	wg.Add(delta)
	return &testWaiter{
		wg: wg,
	}
}

func (w *testWaiter) Add(delta int) {
	w.wg.Add(delta)
}

func (w *testWaiter) Done() {
	w.wg.Done()
}

func (w *testWaiter) Wait() {
	w.wg.Wait()
}

func (w *testWaiter) WaitTimeout(t *testing.T, timeout time.Duration) (timedout bool) {
	c := make(chan struct{})

	go func() {
		defer close(c)
		w.wg.Wait()
	}()

	select {
	case <-c:
		return false
	case <-time.After(timeout):
		t.Error("timeout exceeded")
		return true
	}
}

type fakeServerTransport struct{}

var _ ServerTransport = newFakeServerTransport()

func newFakeServerTransport() *fakeServerTransport {
	return new(fakeServerTransport)
}

func (t *fakeServerTransport) Name() string {
	return "fake"
}

func (t *fakeServerTransport) Handshake(handshakePacket *parser.Packet, w http.ResponseWriter, r *http.Request) error {
	return nil
}

func (t *fakeServerTransport) SetCallbacks(onPacket func(p *parser.Packet), onClose func(transportName string, err error)) {

}

func (t *fakeServerTransport) PostHandshake() {}

func (t *fakeServerTransport) ServeHTTP(w http.ResponseWriter, r *http.Request) {}

func (t *fakeServerTransport) QueuedPackets() []*parser.Packet { return nil }

func (t *fakeServerTransport) SendPacket(p *parser.Packet) {}

func (t *fakeServerTransport) Discard() {}
func (t *fakeServerTransport) Close()   {}

func TestServerErrors(t *testing.T) {
	for i, e1 := range serverErrors {
		e2, ok := serverErrors[i]
		if !ok {
			t.Fatal("serverErrors[i] should be set")
		}
		assert.Equal(t, e1, e2)
		assert.Equal(t, i, e1.Code)
	}
}

func TestInvalidEIOVersion(t *testing.T) {
	io := NewServer(nil, nil)

	err := io.Run()
	if err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}

	q := req.URL.Query()
	q.Add("EIO", "523523") // Random value
	req.URL.RawQuery = q.Encode()

	io.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)

	e := new(ServerError)
	err = json.Unmarshal(rec.Body.Bytes(), e)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, serverErrors[ErrorUnsupportedProtocolVersion].Code, e.Code)
	assert.Equal(t, serverErrors[ErrorUnsupportedProtocolVersion].Message, e.Message)
}

func TestUnknownTransport(t *testing.T) {
	io := NewServer(nil, nil)

	err := io.Run()
	if err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}

	const fakeTransportName = "UFO"

	q := req.URL.Query()
	q.Add("EIO", strconv.Itoa(ProtocolVersion))
	q.Add("transport", fakeTransportName) // There's no such transport
	req.URL.RawQuery = q.Encode()

	io.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)

	e := new(ServerError)
	err = json.Unmarshal(rec.Body.Bytes(), e)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, serverErrors[ErrorUnknownTransport].Code, e.Code)
	assert.Equal(t, serverErrors[ErrorUnknownTransport].Message, e.Message)
}

func TestUnknownSID(t *testing.T) {
	io := NewServer(nil, nil)

	err := io.Run()
	if err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}

	q := req.URL.Query()
	q.Add("EIO", strconv.Itoa(ProtocolVersion))
	q.Add("sid", "dsaaskmsdkfakfasfjmsaklfam") // Random SID
	req.URL.RawQuery = q.Encode()

	io.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)

	e := new(ServerError)
	err = json.Unmarshal(rec.Body.Bytes(), e)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, serverErrors[ErrorUnknownSID].Code, e.Code)
	assert.Equal(t, serverErrors[ErrorUnknownSID].Message, e.Message)
}

func TestBadHandshakeMethod(t *testing.T) {
	io := NewServer(nil, nil)

	err := io.Run()
	if err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	req, err := http.NewRequest("POST", "/", nil)
	if err != nil {
		t.Fatal(err)
	}

	q := req.URL.Query()
	q.Add("EIO", strconv.Itoa(ProtocolVersion))
	q.Add("transport", "polling")
	req.URL.RawQuery = q.Encode()

	io.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)

	e := new(ServerError)
	err = json.Unmarshal(rec.Body.Bytes(), e)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, serverErrors[ErrorBadHandshakeMethod].Code, e.Code)
	assert.Equal(t, serverErrors[ErrorBadHandshakeMethod].Message, e.Message)
}

func TestAuthenticator(t *testing.T) {
	authenticator := func(w http.ResponseWriter, r *http.Request) (ok bool) {
		// Fail.
		return false
	}

	io := NewServer(nil, &ServerConfig{
		Authenticator: authenticator,
	})

	err := io.Run()
	if err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}

	q := req.URL.Query()
	q.Add("EIO", strconv.Itoa(ProtocolVersion))
	q.Add("transport", "polling")
	req.URL.RawQuery = q.Encode()

	io.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestMaxBufferSizeWebSocket(t *testing.T) {
	tw := newTestWaiter(2) // Wait for the server and client.

	onSocket := func(socket Socket) *Callbacks {
		return &Callbacks{
			OnClose: func(reason string, err error) {
				defer tw.Done()

				if reason != ReasonTransportError || !errors.Is(err, websocket.ErrReadLimit) {
					t.Error("exceeding the MaxBufferSize should've caused a transport error and an ErrReadLimit error")
				}
			},
		}
	}

	io := NewServer(onSocket, &ServerConfig{
		MaxBufferSize: 5,
	})

	err := io.Run()
	if err != nil {
		t.Fatal(err)
	}

	s := httptest.NewServer(io)

	callbacks := &Callbacks{
		OnClose: func(reason string, err error) {
			defer tw.Done()

			if reason != ReasonTransportError || !websocket.IsCloseError(err, websocket.CloseMessageTooBig) {
				t.Error("exceeding the MaxBufferSize should've caused a transport error and the close code should've been 1009")
			}
		},
	}

	socket := testDial(t, s.URL, callbacks, &ClientConfig{
		Transports: []string{"websocket"},
	})

	assert.Equal(t, "websocket", socket.TransportName())

	socket.SendMessage([]byte("123456"), false)

	tw.WaitTimeout(t, 3*time.Second)
}

func TestMaxBufferSizePolling(t *testing.T) {
	tw := newTestWaiter(2) // Wait for the server and client.

	onSocket := func(socket Socket) *Callbacks {
		return &Callbacks{
			OnClose: func(reason string, err error) {
				defer tw.Done()

				if reason != ReasonTransportError || err == nil {
					t.Error("exceeding the MaxBufferSize should've caused a transport error and the err should be non-nil")
				}
			},
		}
	}

	io := NewServer(onSocket, &ServerConfig{
		MaxBufferSize: 5,
	})

	err := io.Run()
	if err != nil {
		t.Fatal(err)
	}

	s := httptest.NewServer(io)

	callbacks := &Callbacks{
		OnClose: func(reason string, err error) {
			defer tw.Done()

			if reason != ReasonTransportError || err == nil {
				t.Error("exceeding the MaxBufferSize should've caused a transport error and the err should be non-nil")
			}
		},
	}

	socket := testDial(t, s.URL, callbacks, &ClientConfig{
		Transports: []string{"polling"},
	})

	assert.Equal(t, "polling", socket.TransportName())

	socket.SendMessage([]byte("123456"), false)

	tw.WaitTimeout(t, 3*time.Second)
}

func TestDisableMaxBufferSizeWebSocket(t *testing.T) {
	tw := newTestWaiter(1) // Wait for the server.

	testData := []byte("12345678")

	onSocket := func(socket Socket) *Callbacks {
		return &Callbacks{
			OnMessage: func(data []byte, isBinary bool) {
				defer tw.Done()

				if !bytes.Equal(testData, data) {
					t.Error("data doesn't match")
				}
			},
		}
	}

	io := NewServer(onSocket, &ServerConfig{
		MaxBufferSize:        5,
		DisableMaxBufferSize: true,
	})

	err := io.Run()
	if err != nil {
		t.Fatal(err)
	}

	s := httptest.NewServer(io)

	socket := testDial(t, s.URL, nil, &ClientConfig{
		Transports: []string{"websocket"},
	})

	assert.Equal(t, "websocket", socket.TransportName())

	socket.SendMessage(testData, false)

	tw.WaitTimeout(t, 3*time.Second)
}

func TestDisableMaxBufferSizePolling(t *testing.T) {
	tw := newTestWaiter(1) // Wait for the server.

	testData := []byte("12345678")

	onSocket := func(socket Socket) *Callbacks {
		return &Callbacks{
			OnMessage: func(data []byte, isBinary bool) {
				defer tw.Done()

				if !bytes.Equal(testData, data) {
					t.Error("data doesn't match")
				}
			},
		}
	}

	io := NewServer(onSocket, &ServerConfig{
		MaxBufferSize:        5,
		DisableMaxBufferSize: true,
	})

	err := io.Run()
	if err != nil {
		t.Fatal(err)
	}

	s := httptest.NewServer(io)

	socket := testDial(t, s.URL, nil, &ClientConfig{
		Transports: []string{"polling"},
	})

	assert.Equal(t, "polling", socket.TransportName())

	socket.SendMessage(testData, false)

	tw.WaitTimeout(t, 3*time.Second)
}

func TestJSONP(t *testing.T) {
	tw := newTestWaiter(2)

	const (
		pingInterval = 123456 * time.Second
		pingTimeout  = 654321 * time.Second
	)

	var (
		testMessage1 = []byte("Hello from server")
		testMessage2 = []byte{0x1, 0x2, 0x3}
	)

	onSocket := func(socket Socket) *Callbacks {
		socket.SendMessage(testMessage1, false)
		socket.SendMessage(testMessage2, true)

		return &Callbacks{
			OnMessage: func(data []byte, isBinary bool) {
				switch {
				case bytes.Equal(data, testMessage1) && isBinary == false:
					tw.Done()
				case bytes.Equal(data, testMessage2) && isBinary == true:
					tw.Done()
				default:
					t.Error("invalid message received")
				}
			},
		}
	}

	io := NewServer(onSocket, &ServerConfig{
		PingInterval: pingInterval,
		PingTimeout:  pingTimeout,
	})

	err := io.Run()
	if err != nil {
		t.Fatal(err)
	}

	// Test handshake

	rec := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}

	const jsonp = "21"

	q := req.URL.Query()
	q.Add("EIO", strconv.Itoa(ProtocolVersion))
	q.Add("transport", "polling")
	q.Add("j", jsonp)
	req.URL.RawQuery = q.Encode()

	io.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatal("non-200 response received")
	}

	if rec.Header().Get("Content-Type") != "text/javascript; charset=UTF-8" {
		t.Fatal("invalid Content-Type")
	}

	body := rec.Body.String()

	head := "___eio[" + jsonp + "](\""
	foot := "\");"

	if !strings.HasPrefix(body, head) {
		t.Fatal("invalid JSON-P head")
	}
	if !strings.HasSuffix(body, foot) {
		t.Fatal("invalid JSON-P foot")
	}

	body = strings.TrimPrefix(body, head)
	body = strings.TrimSuffix(body, foot)

	body = strings.ReplaceAll(body, "\\\"", "\"")

	p, err := parser.Parse([]byte(body), false)
	if err != nil {
		t.Fatal(err)
	}

	hr := new(parser.HandshakeResponse)
	err = json.Unmarshal(p.Data, hr)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, pingInterval, hr.GetPingInterval())
	assert.Equal(t, pingTimeout, hr.GetPingTimeout())

	sid := hr.SID

	// Test receiving packets from server

	rec = httptest.NewRecorder()
	req, err = http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}

	q = req.URL.Query()
	q.Add("sid", sid)
	q.Add("EIO", strconv.Itoa(ProtocolVersion))
	q.Add("transport", "polling")
	q.Add("j", jsonp)
	req.URL.RawQuery = q.Encode()

	io.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatal("non-200 response received")
	}

	if rec.Header().Get("Content-Type") != "text/javascript; charset=UTF-8" {
		t.Fatal("invalid Content-Type")
	}

	body = rec.Body.String()

	if !strings.HasPrefix(body, head) {
		t.Fatal("invalid JSON-P head")
	}
	if !strings.HasSuffix(body, foot) {
		t.Fatal("invalid JSON-P foot")
	}

	body = strings.TrimPrefix(body, head)
	body = strings.TrimSuffix(body, foot)

	body = strings.ReplaceAll(body, "\\\"", "\"")

	splitted := strings.Split(body, "\\u001E")

	if len(splitted) != 2 {
		t.Fatal("invalid response body")
	}

	p1, err := parser.Parse([]byte(splitted[0]), false)
	if err != nil {
		t.Fatal(err)
	}

	p2, err := parser.Parse([]byte(splitted[1]), false)
	if err != nil {
		t.Fatal(err)
	}

	if !p2.IsBinary {
		t.Fatal("second packet should be a binary packet")
	}

	if !bytes.Equal(p1.Data, testMessage1) {
		t.Fatal("data doesn't match")
	}

	if !bytes.Equal(p2.Data, testMessage2) {
		t.Fatal("data doesn't match")
	}

	// Test sending packets to server

	p1, err = parser.NewPacket(parser.PacketTypeMessage, false, testMessage1)
	if err != nil {
		t.Fatal(err)
	}

	p2, err = parser.NewPacket(parser.PacketTypeMessage, true, testMessage2)
	if err != nil {
		t.Fatal(err)
	}

	d := string(parser.EncodePayloads(p1, p2))
	d = url.QueryEscape(d)
	d = "d=" + d

	postForm := bytes.NewBuffer([]byte(d))

	rec = httptest.NewRecorder()
	req, err = http.NewRequest("POST", "/", postForm)
	if err != nil {
		t.Fatal(err)
	}

	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	q = req.URL.Query()
	q.Add("sid", sid)
	q.Add("EIO", strconv.Itoa(ProtocolVersion))
	q.Add("transport", "polling")
	q.Add("j", jsonp)
	req.URL.RawQuery = q.Encode()

	io.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatal("non-200 response received")
	}

	if rec.Header().Get("Content-Type") != "text/html" {
		t.Fatal("invalid Content-Type")
	}

	body = rec.Body.String()
	if body != "ok" {
		t.Fatal("ok expected")
	}

	tw.WaitTimeout(t, time.Second*1)
}

func TestServerClose(t *testing.T) {
	tw := newTestWaiter(0)
	utw := newTestWaiter(0) // For upgrades.

	onSocket := func(socket Socket) *Callbacks {
		socket.SendMessage(nil, false)

		return &Callbacks{
			OnClose: func(reason string, err error) {
				defer tw.Done()

				if reason != ReasonForcedClose {
					t.Errorf("server: expected reason: %s, but got: %s", ReasonForcedClose, reason)
				}

				if err != nil {
					t.Errorf("server: err should be nil. Error: %v", err)
				}
			},
		}
	}

	io := NewServer(onSocket, nil)

	err := io.Run()
	if err != nil {
		t.Fatal(err)
	}

	s := httptest.NewServer(io)

	transportsToTest := [][]string{
		{"polling"},
		{"websocket"},
		{"polling", "websocket"},
	}

	for _, transports := range transportsToTest {
		tw.Add(2)

		callbacks := &Callbacks{
			OnClose: func(reason string, err error) {
				defer tw.Done()

				if reason != ReasonTransportClose {
					t.Errorf("client: expected reason: %s, but got: %s", ReasonTransportClose, reason)
				}

				if err != nil {
					t.Errorf("client: err should be nil. Error: %v", err)
				}
			},
		}

		if len(transports) > 1 {
			utw.Add(1)
		}

		upgradeDone := func(transportName string) {
			utw.Done()
		}

		testDial(t, s.URL, callbacks, &ClientConfig{Transports: transports, UpgradeDone: upgradeDone})
	}

	// Wait for upgrades to finish.
	timedout := utw.WaitTimeout(t, time.Second*3)
	if timedout {
		t.Fatal("upgrades couldn't finish")
	}

	err = io.Close()
	if err != nil {
		t.Fatal(err)
	}

	req, err := http.NewRequest("GET", s.URL, nil)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := s.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	assert.Equal(t, http.StatusTeapot, resp.StatusCode, "server should have been closed")

	tw.WaitTimeout(t, time.Second*3)
}
