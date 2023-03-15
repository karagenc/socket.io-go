package adapter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	jsonparser "github.com/tomruk/socket.io-go/parser/json"
	"github.com/tomruk/socket.io-go/parser/json/serializer/stdjson"
)

func TestInMemoryAdapterAddRemove(t *testing.T) {
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
}

func newTestInMemoryAdapter() *inMemoryAdapter {
	// TODO: Add test socket store
	creator := NewInMemoryAdapterCreator()
	return creator(nil, jsonparser.NewCreator(0, stdjson.New())).(*inMemoryAdapter)
}
