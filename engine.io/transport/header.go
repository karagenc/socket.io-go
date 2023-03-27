package transport

import (
	"net/http"

	"github.com/tomruk/socket.io-go/internal/sync"
)

// A concurrent HTTP request header.
type RequestHeader struct {
	mu     sync.Mutex
	header http.Header
}

func NewRequestHeader(header http.Header) *RequestHeader {
	return &RequestHeader{
		header: header.Clone(),
	}
}

func (r *RequestHeader) Header() http.Header {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.header.Clone()
}

func (r *RequestHeader) Set(key, value string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.header.Set(key, value)
}

func (r *RequestHeader) Get(key string) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.header.Get(key)
}

func (r *RequestHeader) Del(key string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.header.Del(key)
}
