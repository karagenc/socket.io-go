package parser

import (
	"encoding/base64"
	"fmt"
	"io"
)

type PacketType byte

const (
	PacketTypeOpen PacketType = iota
	PacketTypeClose
	PacketTypePing
	PacketTypePong
	PacketTypeMessage
	PacketTypeUpgrade
	PacketTypeNoop

	packetTypeMin = PacketTypeOpen
	packetTypeMax = PacketTypeNoop
)

func (p PacketType) ToChar() byte {
	b := byte(p)
	b += 48
	return b
}

func (p *PacketType) FromChar(b byte) error {
	if b < 48 || b > byte(48+packetTypeMax) {
		return errInvalidPacketType
	}

	b = b - 48
	*p = PacketType(b)
	return nil
}

const base64Prefix byte = 'b'

var (
	errInvalidPacketSize = fmt.Errorf("parser: invalid packet size")
	errInvalidPacketType = fmt.Errorf("parser: invalid packet type")
)

type Packet struct {
	IsBinary bool
	Type     PacketType
	Data     []byte
}

func NewPacket(packetType PacketType, isBinary bool, data []byte) (*Packet, error) {
	if packetType != PacketTypeMessage && isBinary {
		return nil, errInvalidPacketType
	}
	return &Packet{
		IsBinary: isBinary,
		Type:     packetType,
		Data:     data,
	}, nil
}

func (p *Packet) String() string {
	return fmt.Sprintf("type: %d | is binary: %t | data as string: `%s`", p.Type, p.IsBinary, string(p.Data))
}

func (p *Packet) EncodedLen(supportsBinary bool) int {
	if p.IsBinary {
		if supportsBinary {
			return len(p.Data)
		} else {
			return 1 + base64.StdEncoding.EncodedLen(len(p.Data))
		}
	}
	return 1 + len(p.Data)
}

// Note: Writer should either implement io.ByteWriter
// or should not have a problem with writing 1 byte at a time.
func (p *Packet) Encode(w io.Writer, supportsBinary bool) error {
	bw, ok := w.(io.ByteWriter)
	if !ok {
		bw = byteWriter{w: w}
	}

	if p.IsBinary {
		if supportsBinary {
			_, err := w.Write(p.Data)
			return err
		} else {
			err := bw.WriteByte(base64Prefix)
			if err != nil {
				return err
			}

			encoder := base64.NewEncoder(base64.StdEncoding, w)
			defer encoder.Close()

			_, err = encoder.Write(p.Data)
			return err
		}
	}

	err := bw.WriteByte(p.Type.ToChar())
	if err != nil {
		return err
	}

	_, err = w.Write(p.Data)
	return err
}

func Decode(r io.Reader, binaryFrame bool) (*Packet, error) {
	buf, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return decode(buf, binaryFrame)
}

func decode(data []byte, binaryFrame bool) (*Packet, error) {
	packet := new(Packet)

	if binaryFrame {
		packet.IsBinary = true
		packet.Type = PacketTypeMessage
		packet.Data = data
		return packet, nil
	}

	if len(data) < 1 {
		return nil, errInvalidPacketSize
	}

	packetType := data[0]

	if packetType == base64Prefix {
		packet.IsBinary = true
		packet.Type = PacketTypeMessage

		data = data[1:]
		dl := base64.StdEncoding.DecodedLen(len(data))
		packet.Data = make([]byte, dl)

		n, err := base64.StdEncoding.Decode(packet.Data, data)
		if err != nil {
			return nil, err
		}

		packet.Data = packet.Data[:n]
		return packet, nil
	}

	packet.IsBinary = false
	err := packet.Type.FromChar(packetType)
	packet.Data = data[1:]
	return packet, err
}
