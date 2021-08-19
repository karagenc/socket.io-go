package sio

var reservedEvents = map[string]bool{
	"error":             true,
	"ping":              true,
	"pong":              true,
	"connect":           true,
	"connect_error":     true,
	"connect_timeout":   true,
	"connecting":        true,
	"disconnect":        true,
	"disconnecting":     true,
	"reconnect":         true,
	"reconnect_attempt": true,
	"reconnect_failed":  true,
	"reconnect_error":   true,
	"newListener":       true,
	"removeListener":    true,
}

func IsEventReserved(eventName string) bool {
	isReserved, ok := reservedEvents[eventName]
	if ok && isReserved {
		return true
	}
	return false
}
