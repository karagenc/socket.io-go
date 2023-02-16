package sio

import (
	"fmt"
	"reflect"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/tomruk/socket.io-go/parser"
)

type BroadcastOptions struct {
	Rooms  mapset.Set[string]
	Except mapset.Set[string]
	Flags  BroadcastFlags
}

func NewBroadcastOptions() *BroadcastOptions {
	return &BroadcastOptions{
		Rooms:  mapset.NewSet[string](),
		Except: mapset.NewSet[string](),
	}
}

// These flags are unused at the moment, but for
// compatibility with the socket.io API, these stay here.
type BroadcastFlags struct {
	Compress bool
	Local    bool
}

type broadcastOperator struct {
	nsp         string
	adapter     Adapter
	parser      parser.Parser
	rooms       mapset.Set[string]
	exceptRooms mapset.Set[string]
	flags       BroadcastFlags
}

func newBroadcastOperator(nsp string, adapter Adapter, parser parser.Parser) *broadcastOperator {
	return &broadcastOperator{
		nsp:         nsp,
		adapter:     adapter,
		parser:      parser,
		rooms:       mapset.NewSet[string](),
		exceptRooms: mapset.NewSet[string](),
	}
}

// Emits an event to all choosen clients.
func (b *broadcastOperator) Emit(eventName string, v ...interface{}) {
	header := parser.PacketHeader{
		Type:      parser.PacketTypeEvent,
		Namespace: b.nsp,
	}

	if IsEventReservedForServer(eventName) {
		panic("sio: broadcastOperator.Emit: attempted to emit to a reserved event")
	}

	v = append([]interface{}{eventName}, v...)

	if len(v) > 0 {
		f := v[len(v)-1]
		rt := reflect.TypeOf(f)

		if rt.Kind() == reflect.Func {
			panic("sio: broadcastOperator.Emit: callbacks are not supported when broadcasting")
		}
	}

	buffers, err := b.parser.Encode(&header, &v)
	if err != nil {
		panic(fmt.Errorf("sio: %w", err))
	}

	opts := NewBroadcastOptions()
	opts.Flags = b.flags

	// Instead of s.conn.sendBuffers(buffers...)
	// we use:
	b.adapter.Broadcast(buffers, opts)

	a := newAckHandler(func(msg string) {
		// TODO: Implement this
	})

	b.adapter.BroadcastWithAck("TODO: packetID", buffers, opts, a)
}

// Sets a modifier for a subsequent event emission that the event
// will only be broadcast to clients that have joined the given room.
//
// To emit to multiple rooms, you can call To several times.
func (b *broadcastOperator) To(room ...string) *broadcastOperator {
	n := *b
	rooms := b.rooms.Clone()
	for _, r := range room {
		rooms.Add(r)
	}
	n.rooms = rooms
	return &n
}

// Alias of To(...)
func (b *broadcastOperator) In(room ...string) *broadcastOperator {
	return b.To(room...)
}

// Sets a modifier for a subsequent event emission that the event
// will only be broadcast to clients that have not joined the given rooms.
func (b *broadcastOperator) Except(room ...string) *broadcastOperator {
	n := *b
	exceptRooms := b.exceptRooms.Clone()
	for _, r := range room {
		n.exceptRooms.Add(r)
	}
	n.exceptRooms = exceptRooms
	return &n
}

// Compression flag is unused at the moment, thus setting this will have no effect on compression.
func (b *broadcastOperator) Compress(compress bool) *broadcastOperator {
	n := *b
	n.flags.Compress = compress
	return &n
}

// Sets a modifier for a subsequent event emission that the event data will only be broadcast to the current node (when scaling to multiple nodes).
//
// See: https://socket.io/docs/v4/using-multiple-nodes
func (b *broadcastOperator) Local() *broadcastOperator {
	n := *b
	n.flags.Local = true
	return &n
}

// Gets a list of socket IDs connected to this namespace (across all nodes if applicable).
func (b *broadcastOperator) AllSockets() (sids mapset.Set[string]) {
	return b.adapter.Sockets(b.rooms)
}

// Makes the matching socket instances join the specified rooms.
func (b *broadcastOperator) SocketsJoin(room ...string) {
	opts := NewBroadcastOptions()
	opts.Rooms = b.rooms.Clone()
	opts.Except = b.exceptRooms.Clone()
	opts.Flags = b.flags

	b.adapter.AddSockets(opts, room...)
}

// Makes the matching socket instances leave the specified rooms.
func (b *broadcastOperator) SocketsLeave(room ...string) {
	opts := NewBroadcastOptions()
	opts.Rooms = b.rooms.Clone()
	opts.Except = b.exceptRooms.Clone()
	opts.Flags = b.flags

	b.adapter.DelSockets(opts, room...)
}

// Makes the matching socket instances disconnect from the namespace.
//
// If value of close is true, closes the underlying connection. Otherwise, it just disconnects the namespace.
func (b *broadcastOperator) DisconnectSockets(close bool) {
	opts := NewBroadcastOptions()
	opts.Rooms = b.rooms.Clone()
	opts.Except = b.exceptRooms.Clone()
	opts.Flags = b.flags

	b.adapter.DisconnectSockets(opts, close)
}
