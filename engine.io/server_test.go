package eio

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tomruk/socket.io-go/engine.io/parser"
)

func TestServer(t *testing.T) {
	t.Run("should emit error if `UpgradeTimeout` is set and is exceeded", func(t *testing.T) {
		tw := NewTestWaiter(2)

		onSocket := func(socket ServerSocket) *Callbacks {
			return &Callbacks{
				OnError: func(err error) {
					defer tw.Done()
					require.Error(t, err)
				},
				OnPacket: func(packets ...*parser.Packet) {
					defer tw.Done()
					// Receive packet as normal while upgrading.
					require.Equal(t, 1, len(packets))
					packet := packets[0]
					require.Equal(t, packet.Type, parser.PacketTypeMessage)
					require.Equal(t, packet.Data, []byte("123456"))
				},
			}
		}

		io := newTestServer(onSocket, &ServerConfig{
			UpgradeTimeout: 1 * time.Second,
		}, nil)
		err := io.Run()
		if err != nil {
			t.Fatal(err)
		}
		ts := httptest.NewServer(io)

		socket := testDial(t, ts.URL, nil, &ClientConfig{
			UpgradeDone: func(transportName string) {
				t.Fatalf("transport upgraded to: %s", transportName)
			},
		}, &testDialOptions{
			testWaitUpgrade: true,
		})
		require.Equal(t, "polling", socket.TransportName())

		packet := mustCreatePacket(t, parser.PacketTypeMessage, false, []byte("123456"))
		socket.Send(packet)

		tw.WaitTimeout(t, DefaultTestWaitTimeout)
	})

	t.Run("map key of `serverErrors` should be equal to error code", func(t *testing.T) {
		for j, e1 := range serverErrors {
			e2, ok := serverErrors[j]
			if !ok {
				t.Fatal("serverErrors[j] should be set")
			}
			require.Equal(t, e1, e2)
			require.Equal(t, j, e1.Code)
		}
	})

	t.Run("should fail with invalid Engine.IO version", func(t *testing.T) {
		io := newTestServer(nil, nil, nil)
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
		require.Equal(t, http.StatusBadRequest, rec.Code)

		e := new(ServerError)
		err = json.Unmarshal(rec.Body.Bytes(), e)
		if err != nil {
			t.Fatal(err)
		}
		require.Equal(t, serverErrors[ErrorUnsupportedProtocolVersion].Code, e.Code)
		require.Equal(t, serverErrors[ErrorUnsupportedProtocolVersion].Message, e.Message)
	})

	t.Run("should fail with unknown transport name", func(t *testing.T) {
		io := newTestServer(nil, nil, nil)
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
		q.Add("transport", "labalubadabaluba") // There's no such transport
		req.URL.RawQuery = q.Encode()
		io.ServeHTTP(rec, req)
		require.Equal(t, http.StatusBadRequest, rec.Code)

		e := new(ServerError)
		err = json.Unmarshal(rec.Body.Bytes(), e)
		if err != nil {
			t.Fatal(err)
		}
		require.Equal(t, serverErrors[ErrorUnknownTransport].Code, e.Code)
		require.Equal(t, serverErrors[ErrorUnknownTransport].Message, e.Message)
	})

	t.Run("should fail with unknown SID", func(t *testing.T) {
		io := newTestServer(nil, nil, nil)
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
		require.Equal(t, http.StatusBadRequest, rec.Code)

		e := new(ServerError)
		err = json.Unmarshal(rec.Body.Bytes(), e)
		if err != nil {
			t.Fatal(err)
		}

		require.Equal(t, serverErrors[ErrorUnknownSID].Code, e.Code)
		require.Equal(t, serverErrors[ErrorUnknownSID].Message, e.Message)
	})

	t.Run("should fail when request is made with an invalid method", func(t *testing.T) {
		io := newTestServer(nil, nil, nil)
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
		require.Equal(t, http.StatusBadRequest, rec.Code)

		e := new(ServerError)
		err = json.Unmarshal(rec.Body.Bytes(), e)
		if err != nil {
			t.Fatal(err)
		}
		require.Equal(t, serverErrors[ErrorBadHandshakeMethod].Code, e.Code)
		require.Equal(t, serverErrors[ErrorBadHandshakeMethod].Message, e.Message)
	})

	t.Run("authenticator should cause 403 to be returned as status code as expected", func(t *testing.T) {
		authenticator := func(w http.ResponseWriter, r *http.Request) (ok bool) {
			// Fail.
			return false
		}

		io := newTestServer(nil, &ServerConfig{
			Authenticator: authenticator,
		}, nil)
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
		require.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("should call `OnClose` with transport error when buffer size is exceeded (polling)", func(t *testing.T) {
		tw := NewTestWaiter(2) // Wait for the server and client.

		onSocket := func(socket ServerSocket) *Callbacks {
			return &Callbacks{
				OnClose: func(reason Reason, err error) {
					defer tw.Done()

					if reason != ReasonTransportError || err == nil {
						t.Error("exceeding the MaxBufferSize should've caused a transport error and the err should be non-nil")
					}
				},
			}
		}

		io := newTestServer(onSocket, &ServerConfig{
			MaxBufferSize: 5,
		}, nil)
		err := io.Run()
		if err != nil {
			t.Fatal(err)
		}
		ts := httptest.NewServer(io)

		callbacks := &Callbacks{
			OnClose: func(reason Reason, err error) {
				defer tw.Done()

				if reason != ReasonTransportError || err == nil {
					t.Error("exceeding the MaxBufferSize should've caused a transport error and the err should be non-nil")
				}
			},
		}

		socket := testDial(t, ts.URL, callbacks, &ClientConfig{
			Transports: []string{"polling"},
		}, nil)

		require.Equal(t, "polling", socket.TransportName())

		packet := mustCreatePacket(t, parser.PacketTypeMessage, false, []byte("123456"))
		socket.Send(packet)

		tw.WaitTimeout(t, DefaultTestWaitTimeout)
	})

	t.Run("`DisableMaxBufferSize` should cause `MaxBufferSize` to be ignored (polling)", func(t *testing.T) {
		tw := NewTestWaiter(1) // Wait for the server.
		testData := []byte("12345678")

		onSocket := func(socket ServerSocket) *Callbacks {
			return &Callbacks{
				OnPacket: func(packets ...*parser.Packet) {
					defer tw.Done()

					for _, packet := range packets {
						if packet.Type == parser.PacketTypeMessage {
							if !bytes.Equal(testData, packet.Data) {
								t.Error("data doesn't match")
							}
						}
					}
				},
			}
		}

		io := newTestServer(onSocket, &ServerConfig{
			MaxBufferSize:        5,
			DisableMaxBufferSize: true,
		}, nil)
		err := io.Run()
		if err != nil {
			t.Fatal(err)
		}

		ts := httptest.NewServer(io)
		socket := testDial(t, ts.URL, nil, &ClientConfig{
			Transports: []string{"polling"},
		}, nil)
		require.Equal(t, "polling", socket.TransportName())

		packet := mustCreatePacket(t, parser.PacketTypeMessage, false, testData)
		socket.Send(packet)

		tw.WaitTimeout(t, DefaultTestWaitTimeout)
	})

	t.Run("`DisableMaxBufferSize` should cause `MaxBufferSize` to be ignored (websocket)", func(t *testing.T) {
		tw := NewTestWaiter(1) // Wait for the server.
		testData := []byte("12345678")

		onSocket := func(socket ServerSocket) *Callbacks {
			return &Callbacks{
				OnPacket: func(packets ...*parser.Packet) {
					defer tw.Done()

					for _, packet := range packets {

						if packet.Type == parser.PacketTypeMessage {
							if !bytes.Equal(testData, packet.Data) {
								t.Error("data doesn't match")
							}
						}
					}
				},
			}
		}

		io := newTestServer(onSocket, &ServerConfig{
			MaxBufferSize:        5,
			DisableMaxBufferSize: true,
		}, nil)
		err := io.Run()
		if err != nil {
			t.Fatal(err)
		}
		ts := httptest.NewServer(io)

		socket := testDial(t, ts.URL, nil, &ClientConfig{
			Transports: []string{"websocket"},
		}, nil)
		require.Equal(t, "websocket", socket.TransportName())

		packet := mustCreatePacket(t, parser.PacketTypeMessage, false, testData)
		socket.Send(packet)

		tw.WaitTimeout(t, DefaultTestWaitTimeout)
	})

	t.Run("JSONP should work", func(t *testing.T) {
		tw := NewTestWaiter(2)

		const (
			pingInterval = 123456 * time.Second
			pingTimeout  = 654321 * time.Second
		)

		var (
			testPacket1 = mustCreatePacket(t, parser.PacketTypeMessage, false, []byte("Hello from server"))
			testPacket2 = mustCreatePacket(t, parser.PacketTypeMessage, true, []byte{0x1, 0x2, 0x3})
		)

		onSocket := func(socket ServerSocket) *Callbacks {
			socket.Send(testPacket1, testPacket2)

			return &Callbacks{
				OnPacket: func(packets ...*parser.Packet) {
					for _, packet := range packets {
						if packet.Type != parser.PacketTypeMessage {
							return
						}

						switch {
						case bytes.Equal(packet.Data, testPacket1.Data) && packet.IsBinary == false:
							tw.Done()
						case bytes.Equal(packet.Data, testPacket2.Data) && packet.IsBinary == true:
							tw.Done()
						default:
							t.Error("invalid message received")
						}
					}
				},
			}
		}

		io := newTestServer(onSocket, &ServerConfig{
			PingInterval: pingInterval,
			PingTimeout:  pingTimeout,
		}, nil)
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

		buf := bytes.NewBuffer([]byte(body))
		p, err := parser.Decode(buf, false)
		if err != nil {
			t.Fatal(err)
		}

		hr := new(parser.HandshakeResponse)
		err = json.Unmarshal(p.Data, hr)
		if err != nil {
			t.Fatal(err)
		}

		require.Equal(t, pingInterval, hr.GetPingInterval())
		require.Equal(t, pingTimeout, hr.GetPingTimeout())

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

		buf = bytes.NewBuffer([]byte(splitted[0]))

		p1, err := parser.Decode(buf, false)
		if err != nil {
			t.Fatal(err)
		}

		buf = bytes.NewBuffer([]byte(splitted[1]))

		p2, err := parser.Decode(buf, false)
		if err != nil {
			t.Fatal(err)
		}

		if !p2.IsBinary {
			t.Fatal("second packet should be a binary packet")
		}

		if !bytes.Equal(p1.Data, testPacket1.Data) {
			t.Fatal("data doesn't match")
		}

		if !bytes.Equal(p2.Data, testPacket2.Data) {
			t.Fatal("data doesn't match")
		}

		// Test sending packets to server

		buf = bytes.NewBuffer(nil)
		err = parser.EncodePayloads(buf, testPacket1, testPacket2)
		if err != nil {
			t.Fatal(err)
		}

		d := buf.String()
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

		tw.WaitTimeout(t, DefaultTestWaitTimeout)
	})

	t.Run("server `Close` method should close sockets", func(t *testing.T) {
		tw := NewTestWaiter(0)
		utw := NewTestWaiter(0) // For upgrades.

		onSocket := func(socket ServerSocket) *Callbacks {
			return &Callbacks{
				OnClose: func(reason Reason, err error) {
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

		io := newTestServer(onSocket, nil, nil)

		err := io.Run()
		if err != nil {
			t.Fatal(err)
		}

		ts := httptest.NewServer(io)

		transportsToTest := [][]string{
			{"polling"},
			{"websocket"},
			{"polling", "websocket"},
		}

		for _, transports := range transportsToTest {
			tw.Add(2) // For server and client.

			callbacks := &Callbacks{
				OnClose: func(reason Reason, err error) {
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

			testDial(t, ts.URL, callbacks, &ClientConfig{Transports: transports, UpgradeDone: upgradeDone}, nil)
		}

		// Wait for upgrades to finish.
		timedout := utw.WaitTimeout(t, 10*time.Second)
		if timedout {
			t.Fatal("upgrades couldn't finish")
		}

		err = io.Close()
		if err != nil {
			t.Fatal(err)
		}

		req, err := http.NewRequest("GET", ts.URL, nil)
		if err != nil {
			t.Fatal(err)
		}

		resp, err := ts.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		require.Equal(t, http.StatusServiceUnavailable, resp.StatusCode, "server should have been closed")

		tw.WaitTimeout(t, DefaultTestWaitTimeout)
	})
}

type testServerOptions struct {
	testWaitUpgrade bool
}

func newTestServer(onSocket NewSocketCallback, config *ServerConfig, options *testServerOptions) *Server {
	if config == nil {
		config = new(ServerConfig)
	}
	if options == nil {
		options = new(testServerOptions)
	}
	enablePrintDebugger := os.Getenv("EIO_DEBUGGER_PRINT") == "1"
	if enablePrintDebugger {
		config.Debugger = NewPrintDebugger()
	}
	return newServer(onSocket, config, options.testWaitUpgrade)
}
