package parser

import (
	"testing"
	"time"

	"encoding/json"

	"github.com/stretchr/testify/require"
)

func TestHandshakeResponse(t *testing.T) {
	const (
		testSID                 = "123456789"
		testPingInterval  int64 = 5  // Seconds
		testPingTimeout   int64 = 10 // Seconds
		testMaxBufferSize int64 = 12
	)
	var testUpgrades = []string{"u1", "u2", "u3"}

	hr := &HandshakeResponse{
		SID:          testSID,
		Upgrades:     testUpgrades,
		PingInterval: testPingInterval,
		PingTimeout:  testPingTimeout,
		MaxPayload:   testMaxBufferSize,
	}

	data, err := json.Marshal(hr)
	if err != nil {
		t.Fatal(err)
	}

	p, err := NewPacket(PacketTypeOpen, false, data)
	if err != nil {
		t.Fatal(err)
	}

	hr, err = ParseHandshakeResponse(p)
	if err != nil {
		t.Fatal(err)
	}

	require.Equal(t, testSID, hr.SID)

	require.Equal(t, testPingInterval, hr.PingInterval)
	require.Equal(t, testPingTimeout, hr.PingTimeout)
	require.Equal(t, testMaxBufferSize, hr.MaxPayload)

	for i, u := range hr.Upgrades {
		require.Equal(t, testUpgrades[i], u)
	}
}

func TestHandshakeWithoutOpenPacket(t *testing.T) {
	p, err := NewPacket(PacketTypeClose, false, nil)
	if err != nil {
		t.Fatal(err)
	}

	_, err = ParseHandshakeResponse(p)
	if err == nil {
		t.Fatal("error expected")
	}
}

func TestInvalidJSON(t *testing.T) {
	invalidData := []byte("dssadsadsaada") // Non-JSON data

	p, err := NewPacket(PacketTypeOpen, false, invalidData)
	if err != nil {
		t.Fatal(err)
	}

	_, err = ParseHandshakeResponse(p)
	if err == nil {
		t.Fatal("error expected")
	}
}

func TestPingIntervalAndPingTimeout(t *testing.T) {
	const (
		testPingInterval int64 = 5  // Seconds
		testPingTimeout  int64 = 10 // Seconds
	)

	hr := &HandshakeResponse{
		PingInterval: testPingInterval,
		PingTimeout:  testPingTimeout,
	}

	require.Equal(t, time.Duration(testPingInterval)*time.Millisecond, hr.GetPingInterval())
	require.Equal(t, time.Duration(testPingTimeout)*time.Millisecond, hr.GetPingTimeout())
}
