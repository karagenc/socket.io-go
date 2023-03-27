package sync

import "sync"

type (
	Once      = sync.Once
	WaitGroup = sync.WaitGroup
	Locker    = sync.Locker
	Map       = sync.Map
	Cond      = sync.Cond
	Pool      = sync.Pool
)
