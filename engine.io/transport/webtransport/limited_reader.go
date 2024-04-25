package webtransport

import (
	"fmt"
	"io"
)

var ErrLimitReached = fmt.Errorf("webtransport: maximum read limit (set via `MaxBufferSize`) is reached")

type limitedReader struct {
	r     io.Reader
	limit int64
	read  int64
}

func newLimitedReader(r io.Reader, limit int64) *limitedReader {
	return &limitedReader{r: r, limit: limit}
}

func (l *limitedReader) Read(p []byte) (n int, err error) {
	n, err = l.r.Read(p)
	l.read += int64(n)
	if l.read > l.limit {
		err = ErrLimitReached
	}
	return
}
