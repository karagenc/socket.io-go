package parser

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEncodeDecodePayloads(t *testing.T) {
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

	encoded := EncodePayloads(test...)
	assert.Greater(t, len(encoded), 0)

	packets, err := DecodePayloads(encoded)
	if err != nil {
		t.Fatal(err)
	}

	for i, p1 := range packets {
		p2 := test[i]

		assert.Equal(t, p2.Type, p1.Type, "packet type doesn't match")
		assert.Equal(t, p2.IsBinary, p1.IsBinary, "isBinary doesn't match")

		if !bytes.Equal(p1.Data, p2.Data) {
			t.Fatal("packet data doesn't match")
		}
	}
}

func TestDecodeSinglePayload(t *testing.T) {
	test := mustCreatePacket(t, PacketTypeMessage, true, []byte{0x0, 0x1, 0x2, 0x3})

	encoded := EncodePayloads(test)
	assert.Greater(t, len(encoded), 0)

	packets, err := DecodePayloads(encoded)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 1, len(packets))

	p1 := test
	p2 := packets[0]

	assert.Equal(t, p2.Type, p1.Type, "packet type doesn't match")
	assert.Equal(t, p2.IsBinary, p1.IsBinary, "isBinary doesn't match")

	if !bytes.Equal(p1.Data, p2.Data) {
		t.Fatal("packet data doesn't match")
	}
}

func TestDecodeInvalidPayload(t *testing.T) {
	test := mustCreatePacket(t, PacketTypeMessage, true, []byte{0x0, 0x1, 0x2, 0x3})

	encoded := EncodePayloads(test)
	assert.Greater(t, len(encoded), 0)

	encoded[0] = 2 // Lower than 48. To provoke errInvalidPacketType.

	_, err := DecodePayloads(encoded)
	if err != errInvalidPacketType {
		t.Fatal("errInvalidPacketType expected")
	}
}
