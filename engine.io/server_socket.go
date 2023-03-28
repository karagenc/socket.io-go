package eio

import (
	"sync/atomic"
	"time"

	"github.com/tomruk/socket.io-go/internal/sync"

	"github.com/tomruk/socket.io-go/engine.io/parser"
	"github.com/tomruk/socket.io-go/engine.io/transport"
)

type serverSocket struct {
	id           string
	upgrades     []string
	pingInterval time.Duration
	pingTimeout  time.Duration

	transport   ServerTransport
	transportMu sync.RWMutex

	callbacks atomic.Value

	pongChan chan struct{}

	onClose   func(sid string)
	closeChan chan struct{}
	closeOnce sync.Once

	debug Debugger
}

func newServerSocket(
	id string,
	upgrades []string,
	transport ServerTransport,
	callbacks *transport.Callbacks,
	pingInterval time.Duration,
	pingTimeout time.Duration,
	debug Debugger,
	onClose func(sid string),
) *serverSocket {
	// The function below might be nil for testing purposes. See: sstore_test.go
	if onClose == nil {
		onClose = func(sid string) {}
	}

	s := &serverSocket{
		id:           id,
		upgrades:     upgrades,
		pingInterval: pingInterval,
		pingTimeout:  pingTimeout,

		transport: transport,

		pongChan: make(chan struct{}, 1),

		closeChan: make(chan struct{}),
		onClose:   onClose,
		debug:     debug.WithContext("[eio/server] Socket with ID: " + id),
	}

	s.setCallbacks(nil)
	callbacks.Set(s.onPacket, s.onTransportClose)
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
	s.transportMu.RLock()
	defer s.transportMu.RUnlock()
	return s.transport
}

func (s *serverSocket) TransportName() string {
	s.transportMu.RLock()
	defer s.transportMu.RUnlock()
	return s.transport.Name()
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

func (s *serverSocket) upgradeTo(t ServerTransport, c *transport.Callbacks) {
	s.debug.Log("UpgradeTo", t.Name())

	c.Set(s.onPacket, s.onTransportClose)

	s.transportMu.Lock()
	defer s.transportMu.Unlock()

	old := s.transport
	s.transport = t
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
			s.debug.Log("pingPong", "`closeChan` was closed")
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
			s.debug.Log("pingPong", "pong received")
		case <-time.After(pingTimeout):
			s.debug.Log("pingPong", "pingTimeout exceeded")
			s.close(ReasonPingTimeout, nil)
			return
		case <-s.closeChan:
			s.debug.Log("pingPong", "`closeChan` was closed")
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
		s.transportMu.RLock()
		defer s.transportMu.RUnlock()
		s.transport.Close()
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
	s.transportMu.RLock()
	defer s.transportMu.RUnlock()
	s.transport.Send(packets...)
}

func (s *serverSocket) onTransportClose(name string, err error) {
	go func() { // <- To prevent s.TransportName() from blocking (locks transportMu).
		s.debug.Log("Transport", name, "closed. Error", err)

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

func (s *serverSocket) close(reason Reason, err error) {
	s.debug.Log("Going to close the socket if it is not already closed. Reason", reason)

	s.closeOnce.Do(func() {
		s.debug.Log("Going to close the socket. It is not already closed. Reason", reason)
		close(s.closeChan)
		defer s.onClose(s.id)

		defer s.getCallbacks().OnClose(reason, err)

		if reason != ReasonTransportClose && reason != ReasonTransportError {
			s.transportMu.RLock()
			defer s.transportMu.RUnlock()
			if s.transport != nil {
				s.transport.Close()
			}
		}
	})
}

func (s *serverSocket) Close() {
	s.close(ReasonForcedClose, nil)
}
