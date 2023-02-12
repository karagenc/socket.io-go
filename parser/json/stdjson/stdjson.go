package stdjson

import (
	"io"

	"encoding/json"

	jsonparser "github.com/tomruk/socket.io-go/parser/json"
)

type stdJSONAPI struct{}

func (j *stdJSONAPI) Marshal(v any) ([]byte, error) {
	return json.Marshal(v)
}

func (j *stdJSONAPI) Unmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

func (j *stdJSONAPI) NewEncoder(w io.Writer) jsonparser.JSONEncoder {
	return json.NewEncoder(w)
}

func (j *stdJSONAPI) NewDecoder(r io.Reader) jsonparser.JSONDecoder {
	return json.NewDecoder(r)
}

func NewStdJSONAPI() jsonparser.JSONAPI {
	return &stdJSONAPI{}
}
