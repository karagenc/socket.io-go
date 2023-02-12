package gojson

import (
	"io"

	"github.com/goccy/go-json"
	jsonparser "github.com/tomruk/socket.io-go/parser/json"
)

type goJSONAPI struct {
	encodeOptions []json.EncodeOptionFunc
	decodeOptions []json.DecodeOptionFunc
}

func (j *goJSONAPI) Marshal(v any) ([]byte, error) {
	return json.MarshalWithOption(v, j.encodeOptions...)
}

func (j *goJSONAPI) Unmarshal(data []byte, v any) error {
	return json.UnmarshalWithOption(data, v, j.decodeOptions...)
}

type goJSONEncoder struct {
	e             *json.Encoder
	encodeOptions []json.EncodeOptionFunc
}

func (e *goJSONEncoder) Encode(v any) error {
	return e.e.EncodeWithOption(v, e.encodeOptions...)
}

func (j *goJSONAPI) NewEncoder(w io.Writer) jsonparser.JSONEncoder {
	e := json.NewEncoder(w)
	return &goJSONEncoder{e: e, encodeOptions: j.encodeOptions}
}

type goJSONDecoder struct {
	d             *json.Decoder
	decodeOptions []json.DecodeOptionFunc
}

func (d *goJSONDecoder) Decode(v any) error {
	return d.d.DecodeWithOption(v, d.decodeOptions...)
}

func (j *goJSONAPI) NewDecoder(r io.Reader) jsonparser.JSONDecoder {
	d := json.NewDecoder(r)
	return &goJSONDecoder{d: d, decodeOptions: j.decodeOptions}
}

func NewGoJSONAPI(encodeOptions []json.EncodeOptionFunc, decodeOptions []json.DecodeOptionFunc) jsonparser.JSONAPI {
	return &goJSONAPI{
		encodeOptions: encodeOptions,
		decodeOptions: decodeOptions,
	}
}
