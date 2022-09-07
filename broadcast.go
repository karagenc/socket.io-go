package sio

import (
	"fmt"
	"reflect"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/tomruk/socket.io-go/parser"
)

type broadcastOptions struct {
	Rooms  map[string]interface{}
	Except map[string]interface{}
	Flags  BroadcastFlags
}

func newBroadcastOptions() *broadcastOptions {
	return &broadcastOptions{
		Rooms:  make(map[string]interface{}),
		Except: make(map[string]interface{}),
	}
}

type BroadcastFlags struct {
	Compress bool
	Local    bool // TODO: Remove this?
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

func (b *broadcastOperator) Clone() *broadcastOperator {
	n := *b
	n.rooms = n.rooms.Clone()
	n.exceptRooms = n.exceptRooms.Clone()
	return &n
}

func (b *broadcastOperator) To(room ...string) *broadcastOperator {
	n := b.Clone()
	for _, r := range room {
		n.rooms.Add(r)
	}
	return n
}

func (b *broadcastOperator) In(room ...string) *broadcastOperator {
	return b.To(room...)
}

func (b *broadcastOperator) Except(room ...string) *broadcastOperator {
	n := b.Clone()
	for _, r := range room {
		n.exceptRooms.Add(r)
	}
	return n
}

func (b *broadcastOperator) Compress(compress bool) *broadcastOperator {
	n := b.Clone()
	n.flags.Compress = compress
	return n
}

func (b *broadcastOperator) Local() *broadcastOperator {
	n := b.Clone()
	n.flags.Local = true
	return n
}

func (b *broadcastOperator) Emit(eventName string, v ...interface{}) {
	header := parser.PacketHeader{
		Type:      parser.PacketTypeEvent,
		Namespace: b.nsp,
	}

	if IsEventReserved(eventName) {
		panic(fmt.Errorf("broadcastOperator.Emit: attempted to emit to a reserved event"))
	}

	v = append([]interface{}{eventName}, v...)

	if len(v) > 0 {
		f := v[len(v)-1]
		rt := reflect.TypeOf(f)

		if rt.Kind() == reflect.Func {
			panic("callbacks are not supported when broadcasting")
		}
	}

	buffers, err := b.parser.Encode(&header, &v)
	if err != nil {
		b.onError(err)
		return
	}

	opts := newBroadcastOptions()
	opts.Flags = b.flags

	// Instead of s.conn.sendBuffers(buffers...)
	// we use:
	b.adapter.Broadcast(buffers, opts)
}

func (b *broadcastOperator) onError(err error) {
	// TODO: Error?
}
