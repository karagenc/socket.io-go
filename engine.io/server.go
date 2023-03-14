package eio

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/tomruk/socket.io-go/engine.io/parser"
	"github.com/tomruk/socket.io-go/engine.io/transport"
	"github.com/tomruk/socket.io-go/engine.io/transport/polling"
	_websocket "github.com/tomruk/socket.io-go/engine.io/transport/websocket"

	"nhooyr.io/websocket"
)

type AuthFunc func(w http.ResponseWriter, r *http.Request) (ok bool)

type ServerError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func GetServerError(code int) (se ServerError, ok bool) {
	se, ok = serverErrors[code]
	return
}

const (
	ErrorUnknownTransport = iota
	ErrorUnknownSID
	ErrorBadHandshakeMethod
	ErrorBadRequest
	ErrorForbidden
	ErrorUnsupportedProtocolVersion
)

var serverErrors = map[int]ServerError{
	ErrorUnknownTransport: {
		Code:    0,
		Message: "Transport unknown",
	},
	ErrorUnknownSID: {
		Code:    1,
		Message: "Session ID unknown",
	},
	ErrorBadHandshakeMethod: {
		Code:    2,
		Message: "Bad handshake method",
	},
	ErrorBadRequest: {
		Code:    3,
		Message: "Bad request",
	},
	ErrorForbidden: {
		Code:    4,
		Message: "Forbidden",
	},
	ErrorUnsupportedProtocolVersion: {
		Code:    5,
		Message: "Unsupported protocol version",
	},
}

func writeError(w http.ResponseWriter, code int) {
	w.WriteHeader(http.StatusBadRequest)

	em, ok := serverErrors[code]
	if ok {
		data, _ := json.Marshal(&em)
		w.Write(data)
	}
}

type ServerConfig struct {
	// This is a middleware function to authenticate clients before doing the handshake.
	// If this function returns false authentication will fail. Or else, the handshake will begin as usual.
	Authenticator AuthFunc

	// When to send PING packets to clients.
	PingInterval time.Duration

	// After sending PING, client should send PONG before this timeout exceeds.
	PingTimeout time.Duration

	// Timeout to wait before upgrading a client transport.
	UpgradeTimeout time.Duration

	// MaxBufferSize is used for preventing DOS.
	// This is the equivalent of maxHTTPBufferSize.
	MaxBufferSize        int
	DisableMaxBufferSize bool

	// Custom WebSocket options to use.
	WebSocketAcceptOptions *websocket.AcceptOptions

	// Callback function for Engine.IO server errors.
	// You may use this function to log server errors.
	OnError ErrorCallback

	// For debugging purposes. Leave it nil if it is of no use.
	Debugger Debugger
}

type Server struct {
	authenticator AuthFunc

	pingInterval   time.Duration
	pingTimeout    time.Duration
	upgradeTimeout time.Duration

	maxBufferSize        int
	disableMaxBufferSize bool

	wsAcceptOptions *websocket.AcceptOptions

	onSocket NewSocketCallback
	onError  ErrorCallback

	store *socketStore

	closed    chan struct{}
	closeOnce sync.Once
}

func NewServer(onSocket NewSocketCallback, config *ServerConfig) *Server {
	if onSocket == nil {
		onSocket = func(socket ServerSocket) *Callbacks { return nil }
	}

	if config == nil {
		config = new(ServerConfig)
	}

	s := &Server{
		authenticator: config.Authenticator,

		pingInterval:   config.PingInterval,
		pingTimeout:    config.PingTimeout,
		upgradeTimeout: config.UpgradeTimeout,

		maxBufferSize:        config.MaxBufferSize,
		disableMaxBufferSize: config.DisableMaxBufferSize,

		wsAcceptOptions: config.WebSocketAcceptOptions,

		onSocket: onSocket,
		onError:  config.OnError,

		store: newSocketStore(),

		closed: make(chan struct{}),
	}

	if s.authenticator == nil {
		s.authenticator = func(w http.ResponseWriter, r *http.Request) (ok bool) { return true }
	}

	if s.pingInterval == 0 {
		s.pingInterval = defaultPingInterval
	}

	if s.pingTimeout == 0 {
		s.pingTimeout = defaultPingTimeout
	}

	if s.upgradeTimeout == 0 {
		s.upgradeTimeout = defaultUpgradeTimeout
	}

	if s.disableMaxBufferSize {
		s.maxBufferSize = 0
	} else {
		if s.maxBufferSize == 0 {
			s.maxBufferSize = defaultMaxBufferSize
		}
	}

	if s.onError == nil {
		s.onError = func(err error) {}
	}
	return s
}

func (s *Server) Run() error {
	if s.IsClosed() {
		return fmt.Errorf("eio: server is closed. a socket.io server cannot be restarted")
	}
	if s.pingInterval < 1*time.Second {
		return fmt.Errorf("eio: pingInterval must be equal or greater than 1 second")
	}
	if s.pingTimeout < 1*time.Second {
		return fmt.Errorf("eio: pingTimeout must be equal or greater than 1 second")
	}
	if s.upgradeTimeout < 1*time.Second {
		return fmt.Errorf("eio: upgradeTimeout must be equal or greater than 1 second")
	}
	return nil
}

func (s *Server) PollTimeout() time.Duration {
	return s.pingInterval + s.pingTimeout
}

func (s *Server) HTTPWriteTimeout() time.Duration {
	// Add a reasonable time (10 seconds) so that if PollTimeout is reached, we can still write the HTTP response.
	return s.PollTimeout() + 10*time.Second
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if s.IsClosed() {
		w.WriteHeader(http.StatusTeapot)
		return
	}

	q := r.URL.Query()

	version, err := strconv.Atoi(q.Get("EIO"))
	if err != nil {
		writeError(w, ErrorUnsupportedProtocolVersion)
		return
	}

	if version != ProtocolVersion {
		writeError(w, ErrorUnsupportedProtocolVersion)
		return
	}

	sid := q.Get("sid")
	if sid == "" {
		s.handleHandshake(w, r)
	} else {
		socket, ok := s.store.Get(sid)
		if !ok {
			writeError(w, ErrorUnknownSID)
			return
		}

		t := socket.Transport()
		n := r.URL.Query().Get("transport")

		if t.Name() != n {
			s.maybeUpgrade(w, r, socket, n)
			return
		}

		t.ServeHTTP(w, r)
	}
}

func (s *Server) handleHandshake(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		writeError(w, ErrorBadHandshakeMethod)
		return
	}

	ok := s.authenticator(w, r)
	if !ok {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	q := r.URL.Query()
	n := q.Get("transport")
	supportsBinary := q.Get("b64") == ""

	sid, err := s.generateSID()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		s.onError(err)
		return
	}

	newHandshakePacket := func(upgrades []string) (*parser.Packet, error) {
		data, err := json.Marshal(&parser.HandshakeResponse{
			SID:          sid,
			Upgrades:     upgrades,
			PingInterval: int64(s.pingInterval / time.Millisecond),
			PingTimeout:  int64(s.pingTimeout / time.Millisecond),
		})
		if err != nil {
			return nil, err
		}

		return parser.NewPacket(parser.PacketTypeOpen, false, data)
	}

	var (
		t        ServerTransport
		upgrades []string
	)

	c := transport.NewCallbacks()

	switch n {
	case "polling":
		t = polling.NewServerTransport(c, s.maxBufferSize, s.PollTimeout())
		upgrades = []string{"websocket"}
	case "websocket":
		t = _websocket.NewServerTransport(c, s.maxBufferSize, supportsBinary, s.wsAcceptOptions)
	default:
		writeError(w, ErrorUnknownTransport)
		return
	}

	handshakePacket, err := newHandshakePacket(upgrades)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		s.onError(wrapInternalError(fmt.Errorf("newHandshakePacket failed: %w", err)))
		return
	}

	err = t.Handshake(handshakePacket, w, r)
	if err != nil {
		return
	}

	socket := newServerSocket(sid, upgrades, t, s.pingInterval, s.pingTimeout, s.store.Delete)

	callbacks := s.onSocket(socket)
	socket.setCallbacks(callbacks)

	ok = s.store.Set(sid, socket)
	if !ok {
		w.WriteHeader(http.StatusInternalServerError)
		s.onError(wrapInternalError(fmt.Errorf("sid's overlap")))
		socket.close(ReasonTransportError, err)
		return
	}

	t.PostHandshake()
}

func (s *Server) maybeUpgrade(w http.ResponseWriter, r *http.Request, socket *serverSocket, upgradeTo string) {
	if upgradeTo != "websocket" {
		writeError(w, ErrorBadRequest)
		return
	}

	c := transport.NewCallbacks()

	t := _websocket.NewServerTransport(c, s.maxBufferSize, true, s.wsAcceptOptions)
	done := make(chan struct{})
	once := new(sync.Once)

	err := t.Handshake(nil, w, r)
	if err != nil {
		return
	}

	go func() {
		select {
		case <-done:
			return
		case <-time.After(s.upgradeTimeout):
			t.Close()
			s.onError(fmt.Errorf("eio: upgrade failed: upgradeTimeout exceeded"))
		}
	}()

	onPacket := func(packet *parser.Packet) {
		switch packet.Type {
		case parser.PacketTypePing:
			pong, err := parser.NewPacket(parser.PacketTypePong, false, []byte("probe"))
			if err != nil {
				return
			}
			t.Send(pong)

			// Force a polling cycle to ensure a fast upgrade.
			noop, err := parser.NewPacket(parser.PacketTypeNoop, false, nil)
			if err != nil {
				return
			}
			go socket.Send(noop)
		case parser.PacketTypeUpgrade:
			once.Do(func() { close(done) })
			socket.UpgradeTo(t)
		default:
			t.Close()
			socket.onError(wrapInternalError(fmt.Errorf("upgrade failed: invalid packet received")))
			return
		}
	}

	c.Set(func(packets ...*parser.Packet) {
		for _, p := range packets {
			onPacket(p)
		}
	}, nil)

	t.PostHandshake()
}

func (s *Server) IsClosed() bool {
	select {
	case <-s.closed:
		return true
	default:
		return false
	}
}

func (s *Server) Close() error {
	// Prevent new clients from connecting.
	s.closeOnce.Do(func() {
		close(s.closed)
	})

	// Close all sockets that are currently connected.
	s.store.CloseAll()
	return nil
}
