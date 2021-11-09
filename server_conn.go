package sio

import (
	"sync"

	eio "github.com/tomruk/socket.io-go/engine.io"
	eioparser "github.com/tomruk/socket.io-go/engine.io/parser"
	"github.com/tomruk/socket.io-go/parser"
)

type serverSocketStore struct {
	sockets map[string]*serverSocket
	mu      sync.Mutex
}

func newServerSocketStore() *serverSocketStore {
	return &serverSocketStore{
		sockets: make(map[string]*serverSocket),
	}
}

func (s *serverSocketStore) Get(sid string) (ss *serverSocket, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ss, ok = s.sockets[sid]
	return
}

func (s *serverSocketStore) GetAll() (sockets []*serverSocket) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sockets = make([]*serverSocket, len(s.sockets))
	i := 0
	for _, ss := range s.sockets {
		sockets[i] = ss
		i++
	}
	return
}

func (s *serverSocketStore) Set(ss *serverSocket) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sockets[ss.ID()] = ss
}

func (s *serverSocketStore) Remove(sid string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sockets, sid)
}

// This struct represents a connection to the server.
//
// This is the equivalent of the Client class at: https://github.com/socketio/socket.io/blob/4.3.2/lib/client.ts#L21
type serverConn struct {
	id  string
	eio eio.Socket

	sockets *serverSocketStore

	// This mutex is used for protecting parser from concurrent calls.
	// Due to the modular and concurrent nature of Engine.IO,
	// we should use a mutex to ensure the Engine.IO doesn't access
	// the parser's Add method from multiple goroutines.
	parserMu sync.Mutex
	parser   parser.Parser
}

// Engine.IO ID
func (c *serverConn) ID() string {
	return c.id
}

func newServerConn(_eio eio.Socket, creator parser.Creator) (*serverConn, *eio.Callbacks) {
	c := &serverConn{
		id:  _eio.ID(),
		eio: _eio,

		sockets: newServerSocketStore(),

		parser: creator(),
	}

	callbacks := &eio.Callbacks{
		OnPacket: c.onEIOPacket,
		OnError:  c.onError,
		OnClose:  c.onClose,
	}

	return c, callbacks
}

func (c *serverConn) onEIOPacket(packets ...*eioparser.Packet) {
	c.parserMu.Lock()
	defer c.parserMu.Unlock()

	for _, packet := range packets {
		if packet.Type == eioparser.PacketTypeMessage {
			err := c.parser.Add(packet.Data, c.onFinishEIOPacket)
			if err != nil {
				c.onError(err)
				return
			}
		}
	}
}

func (c *serverConn) onFinishEIOPacket(header *parser.PacketHeader, eventName string, decode parser.Decode) {
	if header.Namespace == "" {
		header.Namespace = "/"
	}

	if header.Type == parser.PacketTypeConnect {
		c.connect(header, decode)
	} else {
		socket, ok := c.sockets.Get(header.Namespace)
		if !ok {
			return
		}
		socket.onPacket(header, eventName, decode)
	}
}

func (c *serverConn) connect(header *parser.PacketHeader, decode parser.Decode) {

}

func (c *serverConn) onError(err error) {

}

func (c *serverConn) onClose(reason string, err error) {

}
