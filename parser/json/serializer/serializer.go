package serializer

import "io"

type (
	JSONSerializer interface {
		JSONMarshalUnmarshaler
		JSONEncodeDecoder
	}

	JSONMarshalUnmarshaler interface {
		Marshal(v any) ([]byte, error)
		Unmarshal(data []byte, v any) error
	}

	JSONEncodeDecoder interface {
		NewEncoder(w io.Writer) JSONEncoder
		NewDecoder(r io.Reader) JSONDecoder
	}

	JSONEncoder interface {
		Encode(v any) error
	}

	JSONDecoder interface {
		Decode(v any) error
	}
)
