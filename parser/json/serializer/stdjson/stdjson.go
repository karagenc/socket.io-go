package stdjson

import (
	"io"

	"encoding/json"

	"github.com/tomruk/socket.io-go/parser/json/serializer"
)

type stdjsonSerializer struct{}

func (s stdjsonSerializer) Marshal(v any) ([]byte, error) {
	return json.Marshal(v)
}

func (s stdjsonSerializer) Unmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

func (s stdjsonSerializer) NewEncoder(w io.Writer) serializer.JSONEncoder {
	return json.NewEncoder(w)
}

func (s stdjsonSerializer) NewDecoder(r io.Reader) serializer.JSONDecoder {
	return json.NewDecoder(r)
}

func New() serializer.JSONSerializer {
	return &stdjsonSerializer{}
}
