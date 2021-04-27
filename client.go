package sio

import (
	"fmt"

	eio "github.com/tomruk/socket.io-go/engine.io"
)

type ClientConfig struct {
	EIO eio.ClientConfig
}

type Client struct{}

func NewClient(url string, config *ClientConfig) (*Client, error) {
	if config == nil {
		config = new(ClientConfig)
	}
	return nil, fmt.Errorf("not implemented yet")
}
