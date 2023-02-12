//go:build amd64 && (linux || windows || darwin)

package sonic

import (
	"io"

	"github.com/bytedance/sonic"
	jsonparser "github.com/tomruk/socket.io-go/parser/json"
)

type Config = sonic.Config

type serializer struct {
	api sonic.API
}

func (s *serializer) Marshal(v any) ([]byte, error) {
	return s.api.Marshal(v)
}

func (s *serializer) Unmarshal(data []byte, v any) error {
	return s.api.Unmarshal(data, v)
}

func (s *serializer) NewEncoder(w io.Writer) jsonparser.JSONEncoder {
	return s.api.NewEncoder(w)
}

func (s *serializer) NewDecoder(r io.Reader) jsonparser.JSONDecoder {
	return s.api.NewDecoder(r)
}

func New(config sonic.Config) jsonparser.JSONSerializer {
	return &serializer{api: config.Froze()}
}
