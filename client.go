package sio

import (
	"fmt"
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
	// This value is required to be between 0 and 1.
	// Default: 0.5
	RandomizationFactor *float32

	// Authentication data to use with sockets. This value must be a struct or a map.
	// Socket.IO doesn't accept non-JSON-object value for authentication data.
	//
	// When a new socket is created with 'Socket' method,
	// this variable will be set initially.
	// Unless you provide an authData with the 'Connect' method of Socket, this variable will be used.
	AuthData interface{}
}

type Client struct {
	url       string
	eioConfig eio.ClientConfig

	// This mutex is used for protecting parser from concurrent calls.
	// Due to the modular and concurrent nature of Engine.IO,
	// we should use a mutex to ensure the Engine.IO doesn't access
	// the parser's Add method from multiple goroutines.
	parserMu sync.Mutex
	parser   parser.Parser

	preventAutoConnect   bool
	noReconnection       bool
	reconnectionAttempts int
	reconnectionDelay    time.Duration
	reconnectionDelayMax time.Duration
	randomizationFactor  float32

	authData interface{}

	sockets *clientSocketStore

	backoff *backoff

	eio   eio.Socket
	eioMu sync.RWMutex
}

const (
	DefaultReconnectionDelay            = 1 * time.Second
	DefaultReconnectionDelayMax         = 5 * time.Second
	DefaultRandomizationFactor  float32 = 0.5
)

// Create a new client and add it to the client cache.
// You can later use LookupClient function with the same URL to retrieve the created client from client cache.
func NewClient(url string, config *ClientConfig) *Client {
	if config == nil {
		config = new(ClientConfig)
	}

	client := &Client{
		url:       url,
		eioConfig: config.EIO,

		preventAutoConnect:   config.PreventAutoConnect,
		noReconnection:       config.NoReconnection,
		reconnectionAttempts: config.ReconnectionAttempts,

		authData: config.AuthData,

		sockets: newClientSocketStore(),
	}

	if config.ReconnectionDelay != nil {
		client.reconnectionDelay = *config.ReconnectionDelay
	} else {
		client.reconnectionDelay = DefaultReconnectionDelay
	}

	if config.ReconnectionDelayMax != nil {
		client.reconnectionDelayMax = *config.ReconnectionDelayMax
	} else {
		client.reconnectionDelayMax = DefaultReconnectionDelayMax
	}

	if config.RandomizationFactor != nil {
		client.randomizationFactor = *config.RandomizationFactor
	} else {
		client.randomizationFactor = DefaultRandomizationFactor
	}

	client.backoff = newBackoff(client.reconnectionDelay, client.reconnectionDelayMax, client.randomizationFactor)

	parserCreator := config.ParserCreator
	if parserCreator == nil {
		parserCreator = jsonparser.NewCreator(0)
	}
	client.parser = parserCreator()

	// Create the default socket
	client.Socket("/")

	if !client.preventAutoConnect {
		go func() {
			err := client.connect()
			if err != nil && client.noReconnection == false {
				go client.reconnect()
			}
		}()
	}

	ccache.Add(client)

	return client
}

// Look up a client from client cache. If the client is not found or not created with NewClient, ok will be false.
func LookupClient(url string) (client *Client, ok bool) {
	return ccache.Get(url)
}

// Remove a client from client cache.
func RemoveClient(url string) {
	ccache.Remove(url)
}

func (c *Client) Socket(namespace string) Socket {
	if namespace == "" {
		namespace = "/"
	}

	socket, ok := c.sockets.Get(namespace)
	if !ok {
		socket = newClientSocket(c, namespace, c.parser, c.authData)
		c.sockets.Add(socket)
	}
	return socket
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
		return err
	}
	c.eio = eio

	sockets := c.sockets.GetAll()
	for _, socket := range sockets {
		go socket.onEIOConnect()
	}

	return
}

func (c *Client) onEIOPacket(packet *eioparser.Packet) {
	c.parserMu.Lock()
	defer c.parserMu.Unlock()
	err := c.parser.Add(packet.Data, c.onFinishPacket)
	if err != nil {
		go c.onError(err)
		return
	}
}

func (c *Client) onFinishPacket(header *parser.PacketHeader, eventName string, decode parser.Decode) {
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
		// TODO: Reconnect failed
		return
	}

	backoffDuration := c.backoff.Duration()
	time.Sleep(backoffDuration)

	err := c.connect()
	if err != nil {
		c.reconnect()
		return
	}

	c.backoff.Reset()
}

func (c *Client) onEIOError(err error) {
	// TODO: Handle error
	fmt.Printf("eio onError: %s\n", err)
}

func (c *Client) onEIOClose(reason string, err error) {
	// TODO: Handle close
	fmt.Printf("eio onClose: reason: %s, err: %s\n", reason, err)

	go c.onClose(reason, err)

	if err != nil && c.noReconnection == false {
		go c.reconnect()
	}
}

func (c *Client) onError(err error) {
	// TODO: Handle error
	fmt.Printf("sio onError: %s\n", err)
}

func (c *Client) onClose(reason string, err error) {
	// TODO: Handle close
	fmt.Printf("sio onClose: reason: %s, err: %s\n", reason, err)
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

func (s *clientSocketStore) Add(ss *clientSocket) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sockets[ss.namespace] = ss
}

func (s *clientSocketStore) Delete(namespace string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sockets, namespace)
}

var ccache = newClientCache()

type clientCache struct {
	clients map[string]*Client
	mu      sync.Mutex
}

func newClientCache() *clientCache {
	return &clientCache{
		clients: make(map[string]*Client),
	}
}

func (c *clientCache) Get(url string) (cl *Client, ok bool) {
	c.mu.Lock()
	url = stripBackslash(url)
	cl, ok = c.clients[url]
	c.mu.Unlock()
	return
}

func (c *clientCache) Add(cl *Client) {
	c.mu.Lock()
	url := stripBackslash(cl.url)
	c.clients[url] = cl
	c.mu.Unlock()
}

func (c *clientCache) Remove(url string) {
	c.mu.Lock()
	url = stripBackslash(url)
	delete(c.clients, url)
	c.mu.Unlock()
}

func stripBackslash(url string) string {
	if url != "" && url[len(url)-1] == '/' {
		url = url[:len(url)-1]
	}
	return url
}
