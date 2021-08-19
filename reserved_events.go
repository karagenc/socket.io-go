package sio

var reservedEvents = map[string]bool{
	"connect":        true,
	"connect_error":  true,
	"disconnect":     true,
	"disconnecting":  true,
	"newListener":    true,
	"removeListener": true,
}

func IsEventReserved(eventName string) bool {
	isReserved, ok := reservedEvents[eventName]
	if ok && isReserved {
		return true
	}
	return false
}
