package jsonparser

import "io"

type JSONAPI interface {
	JSONMarshalUnmarshaler
	JSONEncodeDecoder
}

type JSONMarshalUnmarshaler interface {
	Marshal(v any) ([]byte, error)
	Unmarshal(data []byte, v any) error
}

type JSONEncodeDecoder interface {
	NewEncoder(w io.Writer) JSONEncoder
	NewDecoder(r io.Reader) JSONDecoder
}

type JSONEncoder interface {
	Encode(v any) error
}

type JSONDecoder interface {
	Decode(v any) error
}
