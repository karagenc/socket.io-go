//go:build !deadlock

package sync

import (
	"sync"
)

type (
	Mutex   = sync.Mutex
	RWMutex = sync.RWMutex
)
