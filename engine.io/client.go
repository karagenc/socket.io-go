package eio

import (
	"net/http"
	"net/url"
	"time"

	"github.com/tomruk/socket.io-go/engine.io/transport"
	"nhooyr.io/websocket"
)

type ClientConfig struct {
	// Valid transports are: polling, websocket.
	//
	// Default value is: ["polling", "websocket"]
	Transports []string

	// Timeout for transport upgrade.
	// If this timeout exceeds before an upgrade takes place, Dial will return an error.
	UpgradeTimeout time.Duration

	// Additional callback to get notified about the transport upgrade.
	UpgradeDone func(transportName string)

	// This is a special data type to concurrently
	// store the additional HTTP request headers to use.
	// Values can be retrieved and changed any time with Get, Set, Del methods.
	// Create this with transport.NewRequestHeader function.
	RequestHeader *transport.RequestHeader

	// Custom HTTP transport to use.
	//
	// If this is a http.Transport it will be cloned and timeout(s) will be set later on.
	// If not, it is the user's responsibility to set a proper timeout so when polling takes too long, we don't fail.
	HTTPTransport http.RoundTripper

	// Custom WebSocket dialer to use.
	WebSocketDialOptions *websocket.DialOptions

	// For debugging purposes. Leave it nil if it is of no use.
	Debugger Debugger
}

func Dial(rawURL string, callbacks *Callbacks, config *ClientConfig) (ClientSocket, error) {
	if callbacks == nil {
		callbacks = new(Callbacks)
	}
	callbacks.setMissing()

	if config == nil {
		config = new(ClientConfig)
	}

	socket := &clientSocket{
		httpClient: newHTTPClient(config.HTTPTransport),

		upgradeTimeout: defaultUpgradeTimeout,
		upgradeDone:    config.UpgradeDone,

		callbacks: *callbacks,

		pingChan:  make(chan struct{}, 1),
		closeChan: make(chan struct{}),
	}

	if config.RequestHeader != nil {
		socket.requestHeader = config.RequestHeader
	} else {
		socket.requestHeader = transport.NewRequestHeader(nil)
	}

	var transports []string
	if len(config.Transports) > 0 {
		transports = config.Transports
	} else {
		transports = []string{"polling", "websocket"}
	}

	if config.UpgradeTimeout != 0 {
		socket.upgradeTimeout = config.UpgradeTimeout
	}

	if socket.upgradeDone == nil {
		socket.upgradeDone = func(transportName string) {}
	}

	if config.WebSocketDialOptions != nil {
		socket.wsDialOptions = config.WebSocketDialOptions
	}

	if config.Debugger != nil {
		socket.debug = config.Debugger
	} else {
		socket.debug = NewNoopDebugger()
	}
	socket.debug = socket.debug.WithDynamicContext("[eio/client] Socket with ID", func() string {
		id := socket.ID()
		if id == "" {
			return "<none>"
		}
		return id
	})

	var err error
	socket.url, err = parseURL(rawURL)
	if err != nil {
		return nil, err
	}

	err = socket.connect(transports)
	if err != nil {
		return nil, err
	}

	return socket, nil
}

func parseURL(rawURL string) (*url.URL, error) {
	url, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}

	if len(url.Path) > 0 && url.Path[len(url.Path)-1] != '/' {
		url.Path += "/"
	}

	switch url.Scheme {
	case "wss":
		url.Scheme = "https"
	case "ws":
		url.Scheme = "http"
	}

	return url, nil
}

func newHTTPClient(t http.RoundTripper) *http.Client {
	// Clone the transport, so that we don't change the default timeouts later on. See: polling/client.go
	// If we're unable to clone the transport, leave it as it is.
	if t == nil {
		ht, ok := http.DefaultTransport.(*http.Transport)
		if ok {
			t = ht.Clone()
		} else {
			t = http.DefaultTransport
		}
	} else {
		ht, ok := t.(*http.Transport)
		if ok {
			t = ht.Clone()
		}
	}

	return &http.Client{
		Transport: t,
		Timeout:   0,
	}
}
