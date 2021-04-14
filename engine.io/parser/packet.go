package parser

import (
	"encoding/base64"
	"fmt"
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

func Parse(data []byte, binaryData bool) (*Packet, error) {
	packet := new(Packet)

	if binaryData {
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
		packet.Data = packet.Data[:n]
		return packet, err
	}

	packet.IsBinary = false
	err := packet.Type.FromChar(packetType)
	packet.Data = data[1:]
	return packet, err
}

func (p *Packet) Build(supportsBinary bool) []byte {
	if p.IsBinary {
		if supportsBinary {
			return p.Data
		} else {
			el := base64.StdEncoding.EncodedLen(len(p.Data))
			b := make([]byte, 1+el)

			b[0] = base64Prefix
			base64.StdEncoding.Encode(b[1:], p.Data)
			return b
		}
	}

	b := make([]byte, 1+len(p.Data))
	b[0] = p.Type.ToChar()
	copy(b[1:], p.Data)
	return b
}
