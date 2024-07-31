package sio

import eio "github.com/karagenc/socket.io-go/engine.io"

type Debugger = eio.Debugger

var (
	NewPrintDebugger = eio.NewPrintDebugger
	newNoopDebugger  = eio.NewNoopDebugger
)
