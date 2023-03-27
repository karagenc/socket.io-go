//go:build deadlock

package sync

import "github.com/sasha-s/go-deadlock"

type (
	Mutex     = deadlock.Mutex
	RWMutex   = deadlock.RWMutex
	Once      = deadlock.Once
	WaitGroup = deadlock.WaitGroup
	Locker    = deadlock.Locker
	Map       = deadlock.Map
	Cond      = deadlock.Cond
	Pool      = deadlock.Pool
)
