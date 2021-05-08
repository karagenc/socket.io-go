package sio

import (
	eio "github.com/tomruk/socket.io-go/engine.io"
	"github.com/tomruk/socket.io-go/parser"
	jsonparser "github.com/tomruk/socket.io-go/parser/json"
)

type ClientConfig struct {
	ParserCreator parser.Creator

	EIO eio.ClientConfig
}

type Client struct {
	url           string
	parserCreator parser.Creator
}

func NewClient(url string, config *ClientConfig) (*Client, error) {
	if config == nil {
		config = new(ClientConfig)
	}

	client := &Client{
		url:           url,
		parserCreator: config.ParserCreator,
	}

	if client.parserCreator == nil {
		client.parserCreator = jsonparser.NewCreator(0)
	}

	return client, nil
}
