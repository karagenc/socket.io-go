package eio

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/tomruk/socket.io-go/engine.io/parser"
)

type serverSocket struct {
	id           string
	upgrades     []string
	pingInterval time.Duration
	pingTimeout  time.Duration

	t   ServerTransport
	tMu sync.RWMutex

	callbacks atomic.Value

	pongChan chan struct{}

	onClose   func(sid string)
	closeChan chan struct{}
	closeOnce sync.Once
}

func newServerSocket(id string, upgrades []string, t ServerTransport, pingInterval time.Duration, pingTimeout time.Duration, onClose func(sid string)) *serverSocket {
	// The function below might be nil for testing purposes. See: sstore_test.go
	if onClose == nil {
		onClose = func(sid string) {}
	}

	s := &serverSocket{
		id:           id,
		upgrades:     upgrades,
		pingInterval: pingInterval,
		pingTimeout:  pingTimeout,

		t: t,

		pongChan: make(chan struct{}, 1),

		closeChan: make(chan struct{}),
		onClose:   onClose,
	}

	s.setCallbacks(nil)
	t.Callbacks().Set(s.onPacket, s.onTransportClose)
	go s.pingPong(pingInterval, pingTimeout)

	return s
}

func (s *serverSocket) getCallbacks() *Callbacks {
	callbacks, _ := s.callbacks.Load().(*Callbacks)
	return callbacks
}

func (s *serverSocket) setCallbacks(callbacks *Callbacks) {
	if callbacks == nil {
		callbacks = new(Callbacks)
	}
	callbacks.setMissing()

	// Copy the callbacks so the user can't change them.
	c := *callbacks
	s.callbacks.Store(&c)
}

func (s *serverSocket) ID() string {
	return s.id
}

func (s *serverSocket) Transport() ServerTransport {
	s.tMu.RLock()
	defer s.tMu.RUnlock()
	return s.t
}

func (s *serverSocket) TransportName() string {
	s.tMu.RLock()
	defer s.tMu.RUnlock()
	return s.t.Name()
}

func (s *serverSocket) Upgrades() []string {
	return s.upgrades
}

func (s *serverSocket) PingInterval() time.Duration {
	return s.pingInterval
}

func (s *serverSocket) PingTimeout() time.Duration {
	return s.pingTimeout
}

func (s *serverSocket) UpgradeTo(t ServerTransport) {
	t.Callbacks().Set(s.onPacket, s.onTransportClose)

	s.tMu.Lock()
	defer s.tMu.Unlock()

	old := s.t
	s.t = t
	old.Discard()

	// Get the queued packets from the old transport and send them with the new one.
	qp := old.QueuedPackets()
	for _, p := range qp {
		if p.Type != parser.PacketTypeNoop {
			t.Send(p)
		}
	}
}

func (s *serverSocket) pingPong(pingInterval time.Duration, pingTimeout time.Duration) {
	for {
		time.Sleep(pingInterval)

		select {
		case <-s.closeChan:
			return
		default:
		}

		ping, err := parser.NewPacket(parser.PacketTypePing, false, nil)
		if err != nil {
			s.onError(err)
			return
		}
		s.Send(ping)

		select {
		case <-s.pongChan:
		case <-time.After(pingTimeout):
			s.close(ReasonPingTimeout, nil)
			return
		case <-s.closeChan:
			return
		}
	}
}

func (s *serverSocket) onPacket(packets ...*parser.Packet) {
	s.getCallbacks().OnPacket(packets...)

	for _, packet := range packets {
		s.handlePacket(packet)
	}
}

func (s *serverSocket) handlePacket(packet *parser.Packet) {
	switch packet.Type {
	case parser.PacketTypePong:
		s.onPong()
	case parser.PacketTypeClose:
		s.tMu.RLock()
		defer s.tMu.RUnlock()
		s.t.Close()
	}
}

func (s *serverSocket) onPong() {
	select {
	case s.pongChan <- struct{}{}:
	default:
	}
}

func (s *serverSocket) onError(err error) {
	if err != nil {
		s.getCallbacks().OnError(err)
	}
}

func (s *serverSocket) Send(packets ...*parser.Packet) {
	s.tMu.RLock()
	defer s.tMu.RUnlock()
	s.t.Send(packets...)
}

func (s *serverSocket) onTransportClose(name string, err error) {
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

func (s *serverSocket) close(reason string, err error) {
	s.closeOnce.Do(func() {
		close(s.closeChan)
		defer s.onClose(s.id)

		defer s.getCallbacks().OnClose(reason, err)

		if reason != ReasonTransportClose && reason != ReasonTransportError {
			s.tMu.RLock()
			defer s.tMu.RUnlock()
			if s.t != nil {
				s.t.Close()
			}
		}
	})
}

func (s *serverSocket) Close() {
	s.close(ReasonForcedClose, nil)
}
