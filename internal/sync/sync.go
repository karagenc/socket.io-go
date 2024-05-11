//go:build !sio_deadlock

package sync

import "sync"

type (
	Mutex     = sync.Mutex
	RWMutex   = sync.RWMutex
	Once      = sync.Once
	WaitGroup = sync.WaitGroup
	Locker    = sync.Locker
	Map       = sync.Map
	Cond      = sync.Cond
	Pool      = sync.Pool
)
