package sio

import (
	"sync"
	"time"
)

type sessionWithTimestamp struct {
	SessionToPersist SessionToPersist
	DisconnectedAt   time.Time
}

type persistedPacket struct {
	ID        string
	EmittedAt int
	Data      []byte
	opts      *BroadcastOptions
}

type sessionAwareAdapter struct {
	*inMemoryAdapter

	maxDisconnectDuration time.Duration

	sessions map[PrivateSessionID]*sessionWithTimestamp
	packets  []*persistedPacket
	mu       sync.Mutex
}

func newSessionAwareAdapter(namespace *Namespace, socketStore *NamespaceSocketStore) Adapter {
	go func() {

	}()
	return &sessionAwareAdapter{
		maxDisconnectDuration: namespace.server.connectionStateRecovery.MaxDisconnectionDuration,
		inMemoryAdapter:       newInMemoryAdapter(namespace, socketStore).(*inMemoryAdapter),
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

	hasExpired := time.Now().Before(session.DisconnectedAt.Add(a.maxDisconnectDuration))
	if hasExpired {
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
