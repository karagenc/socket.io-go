package eio

import (
	"time"

	"github.com/tomruk/socket.io-go/engine.io/parser"
)

const (
	ProtocolVersion = parser.ProtocolVersion

	defaultMaxBufferSize  int = 1e6 // 100 MB
	defaultPingTimeout        = time.Second * 20
	defaultPingInterval       = time.Second * 25
	defaultUpgradeTimeout     = time.Second * 10
)
