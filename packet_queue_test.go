package sio

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	eio "github.com/tomruk/socket.io-go/engine.io"
	"github.com/tomruk/socket.io-go/engine.io/parser"
)

func TestPacketQueue(t *testing.T) {
	t.Run("add, get, and reset", func(t *testing.T) {
		pq := newPacketQueue()

		pq.add(mustCreateEIOPacket(parser.PacketTypeMessage, false, nil))
		pq.add(mustCreateEIOPacket(parser.PacketTypePing, false, nil))

		packets := pq.get()
		require.Equal(t, 2, len(packets))
		require.Equal(t, 0, len(pq.packets))

		pq.add(mustCreateEIOPacket(parser.PacketTypeMessage, false, nil))
		pq.add(mustCreateEIOPacket(parser.PacketTypePing, false, nil))

		pq.reset()
		require.Equal(t, 0, len(pq.packets))
	})

	t.Run("poll", func(t *testing.T) {
		pq := newPacketQueue()

		go func() {
			pq.add(
				mustCreateEIOPacket(parser.PacketTypeMessage, false, nil),
				mustCreateEIOPacket(parser.PacketTypePing, false, nil),
			)
		}()

		packets, ok := pq.poll()
		require.True(t, ok)
		require.Equal(t, 2, len(packets))

		// This time, synchronously add packets.
		pq.add(
			mustCreateEIOPacket(parser.PacketTypeMessage, false, nil),
			mustCreateEIOPacket(parser.PacketTypePing, false, nil),
		)

		packets, ok = pq.poll()
		require.True(t, ok)
		require.Equal(t, 2, len(packets))

		// Do reset.
		pq.reset()
		packets, ok = pq.poll()
		require.False(t, ok)
		require.True(t, packets == nil)

		// `alreadyDrained` should be true.
		timedout := pq.waitForDrain(10000 * time.Second /* Forever */)
		require.False(t, timedout)
	})

	t.Run("waitForDrain", func(t *testing.T) {
		pq := newPacketQueue()

		// Artificially add packet to make `alreadyDrained` false.
		pq.packets = append(pq.packets, mustCreateEIOPacket(parser.PacketTypeMessage, false, nil))

		timedout := pq.waitForDrain(1 * time.Millisecond)
		require.True(t, timedout)
	})

	t.Run("pollAndSend", func(t *testing.T) {
		pq := newPacketQueue()
		tw := newTestWaiter(1)

		socket := eio.NewTestSocket("s1")
		socket.SendFunc = func(packets ...*parser.Packet) {
			assert.Equal(t, parser.PacketTypeMessage, packets[0].Type)
			assert.Equal(t, parser.PacketTypePing, packets[1].Type)
			assert.Equal(t, parser.PacketTypePong, packets[2].Type)
			assert.Equal(t, parser.PacketTypeNoop, packets[3].Type)
			tw.Done()
		}

		go pq.pollAndSend(socket)

		pq.add(
			mustCreateEIOPacket(parser.PacketTypeMessage, false, nil),
			mustCreateEIOPacket(parser.PacketTypePing, false, nil),
			mustCreateEIOPacket(parser.PacketTypePong, false, nil),
			mustCreateEIOPacket(parser.PacketTypeNoop, false, nil),
		)

		tw.Wait()
	})
}

func mustCreateEIOPacket(typ parser.PacketType, isBinary bool, data []byte) *parser.Packet {
	packet, err := parser.NewPacket(typ, isBinary, data)
	if err != nil {
		panic(err)
	}
	return packet
}
