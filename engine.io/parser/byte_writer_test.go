package parser

import (
	"bytes"
	"testing"
)

func TestWriter(t *testing.T) {
	buf := bytes.NewBuffer(nil)

	bw := byteWriter{w: buf}

	err := bw.WriteByte('A')
	if err != nil {
		t.Fatal(err)
	}

	err = bw.WriteByte('B')
	if err != nil {
		t.Fatal(err)
	}

	err = bw.WriteByte('C')
	if err != nil {
		t.Fatal(err)
	}

	err = bw.WriteByte('D')
	if err != nil {
		t.Fatal(err)
	}

	err = bw.WriteByte('E')
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(buf.Bytes(), []byte("ABCDE")) {
		t.Fatal("values are not equal")
	}
}
