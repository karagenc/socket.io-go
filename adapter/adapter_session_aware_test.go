package adapter

import (
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/tomruk/socket.io-go/parser"
	jsonparser "github.com/tomruk/socket.io-go/parser/json"
	"github.com/tomruk/socket.io-go/parser/json/serializer/stdjson"
)

func TestPersistAndRestoreSession(t *testing.T) {
	adapter := newTestSessionAwareAdapter()
	adapter.AddAll("s1", []Room{"r1"})
	store := adapter.sockets.(*testSocketStore)
	store.Set(newTestSocketWithID("s1"))

	adapter.PersistSession(&SessionToPersist{
		SID:   "s1",
		PID:   "p1",
		Rooms: []Room{"r1", "r2"},
	})

	header := parser.PacketHeader{
		Namespace: "/",
		Type:      parser.PacketTypeEvent,
	}
	opts := NewBroadcastOptions()
	offset := ""
	v := []any{"123"}

	store.sendBuffers = func(sid SocketID, buffers [][]byte) (ok bool) {
		assert.Equal(t, SocketID("s1"), sid)

		// Yank the offset with a regex.
		re := regexp.MustCompile(`.*".*".*"(.*)"`)
		_offset := re.FindStringSubmatch(string(buffers[0]))
		//fmt.Printf("'%s'\n", _offset[1])
		offset = _offset[1]
		return true
	}

	adapter.Broadcast(&header, v, opts)

	session, ok := adapter.RestoreSession("p1", offset)
	if !assert.True(t, ok) {
		return
	}
	assert.Equal(t, SocketID("s1"), session.SID)
	assert.Equal(t, PrivateSessionID("p1"), session.PID)
	assert.Equal(t, 0, len(session.MissedPackets))
}

func newTestSessionAwareAdapter() *sessionAwareAdapter {
	const maxDisconnectionDuration = 5 * time.Second
	creator := NewSessionAwareAdapterCreator(maxDisconnectionDuration)
	return creator(newTestSocketStore(), jsonparser.NewCreator(0, stdjson.New())).(*sessionAwareAdapter)
}
