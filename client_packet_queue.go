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
	queuedPackets []*queuedPacket
	socket        *clientSocket
}

func newClientPacketQueue(socket *clientSocket) *clientPacketQueue {
	return &clientPacketQueue{
		socket: socket,
	}
}

func (q *clientPacketQueue) addToQueue(header *parser.PacketHeader, v []any) {
	haveAck := false
	f := v[len(v)-1]
	rt := reflect.TypeOf(f)
	if rt.Kind() == reflect.Func {
		v = v[:len(v)-1]
		haveAck = true
	}

	packet := &queuedPacket{
		id:     q.socket.nextAckID(),
		header: header,
		v:      v,
		mu:     new(sync.Mutex),
	}

	var h *ackHandler
	if haveAck {
		h = newAckHandlerWithTimeout(func(err error, handler *ackHandler) error {
			if err != nil {
				packet.mu.Lock()
				tryCount := packet.tryCount
				packet.mu.Unlock()
				if tryCount > q.socket.config.Retries {

				}
			} else {

			}
		}, f)
	} else {
		h = newAckHandler(f, true)
		q.socket.acksMu.Lock()

		q.socket.acksMu.Unlock()
	}
}
