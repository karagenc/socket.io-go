package parser

import (
	"io"
)

const payloadDelimiter byte = 30

// `packets` must not be nil.
func EncodedPayloadsLen(packets ...*Packet) int {
	l := 0
	for i, packet := range packets {
		l += packet.EncodedLen(false)
		// Seperator
		if i != len(packets)-1 {
			l += 1
		}
	}
	return l
}

// packets must not be nil.
//
// Note: Writer should either implement io.ByteWriter
// or it should not have a problem with writing 1 byte at a time.
func EncodePayloads(w io.Writer, packets ...*Packet) error {
	bw, ok := w.(io.ByteWriter)
	if !ok {
		bw = byteWriter{w: w}
	}

	for i, packet := range packets {
		err := packet.Encode(w, false)
		if err != nil {
			return err
		}

		if i != len(packets)-1 {
			err := bw.WriteByte(payloadDelimiter)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func splitByte(buf []byte, delim byte) [][]byte {
	buffers := make([][]byte, 0, 1)

	last := 0
	for i, c := range buf {
		if c == delim {
			if i == len(buf) {
				buffers = append(buffers, []byte{})
			} else {
				buffers = append(buffers, buf[last:i])
			}
			last = i + 1
		}
	}

	if len(buffers) == 0 {
		buffers = append(buffers, buf)
	} else if last <= len(buf) {
		buffers = append(buffers, buf[last:])
	}
	return buffers
}

func DecodePayloads(r io.Reader) ([]*Packet, error) {
	packets := make([]*Packet, 0, 1) // Minimum 1 packet expected
	buf, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	splitted := splitByte(buf, payloadDelimiter)

	for _, sp := range splitted {
		packet, err := decode(sp, false)
		if err != nil {
			return nil, err
		}
		packets = append(packets, packet)
	}
	return packets, nil
}
