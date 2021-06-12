package sio

import (
	eio "github.com/tomruk/socket.io-go/engine.io"
	eioparser "github.com/tomruk/socket.io-go/engine.io/parser"
	"github.com/tomruk/socket.io-go/parser"
)

type serverSocket struct {
	id     string
	eio    eio.Socket
	parser parser.Parser
}

func newServerSocket(_eio eio.Socket, creator parser.Creator) (*serverSocket, *eio.Callbacks, error) {
	id, err := eio.GenerateBase64ID(eio.Base64IDSize)
	if err != nil {
		return nil, nil, err
	}

	s := &serverSocket{
		id:     id,
		eio:    _eio,
		parser: creator(),
	}

	callbacks := &eio.Callbacks{
		OnPacket: s.onEIOPacket,
		OnError:  s.onError,
		OnClose:  s.onClose,
	}

	return s, callbacks, nil
}

func (s *serverSocket) onEIOPacket(packet *eioparser.Packet) {
	if packet.Type != eioparser.PacketTypeMessage {
		return
	}
	s.parser.Add(packet.Data, s.onFinish)
}

func (s *serverSocket) onFinish(header *parser.PacketHeader, eventName string, decode parser.Decode) {

}

func (s *serverSocket) onError(err error) {

}

func (s *serverSocket) onClose(reason string, err error) {

}

func (s *serverSocket) ID() string {
	return s.id
}

func (s *serverSocket) Emit(v ...interface{}) {

}
