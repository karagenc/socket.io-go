package gojson

import (
	"io"

	"github.com/goccy/go-json"
	"github.com/tomruk/socket.io-go/parser/json/serializer"
)

type gojsonSerializer struct {
	encodeOptions []json.EncodeOptionFunc
	decodeOptions []json.DecodeOptionFunc
}

func (s *gojsonSerializer) Marshal(v any) ([]byte, error) {
	return json.MarshalWithOption(v, s.encodeOptions...)
}

func (s *gojsonSerializer) Unmarshal(data []byte, v any) error {
	return json.UnmarshalWithOption(data, v, s.decodeOptions...)
}

func (s *gojsonSerializer) NewEncoder(w io.Writer) serializer.JSONEncoder {
	e := json.NewEncoder(w)
	return encoder{e: e, options: s.encodeOptions}
}

func (s *gojsonSerializer) NewDecoder(r io.Reader) serializer.JSONDecoder {
	d := json.NewDecoder(r)
	return decoder{d: d, options: s.decodeOptions}
}

func New(encodeOptions []json.EncodeOptionFunc, decodeOptions []json.DecodeOptionFunc) serializer.JSONSerializer {
	return &gojsonSerializer{
		encodeOptions: encodeOptions,
		decodeOptions: decodeOptions,
	}
}
