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
	debug  Debugger

	mu            sync.Mutex
	seq           uint64
	queuedPackets []*queuedPacket
}

func newClientPacketQueue(socket *clientSocket) *clientPacketQueue {
	debug := socket.debug.WithDynamicContext("clientPacketQueue with socket ID", func() string {
		return string(socket.ID())
	})
	return &clientPacketQueue{
		debug:  debug,
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
		id:     pq.nextSeq(),
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
				pq.debug.Log("Packet with ID", packet.id, "discarded after", tryCount)
				pq.mu.Lock()
				pq.queuedPackets = pq.queuedPackets[1:]
				pq.mu.Unlock()
				if haveAck {
					rv.Call(args)
				}
			}
		} else {
			pq.debug.Log("Packet with ID", packet.id, "successfully sent")
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
		return nil
	}

	if haveAck {
		err := doesAckHandlerHasAnError(f)
		if err != nil {
			panic(err)
		}
		err = doesAckHandlerHasReturnValues(f)
		if err != nil {
			panic(err)
		}
		in, variadic := dismantleAckFunc(rt)
		ackF := reflect.MakeFunc(reflect.FuncOf(in, nil, variadic), replacementAck)
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
	pq.debug.Log("Draining queue")
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
		pq.debug.Log("Packet with ID", packet.id, "has already been sent and is waiting for an ack")
		return
	}
	packet.pending = true
	packet.tryCount++
	tryCount := packet.tryCount
	packet.mu.Unlock()

	pq.debug.Log("Sending packet with ID", packet.id, "try", tryCount)
	pq.socket.emit("", 0, false, true, packet.v...)
}

func (pq *clientPacketQueue) nextSeq() uint64 {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	seq := pq.seq
	pq.seq++
	return seq
}
