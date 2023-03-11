package sio

import (
	"reflect"
	"sync"

	"github.com/tomruk/socket.io-go/parser"
)

type queuedPacket struct {
	id     uint64
	header *parser.PacketHeader
	v      []any

	mu       *sync.Mutex
	tryCount int
	pending  bool
}

type clientPacketQueue struct {
	socket *clientSocket

	queuedPackets []*queuedPacket
	mu            sync.Mutex
}

func newClientPacketQueue(socket *clientSocket) *clientPacketQueue {
	return &clientPacketQueue{
		socket: socket,
	}
}

func (pq *clientPacketQueue) addToQueue(header *parser.PacketHeader, v []any) {
	haveAck := false
	f := v[len(v)-1]
	rv := reflect.ValueOf(f)
	rt := reflect.TypeOf(f)
	if rt.Kind() == reflect.Func {
		v = v[:len(v)-1]
		haveAck = true
	}

	packet := &queuedPacket{
		id:     pq.socket.nextAckID(),
		header: header,
		mu:     new(sync.Mutex),
	}

	replacementAck := func(args []reflect.Value) (results []reflect.Value) {
		errV := args[0]
		hasError := errV.IsNil() == false

		if hasError {
			packet.mu.Lock()
			tryCount := packet.tryCount
			packet.mu.Unlock()
			if tryCount > pq.socket.config.Retries {
				pq.mu.Lock()
				pq.queuedPackets = pq.queuedPackets[1:]
				pq.mu.Unlock()
				if haveAck {
					rv.Call(args)
				}
			}
		} else {
			pq.mu.Lock()
			pq.queuedPackets = pq.queuedPackets[1:]
			pq.mu.Unlock()
			if haveAck {
				rv.Call(args)
			}
		}
		packet.mu.Lock()
		packet.pending = false
		packet.mu.Unlock()
		pq.drainQueue(false)
		return nil // TODO: Should this be kept nil?
	}

	if haveAck {
		in, out, variadic := pq.dismantleAckFunc(rt)
		// TODO: Check if first element of `in` is an error?
		ackF := reflect.MakeFunc(reflect.FuncOf(in, out, variadic), replacementAck)
		f = ackF.Interface()
	} else {
		in := []reflect.Type{reflectError}
		ackF := reflect.MakeFunc(reflect.FuncOf(in, nil, false), replacementAck)
		f = ackF.Interface()
	}
	v = append(v, f)
	packet.v = v

	pq.mu.Lock()
	pq.queuedPackets = append(pq.queuedPackets, packet)
	pq.mu.Unlock()
	pq.drainQueue(false)
}

func (pq *clientPacketQueue) drainQueue(force bool) {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	if !pq.socket.Connected() || len(pq.queuedPackets) == 0 {
		return
	}

	packet := pq.queuedPackets[0]
	packet.mu.Lock()
	pending := packet.pending
	if pending && !force {
		packet.mu.Unlock()
		return
	}
	packet.pending = true
	packet.tryCount++
	packet.mu.Unlock()

	pq.socket.emit("", 0, true, packet.v...)
}

func (pq *clientPacketQueue) dismantleAckFunc(rt reflect.Type) (in, out []reflect.Type, variadic bool) {
	in = make([]reflect.Type, rt.NumIn())
	out = make([]reflect.Type, rt.NumOut())

	for i := range in {
		in[i] = rt.In(i)
	}
	for i := range in {
		out[i] = rt.Out(i)
	}
	variadic = rt.IsVariadic()
	return
}
