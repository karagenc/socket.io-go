package sio

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tomruk/socket.io-go/engine.io/parser"
	"github.com/tomruk/socket.io-go/internal/utils"
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

		packets, ok, closed := pq.poll()
		require.False(t, closed)
		require.True(t, ok)
		require.Equal(t, 2, len(packets))

		// This time, synchronously add packets.
		pq.add(
			mustCreateEIOPacket(parser.PacketTypeMessage, false, nil),
			mustCreateEIOPacket(parser.PacketTypePing, false, nil),
		)

		packets, ok, closed = pq.poll()
		require.False(t, closed)
		require.True(t, ok)
		require.Equal(t, 2, len(packets))

		pq.reset()
		packets, ok, closed = pq.poll()
		require.False(t, closed)
		require.False(t, ok)
		require.True(t, packets == nil)

		// `alreadyDrained` should be true.
		timedout := pq.waitForDrain(10000 * time.Second /* Forever */)
		require.False(t, timedout)

		pq.close()
		packets, ok, closed = pq.poll()
		require.True(t, closed)
		require.False(t, ok)
		require.True(t, packets == nil)
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
		tw := utils.NewTestWaiter(1)

		socket := utils.NewTestSocket("s1")
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
