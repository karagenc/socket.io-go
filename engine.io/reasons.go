package eio

type Reason string

const (
	ReasonTransportError Reason = "transport error"
	ReasonTransportClose Reason = "transport close"
	ReasonForcedClose    Reason = "forced close"
	ReasonPingTimeout    Reason = "ping timeout"
	ReasonParseError     Reason = "parse error"
)
