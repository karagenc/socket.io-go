package webtransport

import (
	"encoding/binary"
	"io"

	"github.com/karagenc/socket.io-go/engine.io/parser"
)

type clientOpenPacketData struct {
	SID string `json:"sid"`
}

func send(w io.Writer, packet *parser.Packet) error {
	var (
		header     []byte
		encodedLen = packet.EncodedLen(true)
	)
	if encodedLen < 126 {
		header = make([]byte, 1)
		header[0] = byte(encodedLen)
	} else if encodedLen < 65536 {
		header = make([]byte, 3)
		header[0] = 126
		binary.BigEndian.PutUint16(header[1:], uint16(encodedLen))
	} else {
		header = make([]byte, 9)
		header[0] = 127
		binary.BigEndian.PutUint64(header[1:], uint64(encodedLen))
	}
	if packet.IsBinary {
		header[0] |= 0x80
	}

	_, err := w.Write(header)
	if err != nil {
		return err
	}
	return packet.Encode(w, true)
}

func nextPacket(r io.Reader) (*parser.Packet, error) {
	var firstByte [1]byte
	_, err := io.ReadFull(r, firstByte[:])
	if err != nil {
		return nil, err
	}

	const (
		ReadHeader = iota
		ReadExtendedLen16
		ReadExtendedLen64
		ReadPayload
	)

	var (
		state       = ReadHeader
		expectedLen int
		isBinary    bool
	)

	for {
		switch state {
		case ReadHeader:
			expectedLen = int(firstByte[0] & 0x7f)
			isBinary = firstByte[0]&0x80 == 0x80
			if expectedLen < 126 {
				state = ReadPayload
			} else if expectedLen == 126 {
				state = ReadExtendedLen16
			} else {
				state = ReadExtendedLen64
			}
		case ReadExtendedLen16:
			var header [2]byte
			_, err = io.ReadFull(r, header[:])
			if err != nil {
				return nil, err
			}
			expectedLen = int(binary.BigEndian.Uint16(header[:]))
			state = ReadPayload
		case ReadExtendedLen64:
			var header [8]byte
			_, err = io.ReadFull(r, header[:])
			if err != nil {
				return nil, err
			}
			expectedLen = int(binary.BigEndian.Uint32(header[:]))
			state = ReadPayload
		case ReadPayload:
			return parser.DecodeWithLen(r, isBinary, expectedLen)
		}
	}
}
