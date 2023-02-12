//go:build !amd64 || (amd64 && !(linux || windows || darwin))

package fast

import (
	jsonparser "github.com/tomruk/socket.io-go/parser/json"
	gojson "github.com/tomruk/socket.io-go/parser/json/go-json"
)

func New() jsonparser.JSONSerializer {
	defaultConfig := DefaultConfig()
	return gojson.New(defaultConfig.GoJSON.EncodeOptions, defaultConfig.GoJSON.DecodeOptions)
}

func NewWithConfig(config Config) jsonparser.JSONSerializer {
	return gojson.New(config.GoJSON.EncodeOptions, config.GoJSON.DecodeOptions)
}

func Type() SerializerType {
	return SerializerTypeGoJSON
}
