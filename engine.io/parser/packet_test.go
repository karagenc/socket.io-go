package parser

import (
	"bytes"
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPacketCreate(t *testing.T) {
	var (
		packetType = PacketTypeOpen // Something other than MESSAGE
		isBinary   = true
		data       = []byte{}
	)

	_, err := NewPacket(packetType, isBinary, data)
	assert.Equal(t, errInvalidPacketType, err, "if the packet type is not MESSAGE, packet cannot contain a binary data")
}

func TestPacketParse(t *testing.T) {
	test := []*Packet{
		mustCreatePacket(t, PacketTypeOpen, false, nil),
		mustCreatePacket(t, PacketTypeClose, false, nil),
		mustCreatePacket(t, PacketTypePing, false, []byte("testing123")),
		mustCreatePacket(t, PacketTypePong, false, []byte("testing123")),
		mustCreatePacket(t, PacketTypeMessage, false, []byte("testing123")),
		mustCreatePacket(t, PacketTypeMessage, true, []byte{0x0, 0x1, 0x2, 0x3}),
		mustCreatePacket(t, PacketTypeUpgrade, false, nil),
		mustCreatePacket(t, PacketTypeNoop, false, nil),
	}

	for _, p1 := range test {

		// supportsBinary = true

		built := p1.Build(true)

		binaryData := p1.Type == PacketTypeMessage && p1.IsBinary == true

		p2, err := Parse(built, binaryData)
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, p1.Type, p2.Type, "packet type doesn't match")
		assert.Equal(t, p1.IsBinary, p2.IsBinary, "isBinary doesn't match")

		if !bytes.Equal(p1.Data, p2.Data) {
			t.Fatal("packet data doesn't match")
		}

		// supportsBinary = false

		built = p1.Build(false)

		p2, err = Parse(built, false)
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, p1.Type, p2.Type, "packet type doesn't match")
		assert.Equal(t, p1.IsBinary, p2.IsBinary, "isBinary doesn't match")

		if !bytes.Equal(p1.Data, p2.Data) {
			t.Fatal("packet data doesn't match")
		}
	}
}

func TestEmptyPacket(t *testing.T) {
	data := []byte{}
	_, err := Parse(data, false)
	if err != errInvalidPacketSize {
		t.Fatal("errInvalidPacketSize expected")
	}
}

func TestInvalidPacketType(t *testing.T) {
	p := mustCreatePacket(t, PacketTypePing, false, nil)
	built := p.Build(false)

	built[0] = 2 // Lower than 48

	_, err := Parse(built, false)
	if err != errInvalidPacketType {
		t.Fatal("errInvalidPacketType expected")
	}
}

func TestPacketBuild(t *testing.T) {
	test := []*Packet{
		mustCreatePacket(t, PacketTypeOpen, false, nil),
		mustCreatePacket(t, PacketTypeClose, false, nil),
		mustCreatePacket(t, PacketTypePing, false, []byte("testing123")),
		mustCreatePacket(t, PacketTypePong, false, []byte("testing123")),
		mustCreatePacket(t, PacketTypeMessage, false, []byte("testing123")),
		mustCreatePacket(t, PacketTypeMessage, true, []byte{0x0, 0x1, 0x2, 0x3}),
		mustCreatePacket(t, PacketTypeUpgrade, false, nil),
		mustCreatePacket(t, PacketTypeNoop, false, nil),
	}

	for _, packet := range test {

		// supportsBinary = true

		built := packet.Build(true)

		if packet.Type != PacketTypeMessage || (packet.Type == PacketTypeMessage && packet.IsBinary == false) {
			assert.GreaterOrEqual(t, len(built), 1, "minimum length for a non-binary packet is 1")

			pt := built[0]
			assert.Equal(t, packet.Type.ToChar(), pt, "packet type doesn't match")

			if !bytes.Equal(packet.Data, built[1:]) {
				t.Fatal("packet data doesn't match")
			}
		} else {
			if !bytes.Equal(packet.Data, built) {
				t.Fatal("packet data doesn't match")
			}
		}

		// supportsBinary = false

		built = packet.Build(false)

		if packet.Type != PacketTypeMessage || (packet.Type == PacketTypeMessage && packet.IsBinary == false) {
			assert.GreaterOrEqual(t, len(built), 1, "minimum length for a non-binary packet is 1")

			pt := built[0]
			assert.Equal(t, packet.Type.ToChar(), pt, "packet type doesn't match")

			if !bytes.Equal(packet.Data, built[1:]) {
				t.Fatal("packet data doesn't match")
			}
		} else {
			assert.GreaterOrEqual(t, len(built), 1, "minimum length for a base64 encoded binary packet is 1")

			assert.Equal(t, base64Prefix, built[0], "a base64 encoded binary packet should start with base64Prefix: '%s'", base64Prefix)

			data := built[1:]
			dl := base64.StdEncoding.DecodedLen(len(data))
			decoded := make([]byte, dl)

			n, err := base64.StdEncoding.Decode(decoded, data)
			if err != nil {
				t.Fatal(err)
			}
			decoded = decoded[:n]

			if !bytes.Equal(decoded, packet.Data) {
				t.Fatal("packet data doesn't match")
			}
		}
	}
}

func mustCreatePacket(t *testing.T, packetType PacketType, isBinary bool, data []byte) *Packet {
	p, err := NewPacket(packetType, isBinary, data)
	if err != nil {
		t.Fatal(err)
	}
	return p
}
