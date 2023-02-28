package sio

import (
	"fmt"
	"math"
	"reflect"
	"sync"
	"time"

	eio "github.com/tomruk/socket.io-go/engine.io"
	eioparser "github.com/tomruk/socket.io-go/engine.io/parser"
	"github.com/tomruk/socket.io-go/parser"
	jsonparser "github.com/tomruk/socket.io-go/parser/json"
	"github.com/tomruk/socket.io-go/parser/json/serializer/stdjson"
)

type clientConnectionState int

const (
	clientConnStateConnecting clientConnectionState = iota
	clientConnStateConnected
	clientConnStateReconnecting
	clientConnStateDisconnected
)

type ClientConfig struct {
	// A creator function for the Socket.IO parser.
	// This function is used for creating a parser.Parser object.
	// You can use a custom parser by changing this variable.
	//
	// By default this function is nil and default JSON parser is used.
	ParserCreator parser.Creator

	// Configuration for the Engine.IO.
	EIO eio.ClientConfig

	// The NewClient function automatically connects to the server.
	//
	// If you enable this, no connection will be made, and you will
	// need to use the Connect method to make connection.
	NoAutoConnection bool

	// Should we disallow reconnections?
	// Default: false (allow reconnections)
	NoReconnection bool

	// How many reconnection attempts should we try?
	// Default: 0 (Infinite)
	ReconnectionAttempts uint32

	// The time delay between reconnection attempts.
	// Default: 1 second
	ReconnectionDelay *time.Duration

	// The max time delay between reconnection attempts.
	// Default: 5 seconds
	ReconnectionDelayMax *time.Duration

	// Used in the exponential backoff jitter when reconnecting.
	// This value is required to be between 0 and 1
	//
	// Default: 0.5
	RandomizationFactor *float32
}

type Client struct {
	url       string
	eioConfig eio.ClientConfig

	// This mutex is used for protecting parser from concurrent calls.
	// Due to the modular and concurrent nature of Engine.IO,
	// we should use a mutex to ensure that the Engine.IO doesn't access
	// the parser's Add method from multiple goroutines.
	parserMu sync.Mutex
	parser   parser.Parser

	noAutoConnection     bool
	noReconnection       bool
	reconnectionAttempts uint32
	reconnectionDelay    time.Duration
	reconnectionDelayMax time.Duration
	randomizationFactor  float32

	sockets *clientSocketStore
	emitter *eventEmitter
	backoff *backoff

	eio            eio.ClientSocket
	eioPacketQueue *packetQueue
	eioMu          sync.RWMutex

	connState   clientConnectionState
	connStateMu sync.RWMutex
}

const (
	DefaultReconnectionDelay            = 1 * time.Second
	DefaultReconnectionDelayMax         = 5 * time.Second
	DefaultRandomizationFactor  float32 = 0.5
)

// This function creates a new Client.
//
// You should create a new Socket using the Socket
// method of the Client returned by this function.
// If you don't do that, server will terminate your connection. See: https://socket.io/docs/v4/server-initialization/#connectTimeout
//
// The Client is called "Manager" in official implementation of Socket.IO: https://github.com/socketio/socket.io-client/blob/4.1.3/lib/manager.ts#L295
func NewClient(url string, config *ClientConfig) *Client {
	if config == nil {
		config = new(ClientConfig)
	} else {
		c := *config
		config = &c
	}

	io := &Client{
		url:       url,
		eioConfig: config.EIO,

		noAutoConnection:     config.NoAutoConnection,
		noReconnection:       config.NoReconnection,
		reconnectionAttempts: config.ReconnectionAttempts,

		sockets: newClientSocketStore(),
		emitter: newEventEmitter(),

		eioPacketQueue: newPacketQueue(),
	}

	if config.ReconnectionDelay != nil {
		io.reconnectionDelay = *config.ReconnectionDelay
	} else {
		io.reconnectionDelay = DefaultReconnectionDelay
	}

	if config.ReconnectionDelayMax != nil {
		io.reconnectionDelayMax = *config.ReconnectionDelayMax
	} else {
		io.reconnectionDelayMax = DefaultReconnectionDelayMax
	}

	if config.RandomizationFactor != nil {
		io.randomizationFactor = *config.RandomizationFactor
	} else {
		io.randomizationFactor = DefaultRandomizationFactor
	}

	io.backoff = newBackoff(io.reconnectionDelay, io.reconnectionDelayMax, io.randomizationFactor)

	parserCreator := config.ParserCreator
	if parserCreator == nil {
		json := stdjson.New()
		parserCreator = jsonparser.NewCreator(0, json)
	}
	io.parser = parserCreator()

	if !io.noAutoConnection {
		go func() {
			err := io.connect()
			if err != nil && io.noReconnection == false {
				go io.reconnect()
				// TODO: Which one should handle this error, Client or clientSocket?
				io.onError(err)
			}
		}()
	}

	return io
}

func (c *Client) Socket(namespace string) ClientSocket {
	if namespace == "" {
		namespace = "/"
	}

	socket, ok := c.sockets.Get(namespace)
	if !ok {
		socket = newClientSocket(c, namespace, c.parser)
		c.sockets.Set(socket)
	}

	if !c.noAutoConnection {
		c.eioMu.RLock()
		defer c.eioMu.RUnlock()
		connected := c.eio != nil

		if !connected {
			socket.sendConnectPacket()
		}
	}
	return socket
}

func (c *Client) On(eventName string, handler interface{}) {
	c.checkHandler(eventName, handler)
	c.emitter.On(eventName, handler)
}

func (c *Client) Once(eventName string, handler interface{}) {
	c.checkHandler(eventName, handler)
	c.emitter.On(eventName, handler)
}

func (c *Client) checkHandler(eventName string, handler interface{}) {
	switch eventName {
	case "":
		fallthrough
	case "open":
		fallthrough
	case "error":
		fallthrough
	case "ping":
		fallthrough
	case "close":
		fallthrough
	case "reconnect":
		fallthrough
	case "reconnect_attempt":
		fallthrough
	case "reconnect_error":
		fallthrough
	case "reconnect_failed":
		err := checkHandler(eventName, handler)
		if err != nil {
			panic(fmt.Errorf("sio: %w", err))
		}
	}
}

func (c *Client) Off(eventName string, handler interface{}) {
	c.emitter.Off(eventName, handler)
}

func (c *Client) OffAll() {
	c.emitter.OffAll()
}

func (c *Client) connect() (err error) {
	c.eioMu.Lock()
	defer c.eioMu.Unlock()

	callbacks := &eio.Callbacks{
		OnPacket: c.onEIOPacket,
		OnError:  c.onEIOError,
		OnClose:  c.onEIOClose,
	}

	_eio, err := eio.Dial(c.url, callbacks, &c.eioConfig)
	if err != nil {
		c.emitReserved("error", err)
		return err
	}
	c.eio = _eio
	go pollAndSend(c.eio, c.eioPacketQueue)

	sockets := c.sockets.GetAll()
	for _, socket := range sockets {
		go socket.sendConnectPacket()
	}

	c.emitReserved("open")
	return
}

func (c *Client) onEIOPacket(packets ...*eioparser.Packet) {
	c.parserMu.Lock()
	defer c.parserMu.Unlock()

	for _, packet := range packets {
		switch packet.Type {
		case eioparser.PacketTypeMessage:
			err := c.parser.Add(packet.Data, c.onFinishEIOPacket)
			if err != nil {
				c.onError(wrapInternalError(err))
				return
			}

		case eioparser.PacketTypePing:
			c.emitReserved("ping")
		}
	}
}

func (c *Client) onFinishEIOPacket(header *parser.PacketHeader, eventName string, decode parser.Decode) {
	if header.Namespace == "" {
		header.Namespace = "/"
	}

	socket, ok := c.sockets.Get(header.Namespace)
	if !ok {
		return
	}
	socket.onPacket(header, eventName, decode)
}

func (c *Client) reconnect() {
	attempts := c.backoff.Attempts()
	didAttemptsReachedMaxAttempts := c.reconnectionAttempts > 0 && attempts >= c.reconnectionAttempts
	// Just in case
	didAttemptsReachedMaxInt := c.reconnectionAttempts == 0 && attempts == math.MaxUint32

	if didAttemptsReachedMaxAttempts || didAttemptsReachedMaxInt {
		c.backoff.Reset()
		c.emitReserved("reconnect_failed")
		return
	}

	backoffDuration := c.backoff.Duration()
	time.Sleep(backoffDuration)

	attempts = c.backoff.Attempts()
	c.emitReserved("reconnect_attempt", attempts)

	err := c.connect()
	if err != nil {
		c.emitReserved("reconnect", err)
		c.reconnect()
		return
	}

	attempts = c.backoff.Attempts()
	c.backoff.Reset()

	c.emitReserved("reconnect", attempts)
}

func (c *Client) onEIOError(err error) {
	c.onError(err)
}

func (c *Client) onEIOClose(reason string, err error) {
	go c.onClose(reason, err)

	if err != nil && c.noReconnection == false {
		go c.reconnect()
		// TODO: Which one should handle this error, Client or clientSocket?
		c.onError(err)
	}
}

// Convenience method for emitting events to the user.
func (s *Client) emitReserved(eventName string, v ...interface{}) {
	handlers := s.emitter.GetHandlers(eventName)
	values := make([]reflect.Value, len(v))
	for i := range values {
		values[i] = reflect.ValueOf(v)
	}

	go func() {
		for _, handler := range handlers {
			_, err := handler.Call(values...)
			if err != nil {
				s.onError(wrapInternalError(fmt.Errorf("emitReserved: %s", err)))
				return
			}
		}
	}()
}

func (c *Client) onError(err error) {
	// emitReserved is not used because if an error would happen in handler.Call
	// onError would be called recursively.

	errValue := reflect.ValueOf(err)

	handlers := c.emitter.GetHandlers("error")
	go func() {
		for _, handler := range handlers {
			_, err := handler.Call(errValue)
			if err != nil {
				// This should panic.
				// If you cannot handle the error via `onError`
				// then what option do you have?
				panic(fmt.Errorf("sio: %w", err))
			}
		}
	}()
}

func (c *Client) destroy(socket *clientSocket) {
	// TODO: Implement this.
}

func (c *Client) onClose(reason string, err error) {
	c.parserMu.Lock()
	defer c.parserMu.Unlock()
	c.parser.Reset()
	c.backoff.Reset()
	c.emitReserved("close", reason, err)

	if !c.noReconnection {
		c.reconnect()
	}
}

func (c *Client) close() {
	c.connStateMu.Lock()
	defer c.connStateMu.Unlock()
	c.connState = clientConnStateDisconnected
	c.onClose("forced close", nil)
	c.eioMu.Lock()
	defer c.eioMu.Unlock()
	c.eio.Close()
	c.eioPacketQueue.Reset()
}

func (c *Client) Close() {
	c.close()
}

func (c *Client) packet(packets ...*eioparser.Packet) {
	c.eioMu.RLock()
	defer c.eioMu.RUnlock()
	c.eioPacketQueue.Add(packets...)
}
