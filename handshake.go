package sio

import (
	"encoding/json"
	"time"
)

type Handshake struct {
	// Date of creation.
	Time time.Time

	// Authentication data
	Auth json.RawMessage
}
