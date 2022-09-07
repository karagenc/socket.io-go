package sio

var clientReservedEvents = map[string]bool{
	"connect":        true,
	"connect_error":  true,
	"disconnect":     true,
	"disconnecting":  true,
	"newListener":    true,
	"removeListener": true,
}

var serverReservedEvents = map[string]bool{
	"connect":        true,
	"connect_error":  true,
	"disconnect":     true,
	"disconnecting":  true,
	"newListener":    true,
	"removeListener": true,
	"connection":     true,
	"error":          true,
}

func IsEventReservedForClient(eventName string) bool {
	isReserved, ok := clientReservedEvents[eventName]
	if ok && isReserved {
		return true
	}
	return false
}

func IsEventReservedForServer(eventName string) bool {
	isReserved, ok := serverReservedEvents[eventName]
	if ok && isReserved {
		return true
	}
	return false
}
