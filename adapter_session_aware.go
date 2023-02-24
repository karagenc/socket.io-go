package sio

import (
	"math"
	"sync"
	"time"

	"github.com/tomruk/socket.io-go/parser"
)

type sessionWithTimestamp struct {
	SessionToPersist SessionToPersist
	DisconnectedAt   time.Time
}

func (s *sessionWithTimestamp) hasExpired(maxDisconnectDuration time.Duration) bool {
	return time.Now().Before(s.DisconnectedAt.Add(maxDisconnectDuration))
}

type persistedPacket struct {
	ID        string
	EmittedAt time.Time
	Buffers   [][]byte
	opts      *BroadcastOptions
}

func (p *persistedPacket) hasExpired(maxDisconnectDuration time.Duration) bool {
	return time.Now().Before(p.EmittedAt.Add(maxDisconnectDuration))
}

type sessionAwareAdapter struct {
	*inMemoryAdapter

	maxDisconnectDuration time.Duration
	yeaster               *yeaster

	sessions map[PrivateSessionID]*sessionWithTimestamp
	packets  []*persistedPacket
	mu       sync.Mutex
}

func newSessionAwareAdapter(namespace *Namespace, socketStore *NamespaceSocketStore) Adapter {
	s := &sessionAwareAdapter{
		inMemoryAdapter:       newInMemoryAdapter(namespace, socketStore).(*inMemoryAdapter),
		maxDisconnectDuration: namespace.server.connectionStateRecovery.MaxDisconnectionDuration,
		yeaster:               newYeaster(),
	}
	go s.cleaner()
	return s
}

func (a *sessionAwareAdapter) cleaner() {
	for {
		time.Sleep(60 * time.Second)

		a.mu.Lock()
		for sessionID, session := range a.sessions {
			if session.hasExpired(a.maxDisconnectDuration) {
				delete(a.sessions, sessionID)
			}
		}

		for i := len(a.packets) - 1; i >= 0; i-- {
			packet := a.packets[i]
			if packet.hasExpired(a.maxDisconnectDuration) {
				a.packets = append(a.packets[:i], a.packets[i+1:]...)
				break
			}
		}
		a.mu.Unlock()
	}
}

func (a *sessionAwareAdapter) PersistSession(session *SessionToPersist) {
	sessionWithTS := &sessionWithTimestamp{
		SessionToPersist: *session,
		DisconnectedAt:   time.Now(),
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.sessions[session.PID] = sessionWithTS
}

func (a *sessionAwareAdapter) RestoreSession(pid PrivateSessionID, offset string) *SessionToPersist {
	a.mu.Lock()
	defer a.mu.Unlock()
	session, ok := a.sessions[pid]
	if !ok {
		return nil
	}

	if session.hasExpired(a.maxDisconnectDuration) {
		delete(a.sessions, pid)
		return nil
	}

	index := -1
	for i, packet := range a.packets {
		if packet.ID == offset {
			index = i
			break
		}
	}
	if index == -1 {
		return nil
	}

	// TODO: Implement this
	/* var missedPackets []byte
	for i := index + 1; i < len(a.packets); i++ {
		packet := a.packets[i]
		if shouldIncludePacket(session.SessionToPersist.Rooms, packet.opts) {

		}
	} */

	// Return a copy to prevent race conditions.
	return &*&session.SessionToPersist
}

func shouldIncludePacket(sessionRooms []Room, opts *BroadcastOptions) bool {
	included := false
	for _, sessionRoom := range sessionRooms {
		if opts.Rooms.Contains(sessionRoom) {
			included = true
			break
		}
	}

	included = opts.Rooms.Cardinality() == 0 || included

	notExcluded := true
	for _, sessionRoom := range sessionRooms {
		if opts.Except.Contains(sessionRoom) {
			notExcluded = false
			break
		}
	}
	return included && notExcluded
}

func (a *sessionAwareAdapter) Broadcast(header *parser.PacketHeader, buffers [][]byte, opts *BroadcastOptions) {
	isEventPacket := header.Type == parser.PacketTypeEvent
	withoutAcknowledgement := header.ID == nil
	if isEventPacket && withoutAcknowledgement {
		a.mu.Lock()
		id := a.yeaster.yeast()
		// TODO: modify buffers
		packet := &persistedPacket{
			ID:        id,
			opts:      opts,
			EmittedAt: time.Now(),
			Buffers:   buffers,
		}
		a.packets = append(a.packets, packet)
		a.mu.Unlock()
	}
	a.inMemoryAdapter.Broadcast(header, buffers, opts)
}

type yeaster struct {
	alphabet string
	length   int
	m        map[rune]int

	seed float64
	prev string
}

func newYeaster() *yeaster {
	const alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz-_"
	y := &yeaster{
		alphabet: alphabet,
		length:   len(alphabet),
		m:        make(map[rune]int),
	}
	for i, c := range y.alphabet {
		y.m[c] = i
	}
	return y
}

func (y *yeaster) yeast() string {
	now := y.encode(float64(time.Now().Unix()))

	if now != y.prev {
		y.seed = 0
		y.prev = now
		return now
	}

	y.seed += 1
	return now + "." + y.encode(y.seed)
}

func (y *yeaster) encode(num float64) string {
	encoded := ""
	for {
		encoded = string(y.alphabet[int(num)%y.length]) + encoded
		num = math.Floor(num / float64(y.length))
		if num <= 0 {
			break
		}
	}
	return encoded
}

func (y *yeaster) decode(s string) int {
	decoded := 0
	for _, c := range s {
		decoded = decoded*y.length + y.m[c]
	}
	return decoded
}

var alphabet = []string{"0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz-_"}
