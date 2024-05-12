package eio

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/quic-go/webtransport-go"
	"github.com/tomruk/socket.io-go/internal/sync"

	"github.com/tomruk/socket.io-go/engine.io/parser"
	"github.com/tomruk/socket.io-go/engine.io/transport"
	"github.com/tomruk/socket.io-go/engine.io/transport/polling"
	_websocket "github.com/tomruk/socket.io-go/engine.io/transport/websocket"
	_webtransport "github.com/tomruk/socket.io-go/engine.io/transport/webtransport"

	"nhooyr.io/websocket"
)

type (
	ServerAuthFunc func(w http.ResponseWriter, r *http.Request) (ok bool)

	ServerConfig struct {
		// This is a middleware function to authenticate clients before doing the handshake.
		// If this function returns false authentication will fail. Or else, the handshake will begin as usual.
		Authenticator ServerAuthFunc

		// When to send PING packets to clients.
		PingInterval time.Duration

		// After sending PING, client should send PONG before this timeout exceeds.
		PingTimeout time.Duration

		// Timeout to wait while a client transport is being upgraded.
		UpgradeTimeout time.Duration

		// MaxBufferSize is used for preventing DOS.
		// This is the equivalent of `maxHTTPBufferSize` in original Engine.IO.
		MaxBufferSize        int
		DisableMaxBufferSize bool

		// For accepting WebTransport connections
		WebTransportServer *webtransport.Server

		// Custom WebSocket options to use.
		WebSocketAcceptOptions *websocket.AcceptOptions

		// Callback function for Engine.IO server errors.
		// You may use this function to log server errors.
		OnError ErrorCallback

		// For debugging purposes. Leave it nil if it is of no use.
		Debugger Debugger
	}

	Server struct {
		authenticator ServerAuthFunc

		pingInterval   time.Duration
		pingTimeout    time.Duration
		upgradeTimeout time.Duration

		maxBufferSize        int
		disableMaxBufferSize bool

		webTransportServer *webtransport.Server

		wsAcceptOptions *websocket.AcceptOptions

		onSocket NewSocketCallback
		onError  ErrorCallback
		store    *socketStore

		closed          chan struct{}
		closeOnce       sync.Once
		debug           Debugger
		testWaitUpgrade bool
	}
)

func NewServer(onSocket NewSocketCallback, config *ServerConfig) *Server {
	return newServer(onSocket, config, false)
}

func newServer(onSocket NewSocketCallback, config *ServerConfig, testWaitUpgrade bool) *Server {
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

		webTransportServer: config.WebTransportServer,

		wsAcceptOptions: config.WebSocketAcceptOptions,

		onSocket: onSocket,
		onError:  config.OnError,

		store: newSocketStore(),

		closed:          make(chan struct{}),
		testWaitUpgrade: testWaitUpgrade,
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

	if config.Debugger != nil {
		s.debug = config.Debugger
	} else {
		s.debug = NewNoopDebugger()
	}
	s.debug = s.debug.WithContext("[eio/server] Server")

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
		s.debug.Log("Connection received after server was closed")
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	q := r.URL.Query()

	// Skip protocol version check for WebTransport
	if r.ProtoMajor != 3 {
		version, err := strconv.Atoi(q.Get("EIO"))
		if err != nil {
			writeServerError(w, ErrorUnsupportedProtocolVersion)
			return
		}
		if version != ProtocolVersion {
			writeServerError(w, ErrorUnsupportedProtocolVersion)
			return
		}
	}

	sid := q.Get("sid")
	if sid == "" {
		s.handleHandshake(w, r)
	} else {
		socket, ok := s.store.get(sid)
		if !ok {
			writeServerError(w, ErrorUnknownSID)
			return
		}

		t := socket.Transport()
		n := r.URL.Query().Get("transport")

		if t.Name() != n {
			s.maybeUpgrade(w, r, socket, n, nil, nil)
			return
		}

		t.ServeHTTP(w, r)
	}
}

func (s *Server) handleHandshake(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	n := q.Get("transport")
	supportsBinary := q.Get("b64") == ""

	if r.Method != "GET" && r.ProtoMajor != 3 {
		writeServerError(w, ErrorBadHandshakeMethod)
		return
	} else if r.Method == "CONNECT" && r.ProtoMajor == 3 && n == "" {
		s.onWebTransport(w, r)
		return
	}

	ok := s.authenticator(w, r)
	if !ok {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	sid, err := s.generateSID()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		s.onError(err)
		return
	}

	var (
		t        ServerTransport
		upgrades []string
		c        = transport.NewCallbacks()
	)

	switch n {
	case "polling":
		t = polling.NewServerTransport(c, s.maxBufferSize, s.PollTimeout())
		upgrades = []string{"websocket"}
		if s.webTransportServer != nil {
			upgrades = append(upgrades, "webtransport")
		}
	case "websocket":
		t = _websocket.NewServerTransport(c, s.maxBufferSize, supportsBinary, s.wsAcceptOptions)
		if s.webTransportServer != nil {
			upgrades = []string{"webtransport"}
		}
	default:
		writeServerError(w, ErrorUnknownTransport)
		return
	}

	s.debug.Log("Transport is set to", n)

	handshakePacket, err := s.newHandshakePacket(sid, upgrades)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		s.onError(wrapInternalError(fmt.Errorf("newHandshakePacket failed: %w", err)))
		return
	}
	_, err = t.Handshake(handshakePacket, w, r)
	if err != nil {
		s.debug.Log("Handshake error", err)
		return
	}

	socket := s.newSocket(w, sid, upgrades, c, t)
	if socket == nil {
		return
	}

	t.PostHandshake(nil)
}

func (s *Server) onWebTransport(w http.ResponseWriter, r *http.Request) {
	if s.webTransportServer == nil {
		writeServerError(w, ErrorUnknownTransport)
		return
	}
	c := transport.NewCallbacks()
	t := _webtransport.NewServerTransport(c, s.maxBufferSize, s.webTransportServer)

	sid, err := t.Handshake(nil, w, r)
	if err != nil {
		s.debug.Log("Handshake error", err)
		t.Close()
		return
	}

	if sid == "" {
		sid, err = s.generateSID()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			s.onError(err)
			t.Close()
			return
		}

		socket := s.newSocket(w, sid, nil, c, t)
		if socket == nil {
			t.Close()
			return
		}

		handshakePacket, err := s.newHandshakePacket(sid, nil)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			s.onError(wrapInternalError(fmt.Errorf("newHandshakePacket failed: %w", err)))
			t.Close()
			return
		}
		t.PostHandshake(handshakePacket)
	} else {
		socket, ok := s.store.get(sid)
		if !ok {
			writeServerError(w, ErrorUnknownSID)
			t.Close()
			return
		}

		if socket.Transport().Name() == "webtransport" {
			s.debug.Log("already upgraded to webtransport")
			t.Close()
		} else {
			s.maybeUpgrade(w, r, socket, "webtransport", t, c)
		}
	}
}

func (s *Server) newSocket(w http.ResponseWriter, sid string, upgrades []string, c *transport.Callbacks, t ServerTransport) *serverSocket {
	socket := newServerSocket(sid, upgrades, t, c, s.pingInterval, s.pingTimeout, s.debug, s.store.delete)

	callbacks := s.onSocket(socket)
	socket.setCallbacks(callbacks)

	ok := s.store.set(sid, socket)
	if !ok {
		w.WriteHeader(http.StatusInternalServerError)
		err := fmt.Errorf("sid's overlap")
		s.onError(wrapInternalError(err))
		socket.close(ReasonTransportError, err)
		return nil
	}
	return socket
}

func (s *Server) newHandshakePacket(sid string, upgrades []string) (*parser.Packet, error) {
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

func (s *Server) maybeUpgrade(
	w http.ResponseWriter, r *http.Request,
	socket *serverSocket,
	upgradeTo string,
	t ServerTransport,
	c *transport.Callbacks,
) {
	var (
		q              = r.URL.Query()
		supportsBinary = q.Get("b64") == ""
	)

	if c == nil {
		c = transport.NewCallbacks()
	}

	done := make(chan struct{})
	once := new(sync.Once)

	onPacket := func(packet *parser.Packet) {
		s.debug.Log("Packet received", packet)

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
			socket.upgradeTo(t, c)
		default:
			t.Close()
			socket.onError(wrapInternalError(fmt.Errorf("upgrade failed: invalid packet received: packet type: %d", packet.Type)))
			return
		}
	}

	if !s.testWaitUpgrade {
		c.Set(func(packets ...*parser.Packet) {
			for _, packet := range packets {
				onPacket(packet)
			}
		}, nil)
	}

	switch upgradeTo {
	case "websocket":
		t = _websocket.NewServerTransport(c, s.maxBufferSize, supportsBinary, s.wsAcceptOptions)
		_, err := t.Handshake(nil, w, r)
		if err != nil {
			s.debug.Log("Handshake error", err)
			t.Close()
			return
		}
		if s.testWaitUpgrade {
			time.Sleep(1001 * time.Millisecond)
		}
	case "webtransport":
		if t == nil {
			s.debug.Log("t == nil. This shouldn't have happened")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	default:
		s.debug.Log("Invalid upgradeTo", upgradeTo)
		writeServerError(w, ErrorBadRequest)
		return
	}

	if s.testWaitUpgrade {
		c.Set(func(packets ...*parser.Packet) {
			for _, packet := range packets {
				onPacket(packet)
			}
		}, nil)
	}

	go func() {
		select {
		case <-done:
			s.debug.Log("`done` triggered")
		case <-time.After(s.upgradeTimeout):
			t.Close()
			socket.onError(fmt.Errorf("eio: upgrade failed: %w", errUpgradeTimeoutExceeded))
		}
	}()

	t.PostHandshake(nil)
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
	s.debug.Log("Closing")

	// Prevent new clients from connecting.
	s.closeOnce.Do(func() {
		close(s.closed)
	})

	// Close all sockets that are currently connected.
	s.store.closeAll()
	return nil
}
