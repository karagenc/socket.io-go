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

func TestPacketDecode(t *testing.T) {
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

		buf := bytes.NewBuffer(nil)
		l := p1.EncodedLen(true)
		err := p1.Encode(buf, true)
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, l, buf.Len())

		binaryData := p1.Type == PacketTypeMessage && p1.IsBinary == true

		p2, err := Decode(buf, binaryData)
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, p1.Type, p2.Type, "packet type doesn't match")
		assert.Equal(t, p1.IsBinary, p2.IsBinary, "isBinary doesn't match")

		if !bytes.Equal(p1.Data, p2.Data) {
			t.Fatal("packet data doesn't match")
		}

		// supportsBinary = false

		buf = bytes.NewBuffer(nil)
		l = p1.EncodedLen(false)
		err = p1.Encode(buf, false)
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, l, buf.Len())

		p2, err = Decode(buf, false)
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

func TestInvalidPacketType(t *testing.T) {
	p := mustCreatePacket(t, PacketTypePing, false, nil)
	buf := bytes.NewBuffer(nil)
	err := p.Encode(buf, false)
	if err != nil {
		t.Fatal(err)
	}

	buf.Bytes()[0] = 2 // Lower than 48

	_, err = Decode(buf, false)
	if err != errInvalidPacketType {
		t.Fatal("errInvalidPacketType expected")
	}
}

func TestPacketWrite(t *testing.T) {
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

		buf := bytes.NewBuffer(nil)
		err := packet.Encode(buf, true)
		if err != nil {
			t.Fatal(err)
		}

		if packet.Type != PacketTypeMessage || (packet.Type == PacketTypeMessage && packet.IsBinary == false) {
			if !assert.GreaterOrEqual(t, buf.Len(), 1, "minimum length for a non-binary packet is 1") {
				return
			}

			pt := buf.Bytes()[0]
			assert.Equal(t, packet.Type.ToChar(), pt, "packet type doesn't match")

			if !bytes.Equal(packet.Data, buf.Bytes()[1:]) {
				t.Fatal("packet data doesn't match")
			}
		} else {
			if !bytes.Equal(packet.Data, buf.Bytes()) {
				t.Fatal("packet data doesn't match")
			}
		}

		// supportsBinary = false

		buf = bytes.NewBuffer(nil)
		err = packet.Encode(buf, false)
		if err != nil {
			t.Fatal(err)
		}

		if packet.Type != PacketTypeMessage || (packet.Type == PacketTypeMessage && packet.IsBinary == false) {
			if !assert.GreaterOrEqual(t, buf.Len(), 1, "minimum length for a non-binary packet is 1") {
				return
			}

			pt := buf.Bytes()[0]
			assert.Equal(t, packet.Type.ToChar(), pt, "packet type doesn't match")

			if !bytes.Equal(packet.Data, buf.Bytes()[1:]) {
				t.Fatal("packet data doesn't match")
			}
		} else {
			assert.GreaterOrEqual(t, buf.Len(), 1, "minimum length for a base64 encoded binary packet is 1")

			assert.Equal(t, base64Prefix, buf.Bytes()[0], "a base64 encoded binary packet should start with base64Prefix: '%d'", base64Prefix)

			data := buf.Bytes()[1:]
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
