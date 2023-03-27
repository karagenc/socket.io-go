//go:build deadlock

package sync

import (
	"github.com/sasha-s/go-deadlock"
)

type (
	Mutex   = deadlock.Mutex
	RWMutex = deadlock.RWMutex
)
