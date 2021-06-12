package sio

import (
	eio "github.com/tomruk/socket.io-go/engine.io"
	eioparser "github.com/tomruk/socket.io-go/engine.io/parser"
	"github.com/tomruk/socket.io-go/parser"
)

type clientSocket struct {
	id        string // TODO: Guard with mutex or make it atomic.
	namespace string
	eio       eio.Socket
	parser    parser.Parser
}

func newClientSocket(_eio eio.Socket, parser parser.Parser) (*clientSocket, error) {
	id, err := eio.GenerateBase64ID(eio.Base64IDSize)
	if err != nil {
		return nil, err
	}

	s := &clientSocket{
		id:  id,
		eio: _eio,
	}
	return s, nil
}

func (s *clientSocket) ID() string {
	return s.id
}

func (s *clientSocket) Emit(v ...interface{}) {
	header := parser.PacketHeader{
		Type:      parser.PacketTypeEvent,
		Namespace: s.namespace,
	}

	buffers, err := s.parser.Encode(&header, &v)
	if err != nil {
		// TODO: onError?
		return
	}

	if len(buffers) > 0 {
		packets := make([]*eioparser.Packet, len(buffers))
		buf := buffers[0]
		buffers = buffers[1:]

		packets[0], err = eioparser.NewPacket(eioparser.PacketTypeMessage, false, buf)
		if err != nil {
			// TODO: onError?
			return
		}

		for i, attachment := range buffers {
			packets[i+1], err = eioparser.NewPacket(eioparser.PacketTypeMessage, true, attachment)
			if err != nil {
				// TODO: onError?
				return
			}
		}

		s.eio.Send(packets...)
	}
}
