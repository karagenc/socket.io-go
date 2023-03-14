package eio

import (
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"nhooyr.io/websocket"

	"github.com/tomruk/socket.io-go/engine.io/parser"
	"github.com/tomruk/socket.io-go/engine.io/transport"
	"github.com/tomruk/socket.io-go/engine.io/transport/polling"
	_websocket "github.com/tomruk/socket.io-go/engine.io/transport/websocket"
)

type clientSocket struct {
	transport   ClientTransport
	transportMu sync.RWMutex

	url *url.URL

	upgradeTimeout time.Duration
	upgradeDone    func(transportName string)

	// HTTP client to use on transports.
	httpClient *http.Client

	// HTTP headers to use on transports.
	requestHeader *transport.RequestHeader

	// WebSocket dialer to use on transports.
	wsDialOptions *websocket.DialOptions

	// These are set after the handshake.
	sid          string
	upgrades     []string
	pingInterval time.Duration
	pingTimeout  time.Duration

	callbacks Callbacks

	pingChan chan struct{}

	closeChan chan struct{}
	closeOnce sync.Once

	debug Debugger
}

func (s *clientSocket) Connect(transports []string) (err error) {
	s.transportMu.Lock()
	defer s.transportMu.Unlock()

	for _, name := range transports {
		transports = transports[1:]

		c := transport.NewCallbacks()

		switch name {
		case "websocket":
			s.transport = _websocket.NewClientTransport(c, "", ProtocolVersion, *s.url, s.requestHeader, s.wsDialOptions)
		case "polling":
			s.transport = polling.NewClientTransport(c, ProtocolVersion, *s.url, s.requestHeader, s.httpClient)
		default:
			err = fmt.Errorf("eio: invalid transport name: %s", name)
			return
		}
		s.debug.Log("Transport is set to", name)

		c.Set(s.onPacket, s.onTransportClose)

		var hr *parser.HandshakeResponse
		hr, err = s.transport.Handshake()
		if err != nil {
			s.debug.Log("Handshake failed", err)
			continue
		}

		s.sid = hr.SID
		s.upgrades = hr.Upgrades
		s.pingInterval = hr.GetPingInterval()
		s.pingTimeout = hr.GetPingTimeout()
		s.debug.Log("pingInterval", s.pingInterval)
		s.debug.Log("pingTimeout", s.pingTimeout)
		go s.transport.Run()
		break
	}

	if err == nil {
		go s.maybeUpgrade(transports, s.upgrades)
		go s.handleTimeout()
	}

	return
}

func (s *clientSocket) ID() string {
	return s.sid
}

func (s *clientSocket) Upgrades() []string {
	return s.upgrades
}

func (s *clientSocket) PingInterval() time.Duration {
	return s.pingInterval
}

func (s *clientSocket) PingTimeout() time.Duration {
	return s.pingTimeout
}

func (s *clientSocket) handleTimeout() {
	for {
		timeout := s.pingInterval + s.pingTimeout

		select {
		case <-s.pingChan:
			s.debug.Log("handleTimeout", "ping received")
		case <-time.After(timeout):
			s.debug.Log("handleTimeout", "timed out")
			s.close(ReasonPingTimeout, nil)
			return
		case <-s.closeChan:
			s.debug.Log("handleTimeout", "socket was closed")
			return
		}
	}
}

func (s *clientSocket) maybeUpgrade(transports []string, upgrades []string) {
	if s.TransportName() == "websocket" {
		s.debug.Log("maybeUpgrade", "current transport is websocket. already upgraded")
		return
	} else if !findTransport(upgrades, "websocket") {
		s.debug.Log("maybeUpgrade", "couldn't find 'websocket' in handshake received from server")
		return
	} else if !findTransport(transports, "websocket") {
		s.debug.Log("maybeUpgrade", "couldn't find 'websocket' in `Transports` configuration option")
		return
	}

	s.debug.Log("maybeUpgrade", "upgrading")

	c := transport.NewCallbacks()
	t := _websocket.NewClientTransport(c, s.sid, ProtocolVersion, *s.url, s.requestHeader, s.wsDialOptions)
	done := make(chan struct{})
	once := new(sync.Once)

	onPacket := func(packet *parser.Packet) {
		s.debug.Log("maybeUpgrade", "packet received", packet)

		switch packet.Type {
		case parser.PacketTypePong:
			if string(packet.Data) != "probe" {
				s.onError(wrapInternalError(fmt.Errorf("upgrade failed: invalid packet received")))
				t.Close()
				return
			}

			once.Do(func() { close(done) })
			s.upgradeTo(t)
		default:
			t.Close()
			s.onError(wrapInternalError(fmt.Errorf("upgrade failed: invalid packet received")))
			return
		}
	}

	c.Set(func(packets ...*parser.Packet) {
		for _, packet := range packets {
			onPacket(packet)
		}
	}, nil)

	_, err := t.Handshake()
	if err != nil {
		t.Close()
		s.onError(fmt.Errorf("eio: upgrade failed: %w", err))
		return
	}

	go func() {
		select {
		case <-done:
			s.debug.Log("maybeUpgrade", "channel `done` is triggered")
			return
		case <-time.After(s.upgradeTimeout):
			t.Close()
			s.onError(fmt.Errorf("eio: upgrade failed: upgradeTimeout exceeded"))
		}
	}()

	go t.Run()

	ping, err := parser.NewPacket(parser.PacketTypePing, false, []byte("probe"))
	if err != nil {
		t.Close()
		s.onError(wrapInternalError(fmt.Errorf("upgrade failed: %w", err)))
		return
	}
	t.Send(ping)
}

func (s *clientSocket) upgradeTo(t ClientTransport) {
	p, err := parser.NewPacket(parser.PacketTypeUpgrade, false, nil)
	if err != nil {
		s.onError(fmt.Errorf("upgrade failed: %w", err))
		return
	}

	t.Callbacks().Set(s.onPacket, s.onTransportClose)

	s.transportMu.Lock()
	defer s.transportMu.Unlock()

	old := s.transport
	s.transport = t
	old.Discard()

	t.Send(p)
	s.debug.Log("upgradeTo", "upgraded to", t.Name())
	go s.upgradeDone(t.Name()) // Don't block
}

func findTransport(transports []string, name string) bool {
	for _, n := range transports {
		if name == n {
			return true
		}
	}
	return false
}

func (s *clientSocket) onPacket(packets ...*parser.Packet) {
	s.callbacks.OnPacket(packets...)

	for _, packet := range packets {
		s.handlePacket(packet)
	}
}

func (s *clientSocket) handlePacket(packet *parser.Packet) {
	switch packet.Type {
	case parser.PacketTypePing:
		select {
		case s.pingChan <- struct{}{}:
		default:
		}

		pong, err := parser.NewPacket(parser.PacketTypePong, false, packet.Data)
		if err != nil {
			s.onError(err)
			return
		}
		s.Send(pong)
	case parser.PacketTypeClose:
		s.transportMu.RLock()
		defer s.transportMu.RUnlock()
		s.transport.Close()
	}
}

func (s *clientSocket) onError(err error) {
	if err != nil {
		s.callbacks.OnError(err)
	}
}

func (s *clientSocket) onTransportClose(name string, err error) {
	s.debug.Log("Transport", name, "closed")

	select {
	case <-s.closeChan:
		return
	default:
	}

	if s.TransportName() != name {
		return
	}

	if err == nil {
		s.close(ReasonTransportClose, nil)
	} else {
		s.close(ReasonTransportError, err)
	}
}

func (s *clientSocket) TransportName() string {
	s.transportMu.RLock()
	defer s.transportMu.RUnlock()
	return s.transport.Name()
}

func (s *clientSocket) Send(packets ...*parser.Packet) {
	s.transportMu.RLock()
	defer s.transportMu.RUnlock()
	s.transport.Send(packets...)
}

func (s *clientSocket) close(reason Reason, err error) {
	s.closeOnce.Do(func() {
		s.debug.Log("Closing. Reason", reason)
		close(s.closeChan)
		defer s.callbacks.OnClose(reason, err)

		if reason != ReasonTransportClose && reason != ReasonTransportError {
			s.transportMu.RLock()
			defer s.transportMu.RUnlock()
			if s.transport != nil {
				s.transport.Close()
			}
		}
	})
}

func (s *clientSocket) Close() {
	s.close(ReasonForcedClose, nil)
}
