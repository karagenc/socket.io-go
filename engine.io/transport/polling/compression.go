package polling

import (
	"compress/gzip"
	"io"
	"net/http"
)

func compressedReader(resp *http.Response) (r io.ReadCloser, err error) {
	switch resp.Header.Get("Content-Encoding") {
	case "gzip":
		r, err = gzip.NewReader(resp.Body)
		if err != nil {
			return nil, err
		}
	default:
		r = resp.Body
	}
	return
}
