package parser

import (
	"encoding/json"
	"fmt"
	"time"
)

var errOpenExpected = fmt.Errorf("parser: packet with a type of OPEN was expected")

type HandshakeResponse struct {
	SID          string   `json:"sid"`
	Upgrades     []string `json:"upgrades"`
	PingInterval int64    `json:"pingInterval"`
	PingTimeout  int64    `json:"pingTimeout"`
}

func (hr *HandshakeResponse) GetPingInterval() time.Duration {
	return time.Duration(hr.PingInterval) * time.Millisecond
}

func (hr *HandshakeResponse) GetPingTimeout() time.Duration {
	return time.Duration(hr.PingTimeout) * time.Millisecond
}

func ParseHandshakeResponse(p *Packet) (*HandshakeResponse, error) {
	if p.Type != PacketTypeOpen {
		return nil, errOpenExpected
	}

	hr := new(HandshakeResponse)
	err := json.Unmarshal(p.Data, hr)
	if err != nil {
		return nil, err
	}
	return hr, nil
}
