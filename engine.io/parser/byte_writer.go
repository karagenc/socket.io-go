package parser

import "io"

type byteWriter struct {
	w io.Writer
}

func (w byteWriter) WriteByte(b byte) error {
	_, err := w.w.Write([]byte{b})
	return err
}
