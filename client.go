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

	// Prevent the initial connection.
	PreventAutoConnect bool

	// Should we disallow reconnections?
	// Default: false (allow reconnections)
	NoReconnection bool

	// How many reconnection attempts should we try?
	// Default: 0 (Infinite)
	ReconnectionAttempts int

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

	preventAutoConnect   bool
	noReconnection       bool
	reconnectionAttempts int
	reconnectionDelay    time.Duration
	reconnectionDelayMax time.Duration
	randomizationFactor  float32

	sockets *clientSocketStore
	emitter *eventEmitter
	backoff *backoff

	eio   eio.Socket
	eioMu sync.RWMutex
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

		preventAutoConnect:   config.PreventAutoConnect,
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
		parserCreator = jsonparser.NewCreator(0)
	}
	io.parser = parserCreator()

	if !io.preventAutoConnect {
		go func() {
			err := io.connect()
			if err != nil && io.noReconnection == false {
				go io.reconnect()
			}
		}()
	}

	return io
}

func (c *Client) Socket(namespace string) Socket {
	if namespace == "" {
		namespace = "/"
	}

	socket, ok := c.sockets.Get(namespace)
	if !ok {
		socket = newClientSocket(c, namespace, c.parser)

		if !c.preventAutoConnect {
			c.eioMu.RLock()
			defer c.eioMu.RUnlock()
			connected := c.eio != nil

			if connected {
				socket.sendConnectPacket()
			}
		}

		c.sockets.Set(socket)
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
		checkHandler(eventName, handler)
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

	eio, err := eio.Dial(c.url, callbacks, &c.eioConfig)
	if err != nil {
		errValue := reflect.ValueOf(err)

		handlers := c.emitter.GetHandlers("error")
		for _, handler := range handlers {
			go func(handler *eventHandler) {
				handler.Call(errValue)
			}(handler)
		}

		return err
	}
	c.eio = eio

	sockets := c.sockets.GetAll()
	for _, socket := range sockets {
		go socket.sendConnectPacket()
	}

	handlers := c.emitter.GetHandlers("open")
	for _, handler := range handlers {
		go func(handler *eventHandler) {
			handler.Call()
		}(handler)
	}

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
				c.onError(err)
				return
			}

		case eioparser.PacketTypePing:
			handlers := c.emitter.GetHandlers("ping")
			for _, handler := range handlers {
				go func(handler *eventHandler) {
					handler.Call()
				}(handler)
			}
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

	fmt.Printf("reconnect attempt: %d\n", attempts)

	if c.reconnectionAttempts > 0 && attempts >= c.reconnectionAttempts {
		c.backoff.Reset()
		handlers := c.emitter.GetHandlers("reconnect_failed")
		for _, handler := range handlers {
			go func(handler *eventHandler) {
				handler.Call()
			}(handler)
		}
		return
	}

	backoffDuration := c.backoff.Duration()
	time.Sleep(backoffDuration)

	attemptsValue := reflect.ValueOf(c.backoff.Attempts())
	handlers := c.emitter.GetHandlers("reconnect_attempt")
	for _, handler := range handlers {
		go func(handler *eventHandler) {
			handler.Call(attemptsValue)
		}(handler)
	}

	err := c.connect()
	if err != nil {
		errValue := reflect.ValueOf(err)
		handlers := c.emitter.GetHandlers("reconnect_error")
		for _, handler := range handlers {
			go func(handler *eventHandler) {
				handler.Call(errValue)
			}(handler)
		}

		c.reconnect()
		return
	}

	attemptsValue = reflect.ValueOf(c.backoff.Attempts())
	c.backoff.Reset()

	handlers = c.emitter.GetHandlers("reconnect")
	for _, handler := range handlers {
		go func(handler *eventHandler) {
			handler.Call(attemptsValue)
		}(handler)
	}
}

func (c *Client) onEIOError(err error) {
	c.onError(fmt.Errorf("engine.io error: %w", err))
}

func (c *Client) onEIOClose(reason string, err error) {
	go c.onClose(reason, err)

	if err != nil && c.noReconnection == false {
		go c.reconnect()
	}
}

func (c *Client) onError(err error) {
	errValue := reflect.ValueOf(err)

	handlers := c.emitter.GetHandlers("error")
	for _, handler := range handlers {
		go func(handler *eventHandler) {
			handler.Call(errValue)
		}(handler)
	}
}

func (c *Client) onClose(reason string, err error) {
	reasonValue := reflect.ValueOf(reason)
	errValue := reflect.ValueOf(err)

	handlers := c.emitter.GetHandlers("close")
	for _, handler := range handlers {
		go func(handler *eventHandler) {
			handler.Call(reasonValue, errValue)
		}(handler)
	}

	// TODO: Reconnect.
}

func (c *Client) Close() {
	c.eioMu.Lock()
	defer c.eioMu.Unlock()
	c.eio.Close()

	c.backoff.Reset()
	c.sockets.CloseAll()

	c.parserMu.Lock()
	defer c.parserMu.Unlock()
	c.parser.Reset()
}

func (c *Client) packet(packets ...*eioparser.Packet) {
	go func() {
		c.eioMu.RLock()
		eio := c.eio
		c.eioMu.RUnlock()
		eio.Send(packets...)
	}()
}

type clientSocketStore struct {
	sockets map[string]*clientSocket
	mu      sync.Mutex
}

func newClientSocketStore() *clientSocketStore {
	return &clientSocketStore{
		sockets: make(map[string]*clientSocket),
	}
}

func (s *clientSocketStore) Get(namespace string) (ss *clientSocket, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ss, ok = s.sockets[namespace]
	return
}

func (s *clientSocketStore) GetAll() (sockets []*clientSocket) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sockets = make([]*clientSocket, len(s.sockets))
	i := 0
	for _, ss := range s.sockets {
		sockets[i] = ss
		i++
	}
	return
}

func (s *clientSocketStore) Set(ss *clientSocket) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sockets[ss.namespace] = ss
}

func (s *clientSocketStore) Remove(namespace string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sockets, namespace)
}

func (s *clientSocketStore) CloseAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, socket := range s.sockets {
		socket.Close()
	}
}
