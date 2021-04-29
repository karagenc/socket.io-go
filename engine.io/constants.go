package eio

import "time"

const (
	ProtocolVersion = 4

	defaultMaxBufferSize       int = 1e6 // 100 MB
	defaultWebSocketBufferSize     = 4096
	defaultPingTimeout             = time.Second * 20
	defaultPingInterval            = time.Second * 25
	defaultUpgradeTimeout          = time.Second * 10
)
