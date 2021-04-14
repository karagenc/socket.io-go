package eio

type NewSocketCallback func(socket Socket) *Callbacks

type MessageCallback func(data []byte, isBinary bool)

type ErrorCallback func(err error)

// err can be nil. Always do a nil check.
type CloseCallback func(reason string, err error)

type Callbacks struct {
	OnMessage MessageCallback
	OnError   ErrorCallback
	OnClose   CloseCallback
}

func (c *Callbacks) setMissing() {
	if c.OnMessage == nil {
		c.OnMessage = func(data []byte, isBinary bool) {}
	}
	if c.OnError == nil {
		c.OnError = func(err error) {}
	}
	if c.OnClose == nil {
		c.OnClose = func(reason string, err error) {}
	}
}
