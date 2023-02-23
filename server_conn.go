package sio

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sync"
	"time"

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

func (s *serverSocketStore) Get(sid string) (socket *serverSocket, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	socket, ok = s.sockets[sid]
	return
}

func (s *serverSocketStore) GetAll() (sockets []*serverSocket) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sockets = make([]*serverSocket, len(s.sockets))
	i := 0
	for _, socket := range s.sockets {
		sockets[i] = socket
		i++
	}
	return
}

func (s *serverSocketStore) GetAndRemoveAll() (sockets []*serverSocket) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sockets = make([]*serverSocket, len(s.sockets))
	i := 0
	for _, socket := range s.sockets {
		sockets[i] = socket
		i++
	}
	s.sockets = make(map[string]*serverSocket)
	return
}

func (s *serverSocketStore) Set(socket *serverSocket) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sockets[socket.ID()] = socket
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
	eio            eio.ServerSocket
	eioPacketQueue *packetQueue

	server  *Server
	sockets *serverSocketStore
	nsps    *namespaceStore

	// This mutex is used for protecting parser from concurrent calls.
	// Due to the modular and concurrent nature of Engine.IO,
	// we should use a mutex to ensure the Engine.IO doesn't access
	// the parser's Add method from multiple goroutines.
	parserMu sync.Mutex
	parser   parser.Parser
}

func newServerConn(server *Server, _eio eio.ServerSocket, creator parser.Creator) (*serverConn, *eio.Callbacks) {
	c := &serverConn{
		eio:            _eio,
		eioPacketQueue: newPacketQueue(),

		server:  server,
		sockets: newServerSocketStore(),
		nsps:    newNamespaceStore(),

		parser: creator(),
	}

	callbacks := &eio.Callbacks{
		OnPacket: c.onEIOPacket,
		OnError:  c.onError,
		OnClose:  c.onClose,
	}

	go pollAndSend(c.eio, c.eioPacketQueue)

	go func() {
		time.Sleep(server.connectTimeout)
		if c.nsps.Len() == 0 {
			c.Close()
		}
	}()

	return c, callbacks
}

func (c *serverConn) onEIOPacket(packets ...*eioparser.Packet) {
	c.parserMu.Lock()
	defer c.parserMu.Unlock()

	for _, packet := range packets {
		if packet.Type == eioparser.PacketTypeMessage {
			err := c.parser.Add(packet.Data, c.onFinishEIOPacket)
			if err != nil {
				c.onError(wrapInternalError(err))
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
		sockets := c.sockets.GetAll()
		for _, socket := range sockets {
			if socket.nsp.Name() == header.Namespace {
				err := socket.onPacket(header, eventName, decode)
				c.onError(err)
				break
			}
		}

		c.onError(wrapInternalError(fmt.Errorf("socket not found")))
	}
}

func (c *serverConn) connect(header *parser.PacketHeader, decode parser.Decode) {
	var auth json.RawMessage

	at := reflect.TypeOf(&auth)
	values, err := decode(at)
	if err != nil {
		c.onError(wrapInternalError(err))
		return
	}

	if len(values) == 1 {
		rmp, ok := values[0].Interface().(*json.RawMessage)
		if ok {
			auth = *rmp
		}
	}

	nsp := c.server.Of(header.Namespace)
	socket, err := nsp.add(c, auth)
	if err != nil {
		c.connectError(err, nsp.Name())
		return
	}

	c.sockets.Set(socket)
	c.nsps.Set(nsp)

	socket.onConnect()
}

func (c *serverConn) connectError(err error, nsp string) {
	e := &connectError{
		Message: err.Error(),
	}

	header := parser.PacketHeader{
		Type:      parser.PacketTypeConnectError,
		Namespace: nsp,
	}

	buffers, err := c.parser.Encode(&header, e)
	if err != nil {
		c.onError(wrapInternalError(err))
		return
	}

	c.sendBuffers(buffers...)
}

func (c *serverConn) sendBuffers(buffers ...[]byte) {
	if len(buffers) > 0 {
		packets := make([]*eioparser.Packet, len(buffers))
		buf := buffers[0]
		buffers = buffers[1:]

		var err error
		packets[0], err = eioparser.NewPacket(eioparser.PacketTypeMessage, false, buf)
		if err != nil {
			c.onError(wrapInternalError(err))
			return
		}

		for i, attachment := range buffers {
			packets[i+1], err = eioparser.NewPacket(eioparser.PacketTypeMessage, true, attachment)
			if err != nil {
				c.onError(wrapInternalError(err))
				return
			}
		}

		c.packet(packets...)
	}
}

func (c *serverConn) packet(packets ...*eioparser.Packet) {
	c.eioPacketQueue.Add(packets...)
}

func (c *serverConn) onError(err error) {
	sockets := c.sockets.GetAll()
	for _, socket := range sockets {
		socket.onError(err)
	}

	// TODO: Close engine.io connection or not?
	// We don't close the engine.io connection here
	// (as opposed to the original socket.io) because...
}

func (c *serverConn) onClose(reason string, err error) {
	sockets := c.sockets.GetAndRemoveAll()
	for _, socket := range sockets {
		socket.onClose(reason)
	}

	c.parserMu.Lock()
	defer c.parserMu.Unlock()
	c.parser.Reset()
}

func (c *serverConn) DisconnectAll() {
	for _, socket := range c.sockets.GetAndRemoveAll() {
		socket.Disconnect(false)
	}
}

func (c *serverConn) Close() {
	c.eio.Close()
	c.eioPacketQueue.Reset()
	c.onClose("forced server close", nil)
}
