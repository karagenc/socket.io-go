package eio

import (
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/tomruk/socket.io-go/engine.io/parser"
	"github.com/tomruk/socket.io-go/engine.io/transport/polling"
	_websocket "github.com/tomruk/socket.io-go/engine.io/transport/websocket"
)

type clientSocket struct {
	t   ClientTransport
	tMu sync.RWMutex

	url *url.URL

	upgradeTimeout time.Duration
	upgradeDone    func(transportName string)

	// HTTP client to use on transports.
	httpClient *http.Client

	// HTTP headers to use on transports.
	requestHeader http.Header

	// WebSocket dialer to use on transports.
	wsDialer *websocket.Dialer

	// These are set after the handshake.
	sid          string
	upgrades     []string
	pingInterval time.Duration
	pingTimeout  time.Duration

	callbacks Callbacks

	pingChan chan struct{}

	closeChan chan struct{}
	once      sync.Once
}

func (s *clientSocket) Connect(transports []string) (err error) {
	s.tMu.Lock()
	defer s.tMu.Unlock()

	for _, name := range transports {
		transports = transports[1:]

		switch name {
		case "websocket":
			s.t = _websocket.NewClientTransport("", ProtocolVersion, *s.url, s.requestHeader, s.wsDialer)
		case "polling":
			s.t = polling.NewClientTransport(ProtocolVersion, *s.url, s.requestHeader, s.httpClient)
		default:
			err = fmt.Errorf("invalid transport name: %s", name)
			return
		}

		s.t.SetCallbacks(s.onPacket, s.onTransportClose)

		var hr *parser.HandshakeResponse
		hr, err = s.t.Handshake()
		if err != nil {
			continue
		}

		s.sid = hr.SID
		s.upgrades = hr.Upgrades
		s.pingInterval = hr.GetPingInterval()
		s.pingTimeout = hr.GetPingTimeout()
		go s.t.Run()
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
		case <-time.After(timeout):
			s.close(ReasonPingTimeout, nil)
			return
		case <-s.closeChan:
			return
		}
	}
}

func (s *clientSocket) maybeUpgrade(transports []string, upgrades []string) {
	if s.TransportName() == "websocket" {
		return
	} else if !findTransport(upgrades, "websocket") {
		return
	} else if !findTransport(transports, "websocket") {
		return
	}

	t := _websocket.NewClientTransport(s.sid, ProtocolVersion, *s.url, s.requestHeader, s.wsDialer)
	done := make(chan struct{})
	once := new(sync.Once)

	onPacket := func(packet *parser.Packet) {
		switch packet.Type {
		case parser.PacketTypePong:
			if string(packet.Data) != "probe" {
				s.onError(fmt.Errorf("upgrade failed: invalid packet received"))
				t.Close()
				return
			}

			once.Do(func() { close(done) })
			s.upgradeTo(t)
		default:
			t.Close()
			s.onError(fmt.Errorf("upgrade failed: invalid packet received"))
			return
		}
	}

	onClose := func(string, error) {}

	t.SetCallbacks(onPacket, onClose)

	_, err := t.Handshake()
	if err != nil {
		t.Close()
		s.onError(fmt.Errorf("upgrade failed: %w", err))
		return
	}

	go func() {
		select {
		case <-done:
			return
		case <-time.After(s.upgradeTimeout):
			t.Close()
			s.onError(fmt.Errorf("upgrade failed: upgradeTimeout exceeded"))
		}
	}()

	go t.Run()

	ping, err := parser.NewPacket(parser.PacketTypePing, false, []byte("probe"))
	if err != nil {
		t.Close()
		s.onError(fmt.Errorf("upgrade failed: %w", err))
		return
	}
	t.SendPacket(ping)
}

func (s *clientSocket) upgradeTo(t ClientTransport) {
	p, err := parser.NewPacket(parser.PacketTypeUpgrade, false, nil)
	if err != nil {
		s.onError(fmt.Errorf("upgrade failed: %w", err))
		return
	}

	t.SetCallbacks(s.onPacket, s.onTransportClose)

	s.tMu.Lock()
	defer s.tMu.Unlock()

	old := s.t
	s.t = t
	old.Discard()

	t.SendPacket(p)

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

func (s *clientSocket) onPacket(packet *parser.Packet) {
	switch packet.Type {
	case parser.PacketTypeMessage:
		s.callbacks.OnMessage(packet.Data, packet.IsBinary)
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
		s.sendPacket(pong)
	case parser.PacketTypeClose:
		s.tMu.RLock()
		defer s.tMu.RUnlock()
		s.t.Close()
	}
}

func (s *clientSocket) onError(err error) {
	if err != nil {
		s.callbacks.OnError(err)
	}
}

func (s *clientSocket) onTransportClose(name string, err error) {
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
	s.tMu.RLock()
	defer s.tMu.RUnlock()
	return s.t.Name()
}

func (s *clientSocket) SendMessage(data []byte, isBinary bool) {
	s.tMu.RLock()
	defer s.tMu.RUnlock()

	p, err := parser.NewPacket(parser.PacketTypeMessage, isBinary, data)
	if err != nil {
		s.onError(err)
		return
	}

	s.t.SendPacket(p)
}

func (s *clientSocket) sendPacket(p *parser.Packet) {
	s.tMu.RLock()
	defer s.tMu.RUnlock()
	s.t.SendPacket(p)
}

func (s *clientSocket) close(reason string, err error) {
	s.once.Do(func() {
		close(s.closeChan)
		defer s.callbacks.OnClose(reason, err)

		if reason != ReasonTransportClose && reason != ReasonTransportError {
			s.tMu.RLock()
			defer s.tMu.RUnlock()
			if s.t != nil {
				s.t.Close()
			}
		}
	})
}

func (s *clientSocket) Close() {
	s.close(ReasonForcedClose, nil)
}
