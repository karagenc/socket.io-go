package stdjson

import (
	"io"

	"encoding/json"

	jsonparser "github.com/tomruk/socket.io-go/parser/json"
)

type serializer struct{}

func (s serializer) Marshal(v any) ([]byte, error) {
	return json.Marshal(v)
}

func (s serializer) Unmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

func (s serializer) NewEncoder(w io.Writer) jsonparser.JSONEncoder {
	return json.NewEncoder(w)
}

func (s serializer) NewDecoder(r io.Reader) jsonparser.JSONDecoder {
	return json.NewDecoder(r)
}

func New() jsonparser.JSONSerializer {
	return &serializer{}
}
