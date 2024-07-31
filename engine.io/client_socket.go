package eio

import (
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/karagenc/socket.io-go/internal/sync"

	_webtransport "github.com/quic-go/webtransport-go"
	_websocket "nhooyr.io/websocket"

	"github.com/karagenc/socket.io-go/engine.io/parser"
	"github.com/karagenc/socket.io-go/engine.io/transport"
	"github.com/karagenc/socket.io-go/engine.io/transport/polling"
	"github.com/karagenc/socket.io-go/engine.io/transport/websocket"
	"github.com/karagenc/socket.io-go/engine.io/transport/webtransport"
)

var errUpgradeTimeoutExceeded = fmt.Errorf("upgradeTimeout exceeded")

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

	// WebTransport dialer to use on transports
	webTransportDialer *_webtransport.Dialer

	// WebSocket dialer to use on transports
	wsDialOptions *_websocket.DialOptions

	// These are set after the handshake.
	sid          string
	upgrades     []string
	pingInterval time.Duration
	pingTimeout  time.Duration
	maxPayload   int64

	callbacks Callbacks

	pingChan chan struct{}

	closeChan chan struct{}
	closeOnce sync.Once

	testWaitUpgrade bool
	debug           Debugger
}

func (s *clientSocket) connect(transports []string) (err error) {
	s.transportMu.Lock()
	defer s.transportMu.Unlock()

	for _, name := range transports {
		transports = transports[1:]
		c := transport.NewCallbacks()

		switch name {
		case "polling":
			s.transport = polling.NewClientTransport(
				c,
				ProtocolVersion,
				*s.url,
				s.requestHeader,
				s.httpClient,
			)
		case "websocket":
			s.transport = websocket.NewClientTransport(
				c,
				"",
				ProtocolVersion,
				*s.url,
				s.requestHeader,
				s.wsDialOptions,
			)
		case "webtransport":
			s.transport = webtransport.NewClientTransport(
				c,
				"",
				ProtocolVersion,
				*s.url,
				s.requestHeader,
				s.webTransportDialer,
			)
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
		s.maxPayload = hr.MaxPayload

		go s.transport.Run()
		break
	}

	if err == nil {
		go s.maybeUpgrade(transports, s.upgrades)
		go s.handleTimeout()
	}
	return
}

func (s *clientSocket) ID() string { return s.sid }

func (s *clientSocket) Upgrades() []string { return s.upgrades }

func (s *clientSocket) PingInterval() time.Duration { return s.pingInterval }

func (s *clientSocket) PingTimeout() time.Duration { return s.pingTimeout }

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
	if s.TransportName() == "webtransport" {
		s.debug.Log("maybeUpgrade", "current transport is webtransport. already upgraded")
		return
	} else if s.TransportName() == "websocket" && !findTransport(upgrades, "webtransport") {
		s.debug.Log("maybeUpgrade", "current transport is websocket, and there are no further upgrades. already upgraded")
		return
	} else if !findTransport(upgrades, "websocket") && !findTransport(upgrades, "webtransport") {
		s.debug.Log("maybeUpgrade", "couldn't find 'websocket' and 'webtransport' in handshake received from server")
		return
	} else if !findTransport(transports, "websocket") && !findTransport(transports, "webtransport") {
		s.debug.Log("maybeUpgrade", "couldn't find 'websocket' and 'webtransport' in `Transports` configuration option")
		return
	}

	// Prioritize webtransport
	if findTransport(transports, "webtransport") && findTransport(transports, "websocket") {
		for i, transport := range transports {
			if transport == "webtransport" {
				transports = append(transports[:i], transports[i+1:]...)
			}
		}
		transports = append([]string{"webtransport"}, transports...)
	}

	for _, upgradeTo := range transports {
		if !findTransport(upgrades, upgradeTo) {
			s.debug.Log("skip", upgradeTo)
			continue
		}
		var (
			t ClientTransport
			c = transport.NewCallbacks()
		)
		switch upgradeTo {
		case "websocket":
			s.debug.Log("maybeUpgrade", "upgrading from", s.TransportName(), "to websocket")
			t = websocket.NewClientTransport(c, s.sid, ProtocolVersion, *s.url, s.requestHeader, s.wsDialOptions)
		case "webtransport":
			s.debug.Log("maybeUpgrade", "upgrading from", s.TransportName(), "to webtransport")
			t = webtransport.NewClientTransport(c, s.sid, ProtocolVersion, *s.url, s.requestHeader, s.webTransportDialer)
		default:
			s.debug.Log("skip", upgradeTo)
		}
		if s.tryUpgradeTo(t, c) {
			return
		}
	}
}

func (s *clientSocket) tryUpgradeTo(t ClientTransport, c *transport.Callbacks) (ok bool) {
	done := make(chan struct{})
	once := new(sync.Once)

	onPacket := func(packet *parser.Packet) {
		s.debug.Log("maybeUpgrade", "packet received", packet)

		switch packet.Type {
		case parser.PacketTypePong:
			pong := string(packet.Data)
			if pong != "probe" {
				s.onError(wrapInternalError(fmt.Errorf("upgrade failed: invalid packet received: pong with invalid data: '%s'", pong)))
				t.Close()
				return
			}

			once.Do(func() { close(done) })
			s.finishUpgradeTo(t, c)
		default:
			t.Close()
			s.onError(wrapInternalError(fmt.Errorf("upgrade failed: invalid packet received: packet type: %d", packet.Type)))
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
	if s.testWaitUpgrade {
		time.Sleep(1001 * time.Millisecond)
	}
	go t.Run()

	ping, err := parser.NewPacket(parser.PacketTypePing, false, []byte("probe"))
	if err != nil {
		t.Close()
		s.onError(wrapInternalError(fmt.Errorf("upgrade failed: %w", err)))
		return
	}
	go t.Send(ping)

	select {
	case <-done:
		s.debug.Log("maybeUpgrade", "channel `done` is triggered")
		return true
	case <-time.After(s.upgradeTimeout):
		t.Close()
		s.onError(fmt.Errorf("eio: upgrade failed: %w", errUpgradeTimeoutExceeded))
		return false
	}
}

func (s *clientSocket) finishUpgradeTo(t ClientTransport, c *transport.Callbacks) {
	p, err := parser.NewPacket(parser.PacketTypeUpgrade, false, nil)
	if err != nil {
		s.onError(fmt.Errorf("upgrade failed: %w", err))
		return
	}

	c.Set(s.onPacket, s.onTransportClose)

	s.transportMu.Lock()
	defer s.transportMu.Unlock()

	old := s.transport
	s.transport = t

	old.Discard()

	t.Send(p)
	s.debug.Log("upgradeTo", "upgraded to", t.Name())
	// Don't block
	go s.upgradeDone(t.Name())
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
	go func() { // <- To prevent s.TransportName() from blocking (locks transportMu).
		if err == nil {
			s.debug.Log("Transport", name, "closed")
		} else {
			s.debug.Log("Transport", name, "closed. Error", err)
		}

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
	}()
}

func (s *clientSocket) TransportName() string {
	s.transportMu.RLock()
	defer s.transportMu.RUnlock()
	return s.transport.Name()
}

func (s *clientSocket) Send(packets ...*parser.Packet) {
	s.transportMu.RLock()
	defer s.transportMu.RUnlock()
	s.writeWritablePackets(packets...)
}

// Equivalent of `getWritablePackets` in original Socket.IO.
func (s *clientSocket) writeWritablePackets(packets ...*parser.Packet) {
	shouldCheckPayloadSize := s.maxPayload > 0 && s.transport.Name() == "polling" && len(packets) > 1
	if shouldCheckPayloadSize {
		// In original engine.io client, this variable is set to 1
		// to have first packet type.
		// But we omit it, since packet.EncodedLen(false) includes packet type.
		payloadSize := 0
		total := len(packets)
		// Range based for loop overflows the array.
		// The check, `i < len(packets)`, needs to be made every time.
		for i, count := 0, 0; i < len(packets); i, count = i+1, count+1 {
			packet := packets[i]
			if len(packet.Data) > 0 {
				// Since we're dealing with the polling transport, supportsBinary argument is false.
				payloadSize += packet.EncodedLen(false)
			}
			if i > 0 && int64(payloadSize) > s.maxPayload {
				s.debug.Log("send", count, "out of", total)
				if len(packets) > 0 {
					s.transport.Send(packets[:i]...)
				}
				packets = packets[i:]
				i = 0
				payloadSize = 0
				continue
			}
			payloadSize += 1 // Separator
		}
		s.debug.Log("payload size is", payloadSize, "maxPayload", s.maxPayload)
	}
	if len(packets) > 0 {
		s.transport.Send(packets...)
	}
}

func (s *clientSocket) close(reason Reason, err error) {
	s.debug.Log("Going to close the socket if it is not already closed. Reason", reason)

	s.closeOnce.Do(func() {
		s.debug.Log("Going to close the socket. It is not already closed. Reason", reason)
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

func (s *clientSocket) Close() { s.close(ReasonForcedClose, nil) }
