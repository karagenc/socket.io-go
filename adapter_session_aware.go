package sio

import (
	"sync"
	"time"
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
	Data      []byte
	opts      *BroadcastOptions
}

func (p *persistedPacket) hasExpired(maxDisconnectDuration time.Duration) bool {
	return time.Now().Before(p.EmittedAt.Add(maxDisconnectDuration))
}

type sessionAwareAdapter struct {
	*inMemoryAdapter

	maxDisconnectDuration time.Duration

	sessions map[PrivateSessionID]*sessionWithTimestamp
	packets  []*persistedPacket
	mu       sync.Mutex
}

func newSessionAwareAdapter(namespace *Namespace, socketStore *NamespaceSocketStore) Adapter {
	s := &sessionAwareAdapter{
		maxDisconnectDuration: namespace.server.connectionStateRecovery.MaxDisconnectionDuration,
		inMemoryAdapter:       newInMemoryAdapter(namespace, socketStore).(*inMemoryAdapter),
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
