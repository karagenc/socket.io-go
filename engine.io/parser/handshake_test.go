package parser

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestHandshakeResponse(t *testing.T) {
	const (
		testSID                = "123456789"
		testPingInterval int64 = 5  // Seconds
		testPingTimeout  int64 = 10 // Seconds
	)
	var testUpgrades = []string{"u1", "u2", "u3"}

	hr := &HandshakeResponse{
		SID:          testSID,
		Upgrades:     testUpgrades,
		PingInterval: testPingInterval,
		PingTimeout:  testPingTimeout,
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

	assert.Equal(t, testSID, hr.SID)

	assert.Equal(t, testPingInterval, hr.PingInterval)
	assert.Equal(t, testPingTimeout, hr.PingTimeout)

	for i, u := range hr.Upgrades {
		assert.Equal(t, testUpgrades[i], u)
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

	assert.Equal(t, time.Duration(testPingInterval)*time.Millisecond, hr.GetPingInterval())
	assert.Equal(t, time.Duration(testPingTimeout)*time.Millisecond, hr.GetPingTimeout())
}
