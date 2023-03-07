package sio

import "github.com/tomruk/socket.io-go/parser"

type queuedPacket struct {
	id     int
	header *parser.PacketHeader
	v      []any
}

type clientPacketQueue struct {
	queuedPackets []*queuedPacket
}

func newClientPacketQueue() *clientPacketQueue {
	return &clientPacketQueue{}
}

func (q *clientPacketQueue) addToQueue(header *parser.PacketHeader, v []any) {

}
