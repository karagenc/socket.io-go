package sio

var clientReservedEvents = map[string]bool{
	"connect":        true,
	"connect_error":  true,
	"disconnect":     true,
	"disconnecting":  true,
	"newListener":    true,
	"removeListener": true,
}

func IsEventReservedForClient(eventName string) bool {
	isReserved, ok := clientReservedEvents[eventName]
	if ok && isReserved {
		return true
	}
	return false
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

func IsEventReservedForServer(eventName string) bool {
	isReserved, ok := serverReservedEvents[eventName]
	if ok && isReserved {
		return true
	}
	return false
}

var nspReservedEvents = map[string]bool{
	"connect":       true,
	"connection":    true,
	"new_namespace": true,
}

func IsEventReservedForNsp(eventName string) bool {
	isReserved, ok := nspReservedEvents[eventName]
	if ok && isReserved {
		return true
	}
	return false
}
