package eio

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"sync"
)

const (
	Base64IDSize   = 15
	Base64IDMaxTry = 10
)

var (
	ErrBase64IDMaxTryReached = fmt.Errorf("base64 ID generation failed: base64IDMaxTry reached")
	errBase64IDInvalidSize   = fmt.Errorf("base64 ID generation failed: invalid size")

	base64IDMu  sync.Mutex
	base64IDSeq uint32 = 0 // Sequence number to prevent sid overlaps.
)

func GenerateBase64ID(size int) (string, error) {
	if size <= 4 {
		return "", errBase64IDInvalidSize
	}

	base64IDMu.Lock()
	seq := base64IDSeq
	base64IDSeq++
	base64IDMu.Unlock()

	b := make([]byte, size)
	seqOffset := size - 4

	binary.BigEndian.PutUint32(b[seqOffset:], seq)

	_, err := rand.Read(b[:seqOffset+1])
	if err != nil {
		return "", err
	}

	encoded := base64.URLEncoding.EncodeToString(b)
	return encoded, nil
}

func (s *Server) generateSID() (sid string, err error) {
	for i := 0; ; i++ {
		sid, err = GenerateBase64ID(Base64IDSize)
		if err != nil {
			return "", err
		}

		if !s.store.Exists(sid) {
			return
		}

		if i == Base64IDMaxTry {
			return "", ErrBase64IDMaxTryReached
		}
	}
}
