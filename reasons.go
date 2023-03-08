package sio

import eio "github.com/tomruk/socket.io-go/engine.io"

type Reason = eio.Reason

const (
	ReasonIOServerDisconnect Reason = "io server disconnect"
	ReasonIOClientDisconnect Reason = "io client disconnect"
	ReasonForcedClose        Reason = "forced close"
	ReasonPingTimeout        Reason = "ping timeout"
	ReasonTransportClose     Reason = "transport close"
	ReasonTransportError     Reason = "transport error"
	ReasonParseError         Reason = "parse error"
)

const (
	ReasonServerShuttingDown        Reason = "server shutting down"
	ReasonForcedServerClose         Reason = "forced server close"
	ReasonClientNamespaceDisconnect Reason = "client namespace disconnect"
	ReasonServerNamespaceDisconnect Reason = "server namespace disconnect"
)
