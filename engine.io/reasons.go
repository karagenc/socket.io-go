package eio

type Reason string

const (
	ReasonForcedClose    Reason = "forced close"
	ReasonPingTimeout    Reason = "ping timeout"
	ReasonTransportClose Reason = "transport close"
	ReasonTransportError Reason = "transport error"
)
