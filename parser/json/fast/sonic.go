//go:build amd64 && (linux || windows || darwin)

package fast

import (
	jsonparser "github.com/tomruk/socket.io-go/parser/json"
	"github.com/tomruk/socket.io-go/parser/json/sonic"
)

func New() jsonparser.JSONSerializer {
	defaultConfig := DefaultConfig()
	return sonic.New(defaultConfig.SonicConfig)
}

func NewWithConfig(config Config) jsonparser.JSONSerializer {
	return sonic.New(config.SonicConfig)
}

func Type() SerializerType {
	return SerializerTypeSonic
}
