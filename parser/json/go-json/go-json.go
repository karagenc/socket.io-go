package gojson

import (
	"io"

	"github.com/goccy/go-json"
	jsonparser "github.com/tomruk/socket.io-go/parser/json"
)

type serializer struct {
	encodeOptions []json.EncodeOptionFunc
	decodeOptions []json.DecodeOptionFunc
}

func (s *serializer) Marshal(v any) ([]byte, error) {
	return json.MarshalWithOption(v, s.encodeOptions...)
}

func (s *serializer) Unmarshal(data []byte, v any) error {
	return json.UnmarshalWithOption(data, v, s.decodeOptions...)
}

func (s *serializer) NewEncoder(w io.Writer) jsonparser.JSONEncoder {
	e := json.NewEncoder(w)
	return &encoder{e: e, options: s.encodeOptions}
}

func (s *serializer) NewDecoder(r io.Reader) jsonparser.JSONDecoder {
	d := json.NewDecoder(r)
	return &decoder{d: d, options: s.decodeOptions}
}

func New(encodeOptions []json.EncodeOptionFunc, decodeOptions []json.DecodeOptionFunc) jsonparser.JSONSerializer {
	return &serializer{
		encodeOptions: encodeOptions,
		decodeOptions: decodeOptions,
	}
}
