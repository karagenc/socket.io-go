package eio

import (
	"net/http"

	"github.com/tomruk/socket.io-go/engine.io/parser"
)

type ServerTransport interface {
	// Name of the transport in lowercase.
	Name() string

	// For the caller:
	//
	// Do not call this method with a nil argument. Always provide non-nil arguments.
	//
	// For the transport developer:
	//
	// This method can be called at any time concurrently.
	// Make sure you either use a locking mechanism (Mutex or RWMutex) or use the atomic package to access the callbacks.
	//
	// Initially, this method will be called right after the transport is created. This is to ensure that the callbacks are not nil.
	SetCallbacks(onPacket func(packet *parser.Packet), onClose func(transportName string, err error))

	// handshakePacket can be nil. Do a nil check.
	// onPacket callback must not be called in this method.
	Handshake(handshakePacket *parser.Packet, w http.ResponseWriter, r *http.Request) error

	// This method is for handling an open connection (such as websocket.Conn) without closing the handshake request.
	// Currently this is only used by the websocket transport.
	PostHandshake()

	// If the transport supports handling HTTP requests (after the handshake is completely done) make use of this method.
	// Otherwise, just reply with 400 (Bad request).
	ServeHTTP(w http.ResponseWriter, r *http.Request)

	// Return the packets that are waiting on the pollQueue (polling only).
	QueuedPackets() []*parser.Packet

	// If you run this method in a transport (see the close method of polling for example), call it on a new goroutine.
	// Otherwise it can call the close function recursively.
	SendPacket(p *parser.Packet)

	// This method closes the transport but doesn't call the onClose callback.
	// This method will be called after an upgrade to discard and remove this transport.
	//
	// You must make sure that this method doesn't block or recursively call itself.
	Discard()

	// This method closes the transport and calls the onClose callback.
	//
	// You must make sure that this method doesn't block or recursively call itself.
	Close()
}

type ClientTransport interface {
	// Name of the transport in lowercase.
	Name() string

	// For the caller:
	//
	// Do not call this method with a nil argument. Always provide non-nil arguments.
	//
	// For the transport developer:
	//
	// This method can be called at any time concurrently.
	// Make sure you either use a locking mechanism (Mutex or RWMutex) or use the atomic package to access the callbacks.
	//
	// Initially, this method will be called right after the transport is created. This is to ensure that the callbacks are not nil.
	SetCallbacks(onPacket func(packet *parser.Packet), onClose func(transportName string, err error))

	// This method is used for connecting to the server.
	//
	// You should receive the OPEN packet unless the transport is used for upgrade purposes.
	// If sid is set, you're upgrading to this transport. Expect an OPEN packet. (see websocket/client.go for example)
	//
	// onPacket callback must not be called in this method.
	Handshake() (hr *parser.HandshakeResponse, err error)

	// This method will be called right after the handshake is done and it will only called once, on a new goroutine.
	// Use this method to start the connection loop.
	Run()

	// If you run this method in a transport (see the close method of polling for example), call it on a new goroutine.
	// Otherwise it can call the close function recursively.
	SendPacket(p *parser.Packet)

	// This method closes the transport but doesn't call the onClose callback.
	// This method will be called after an upgrade to discard and remove this transport.
	//
	// You must make sure that this method doesn't block or recursively call itself.
	Discard()

	// This method closes the transport and calls the onClose callback.
	//
	// You must make sure that this method doesn't block or recursively call itself.
	Close()
}
