package sio

type BroadcastOptions struct {
	Rooms  map[string]interface{}
	Except map[string]interface{}
	Flags  BroadcastFlags
}

func newBroadcastOptions() *BroadcastOptions {
	return &BroadcastOptions{
		Rooms:  make(map[string]interface{}),
		Except: make(map[string]interface{}),
	}
}

type BroadcastFlags struct{}

type broadcastOperator struct {
	adapter Adapter
}

func newBroadcastOperator(adapter Adapter) *broadcastOperator {
	return &broadcastOperator{
		adapter: adapter,
	}
}
