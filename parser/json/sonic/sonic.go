//go:build amd64 && (linux || windows || darwin)

package sonic

import (
	"io"

	"github.com/bytedance/sonic"
	jsonparser "github.com/tomruk/socket.io-go/parser/json"
)

type sonicAPI struct {
	api sonic.API
}

func (j *sonicAPI) Marshal(v any) ([]byte, error) {
	return j.api.Marshal(v)
}

func (j *sonicAPI) Unmarshal(data []byte, v any) error {
	return j.api.Unmarshal(data, v)
}

func (j *sonicAPI) NewEncoder(w io.Writer) jsonparser.JSONEncoder {
	return j.api.NewEncoder(w)
}

func (j *sonicAPI) NewDecoder(r io.Reader) jsonparser.JSONDecoder {
	return j.api.NewDecoder(r)
}

func NewSonicAPI(config sonic.Config) jsonparser.JSONAPI {
	return &sonicAPI{api: config.Froze()}
}
