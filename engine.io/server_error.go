package eio

import (
	"encoding/json"
	"net/http"
)

type ServerError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func GetServerError(code int) (se ServerError, ok bool) {
	se, ok = serverErrors[code]
	return
}

const (
	ErrorUnknownTransport = iota
	ErrorUnknownSID
	ErrorBadHandshakeMethod
	ErrorBadRequest
	ErrorForbidden
	ErrorUnsupportedProtocolVersion
)

var serverErrors = map[int]ServerError{
	ErrorUnknownTransport: {
		Code:    0,
		Message: "Transport unknown",
	},
	ErrorUnknownSID: {
		Code:    1,
		Message: "Session ID unknown",
	},
	ErrorBadHandshakeMethod: {
		Code:    2,
		Message: "Bad handshake method",
	},
	ErrorBadRequest: {
		Code:    3,
		Message: "Bad request",
	},
	ErrorForbidden: {
		Code:    4,
		Message: "Forbidden",
	},
	ErrorUnsupportedProtocolVersion: {
		Code:    5,
		Message: "Unsupported protocol version",
	},
}

func writeServerError(w http.ResponseWriter, code int) {
	w.WriteHeader(http.StatusBadRequest)

	em, ok := serverErrors[code]
	if ok {
		data, _ := json.Marshal(&em)
		w.Write(data)
	}
}
