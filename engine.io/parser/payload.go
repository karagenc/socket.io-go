package parser

import (
	"bytes"
)

const payloadDelimiter byte = 30

// Argument must not be nil.
func EncodePayloads(packets ...*Packet) []byte {
	l := 0
	for i, packet := range packets {
		// Packet length
		l += 1 + len(packet.Data)

		// Delimiter
		if i != len(packets)-1 {
			l += 1
		}
	}

	b := make([]byte, 0, l)

	for i, packet := range packets {
		built := packet.Build(false)
		b = append(b, built...)

		if i != len(packets)-1 {
			b = append(b, []byte{payloadDelimiter}...)
		}
	}

	return b
}

func DecodePayloads(b []byte) ([]*Packet, error) {
	packets := make([]*Packet, 0, 1) // Minimum 1 packet expected
	splitted := bytes.Split(b, []byte{payloadDelimiter})

	for _, sp := range splitted {
		packet, err := Parse(sp, false)
		if err != nil {
			return nil, err
		}
		packets = append(packets, packet)
	}

	return packets, nil
}
