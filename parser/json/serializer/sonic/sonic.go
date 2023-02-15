//go:build amd64 && (linux || windows || darwin)

package sonic

import (
	"io"

	"github.com/bytedance/sonic"
	"github.com/tomruk/socket.io-go/parser/json/serializer"
)

type Config = sonic.Config

type sonicSerializer struct {
	api sonic.API
}

func (s *sonicSerializer) Marshal(v any) ([]byte, error) {
	return s.api.Marshal(v)
}

func (s *sonicSerializer) Unmarshal(data []byte, v any) error {
	return s.api.Unmarshal(data, v)
}

func (s *sonicSerializer) NewEncoder(w io.Writer) serializer.JSONEncoder {
	return s.api.NewEncoder(w)
}

func (s *sonicSerializer) NewDecoder(r io.Reader) serializer.JSONDecoder {
	return s.api.NewDecoder(r)
}

func New(config sonic.Config) serializer.JSONSerializer {
	return &sonicSerializer{api: config.Froze()}
}
