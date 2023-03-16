package adapter

import (
	"testing"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/stretchr/testify/assert"
	jsonparser "github.com/tomruk/socket.io-go/parser/json"
	"github.com/tomruk/socket.io-go/parser/json/serializer/stdjson"
)

func TestInMemoryAdapterAddDelete(t *testing.T) {
	adapter := newTestInMemoryAdapter()
	adapter.AddAll("s1", []Room{"r1", "r2"})
	adapter.AddAll("s2", []Room{"r2", "r3"})

	_, ok := adapter.rooms["r1"]
	assert.True(t, ok)
	_, ok = adapter.rooms["r2"]
	assert.True(t, ok)
	_, ok = adapter.rooms["r3"]
	assert.True(t, ok)
	_, ok = adapter.rooms["r4"]
	assert.False(t, ok)

	_, ok = adapter.sids["s1"]
	assert.True(t, ok)
	_, ok = adapter.sids["s2"]
	assert.True(t, ok)
	_, ok = adapter.sids["s3"]
	assert.False(t, ok)

	adapter.Delete("s1", "r1")
	_, ok = adapter.rooms["r1"]
	assert.False(t, ok)

	adapter.DeleteAll("s2")
	_, ok = adapter.sids["s2"]
	assert.False(t, ok)
	_, ok = adapter.rooms["r2"]
	assert.True(t, ok)
	_, ok = adapter.rooms["r3"]
	assert.False(t, ok)
}

func TestSockets(t *testing.T) {
	adapter := newTestInMemoryAdapter()
	store := adapter.sockets.(*testSocketStore)
	store.Set(newTestSocketWithID("s1"))
	store.Set(newTestSocketWithID("s2"))
	store.Set(newTestSocketWithID("s3"))

	adapter.AddAll("s1", []Room{"r1", "r2"})
	adapter.AddAll("s2", []Room{"r2", "r3"})
	adapter.AddAll("s3", []Room{"r3"})

	sockets := adapter.Sockets(mapset.NewThreadUnsafeSet[Room]())
	assert.Equal(t, 3, sockets.Cardinality())

	sockets = adapter.Sockets(mapset.NewThreadUnsafeSet[Room]("r2"))
	assert.Equal(t, 2, sockets.Cardinality())
	sockets = adapter.Sockets(mapset.NewThreadUnsafeSet[Room]("r4"))
	assert.Equal(t, 0, sockets.Cardinality())
}

func newTestInMemoryAdapter() *inMemoryAdapter {
	creator := NewInMemoryAdapterCreator()
	return creator(newTestSocketStore(), jsonparser.NewCreator(0, stdjson.New())).(*inMemoryAdapter)
}
