package gojson

import "github.com/goccy/go-json"

type decoder struct {
	d       *json.Decoder
	options []json.DecodeOptionFunc
}

func (d decoder) Decode(v any) error {
	return d.d.DecodeWithOption(v, d.options...)
}
