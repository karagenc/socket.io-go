//go:build sio_deadlock

package sync

import (
	"sync"

	"github.com/sasha-s/go-deadlock"
)

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

var OnceFunc = sync.OnceFunc
