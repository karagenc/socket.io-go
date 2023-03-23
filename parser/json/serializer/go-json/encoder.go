package gojson

import "github.com/goccy/go-json"

type encoder struct {
	e       *json.Encoder
	options []json.EncodeOptionFunc
}

func (e encoder) Encode(v any) error {
	return e.e.EncodeWithOption(v, e.options...)
}
