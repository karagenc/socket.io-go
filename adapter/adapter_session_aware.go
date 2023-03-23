package adapter

import (
	"sync"
	"time"

	"github.com/tomruk/socket.io-go/parser"
	"github.com/tomruk/yeast"
)

type (
	sessionAwareAdapter struct {
		*inMemoryAdapter

		maxDisconnectDuration time.Duration
		yeaster               *yeast.Yeaster
		cleanerDuration       time.Duration

		sessions map[PrivateSessionID]*sessionWithTimestamp
		packets  []*PersistedPacket
		mu       sync.Mutex
	}

	sessionWithTimestamp struct {
		SessionToPersist SessionToPersist
		DisconnectedAt   time.Time
	}
)

func (s *sessionWithTimestamp) hasExpired(maxDisconnectDuration time.Duration) bool {
	return time.Now().After(s.DisconnectedAt.Add(maxDisconnectDuration))
}

func NewSessionAwareAdapterCreator(maxDisconnectionDuration time.Duration) Creator {
	const cleanerDuration = time.Minute * 1
	creator := NewInMemoryAdapterCreator()
	return func(socketStore SocketStore, parserCreator parser.Creator) Adapter {
		inMemoryAdapter := creator(socketStore, parserCreator).(*inMemoryAdapter)
		return newSessionAwareAdapter(
			inMemoryAdapter,
			maxDisconnectionDuration,
			cleanerDuration,
			socketStore,
			parserCreator,
		)
	}
}

func newSessionAwareAdapter(inMemoryAdapter *inMemoryAdapter,
	maxDisconnectionDuration,
	cleanerDuration time.Duration,
	socketStore SocketStore,
	parserCreator parser.Creator,
) *sessionAwareAdapter {
	a := &sessionAwareAdapter{
		inMemoryAdapter:       inMemoryAdapter,
		maxDisconnectDuration: maxDisconnectionDuration,
		sessions:              make(map[PrivateSessionID]*sessionWithTimestamp),
		yeaster:               yeast.New(),
		cleanerDuration:       cleanerDuration,
	}
	if cleanerDuration != 0 {
		go a.cleaner()
	}
	return a
}

func (a *sessionAwareAdapter) cleaner() {
	for {
		time.Sleep(a.cleanerDuration)

		a.mu.Lock()
		for sessionID, session := range a.sessions {
			if session.hasExpired(a.maxDisconnectDuration) {
				delete(a.sessions, sessionID)
			}
		}

		for i := len(a.packets) - 1; i >= 0; i-- {
			packet := a.packets[i]
			if packet.HasExpired(a.maxDisconnectDuration) {
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

func (a *sessionAwareAdapter) RestoreSession(
	pid PrivateSessionID,
	offset string,
) (session *SessionToPersist, ok bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	sessionWithTS, ok := a.sessions[pid]
	if !ok {
		return nil, false
	}

	if sessionWithTS.hasExpired(a.maxDisconnectDuration) {
		delete(a.sessions, pid)
		return nil, false
	}

	index := -1
	for i, packet := range a.packets {
		if packet.ID == offset {
			index = i
			break
		}
	}
	if index == -1 {
		return nil, false
	}

	var missedPackets []*PersistedPacket
	for i := index + 1; i < len(a.packets); i++ {
		packet := a.packets[i]
		if shouldIncludePacket(sessionWithTS.SessionToPersist.Rooms, packet.Opts) {
			missedPackets = append(missedPackets, packet)
		}
	}

	// Return a copy to prevent race conditions.
	session = &*&sessionWithTS.SessionToPersist
	session.MissedPackets = missedPackets
	return session, true
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

func (a *sessionAwareAdapter) Broadcast(header *parser.PacketHeader, v []any, opts *BroadcastOptions) {
	isEventPacket := header.Type == parser.PacketTypeEvent
	withoutAcknowledgement := header.ID == nil
	if isEventPacket && withoutAcknowledgement {
		a.mu.Lock()
		id := a.yeaster.Yeast()
		v = append(v, id)

		packet := &PersistedPacket{
			ID:        id,
			Opts:      opts,
			EmittedAt: time.Now(),
			Data:      v,
		}
		a.packets = append(a.packets, packet)
		a.mu.Unlock()
	}
	a.inMemoryAdapter.Broadcast(header, v, opts)
}
