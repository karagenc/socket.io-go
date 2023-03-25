package polling

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/tomruk/socket.io-go/engine.io/parser"
)

func TestPollQueue(t *testing.T) {
	pq := newPollQueue()

	test := []*parser.Packet{
		mustCreatePacket(t, parser.PacketTypeOpen, false, nil),
		mustCreatePacket(t, parser.PacketTypeClose, false, nil),
		mustCreatePacket(t, parser.PacketTypePing, false, []byte("testing123")),
		mustCreatePacket(t, parser.PacketTypePong, false, []byte("testing123")),
		mustCreatePacket(t, parser.PacketTypeMessage, false, []byte("testing123")),
		mustCreatePacket(t, parser.PacketTypeMessage, true, []byte{0x0, 0x1, 0x2, 0x3}),
		mustCreatePacket(t, parser.PacketTypeUpgrade, false, nil),
		mustCreatePacket(t, parser.PacketTypeNoop, false, nil),
	}

	for _, p := range test {
		pq.add(p)
	}

	length := pq.len()

	packets := pq.get()
	assert.Equal(t, length, len(packets))
	assert.Equal(t, len(test), length)

	for i, p1 := range packets {
		p2 := test[i]

		if p1.Type != p2.Type {
			t.Fatal("packet types differ")
		} else if p1.IsBinary != p2.IsBinary {
			t.Fatal("isBinary fields differ")
		} else if !bytes.Equal(p1.Data, p2.Data) {
			t.Fatal("data doesn't match")
		}
	}
}

func TestPoll(t *testing.T) {
	pq := newPollQueue()

	const (
		waitFor = 500 * time.Millisecond

		// Slightly increase the time. The receive operation shouldn't exceed this duration.
		max = waitFor + 50*time.Millisecond
	)

	go func() {
		time.Sleep(waitFor)
		p := mustCreatePacket(t, parser.PacketTypeMessage, false, nil)
		pq.add(p)
	}()

	start := time.Now()
	packets := pq.poll(1 * time.Second)
	assert.Equal(t, 1, len(packets), "expected 1 packet")

	elapsed := time.Since(start)

	t.Logf("waitFor: %dms\nelapsed time: %dms\n", waitFor.Milliseconds(), elapsed.Milliseconds())

	if elapsed >= max {
		t.Fatal("it takes too much time to receive a packet from a pollQueue")
	}

	assert.Equal(t, 0, pq.len(), "pq should be empty after running pq.Poll")
}

func TestPollTimeout(t *testing.T) {
	pq := newPollQueue()

	const waitFor = 500 * time.Millisecond

	go func() {
		time.Sleep(waitFor)
		p := mustCreatePacket(t, parser.PacketTypeMessage, false, nil)
		pq.add(p)
	}()

	packets := pq.poll(waitFor - 50*time.Millisecond)
	assert.Equal(t, 0, len(packets), "expected 0 packet (because of the timeout)")
}

func mustCreatePacket(t *testing.T, packetType parser.PacketType, isBinary bool, data []byte) *parser.Packet {
	p, err := parser.NewPacket(packetType, isBinary, data)
	if err != nil {
		t.Fatal(err)
	}
	return p
}
