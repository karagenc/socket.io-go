package sio

import (
	"fmt"
	"reflect"
	"sync"
	"time"

	eio "github.com/tomruk/socket.io-go/engine.io"
	eioparser "github.com/tomruk/socket.io-go/engine.io/parser"
	"github.com/tomruk/socket.io-go/parser"
	jsonparser "github.com/tomruk/socket.io-go/parser/json"
	"github.com/tomruk/socket.io-go/parser/json/serializer/stdjson"
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

	noReconnection       bool
	reconnectionAttempts uint32
	reconnectionDelay    time.Duration
	reconnectionDelayMax time.Duration
	randomizationFactor  float32

	sockets *clientSocketStore
	emitter *eventEmitter
	backoff *backoff
	conn    *clientConn

	skipReconnect   bool
	skipReconnectMu sync.Mutex
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

		noReconnection:       config.NoReconnection,
		reconnectionAttempts: config.ReconnectionAttempts,

		sockets: newClientSocketStore(),
		emitter: newEventEmitter(),
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
	io.conn = newClientConn(io)
	return io
}

func (c *Client) Connect() {
	go func() {
		err := c.conn.Connect()
		if err != nil {
			c.conn.MaybeReconnectOnOpen()
		}
	}()
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

	socket.Connect()
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

func (c *Client) onEIOError(err error) {
	c.onError(err)
}

func (c *Client) onEIOClose(reason string, err error) {
	c.onClose(reason, err)
}

// Convenience method for emitting events to the user.
func (s *Client) emitReserved(eventName string, v ...interface{}) {
	// On original socket.io, events are also emitted to
	// subevents registered by the socket.
	//
	// https://github.com/socketio/socket.io-client/blob/89175d0481fc7633c12bb5b233dc3421f87860ef/lib/socket.ts#L287
	for _, socket := range s.sockets.GetAll() {
		socket.invokeSubEvents(eventName, v...)
	}

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
	for _, socket := range c.sockets.GetAll() {
		if socket.IsActive() {
			return
		}
	}
	c.Disconnect()
}

func (c *Client) onClose(reason string, err error) {
	c.parserMu.Lock()
	defer c.parserMu.Unlock()
	c.parser.Reset()
	c.backoff.Reset()
	c.emitReserved("close", reason, err)

	if !c.noReconnection {
		go c.conn.Reconnect(false)
	}
}

func (c *Client) Disconnect() {
	c.conn.Disconnect()
}
