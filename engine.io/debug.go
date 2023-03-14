package eio

import (
	"fmt"
	"os"
)

type Debugger interface {
	Log(main string, v ...any)
	WithContext(context string) Debugger
	WithDynamicContext(context string, dynamicContext func() string) Debugger
}

type noopDebugger struct{}

func NewNoopDebugger() Debugger {
	return noopDebugger{}
}

func (d noopDebugger) Log(main string, _v ...any) {}

func (d noopDebugger) WithContext(context string) Debugger { return d }

func (d noopDebugger) WithDynamicContext(context string, _ func() string) Debugger { return d }

type printDebugger struct {
	context        string
	dynamicContext func() string
}

func NewPrintDebugger() Debugger {
	return new(printDebugger)
}

func (d *printDebugger) Log(main string, _v ...any) {
	fmt.Print(main)
	if len(d.context) != 0 {
		fmt.Print(d.context)
		fmt.Print(": ")
	}
	if d.dynamicContext != nil {
		context := d.dynamicContext()
		fmt.Print(context)
		fmt.Print(": ")
	}

	for _, v := range _v {
		fmt.Print(": ")
		fmt.Print(v)
	}

	fmt.Print("\n")
	os.Stdout.Sync()
}

func (d printDebugger) WithContext(context string) Debugger {
	d.context = context
	return &d
}

func (d printDebugger) WithDynamicContext(context string, dynamicContext func() string) Debugger {
	d.context = context
	d.dynamicContext = dynamicContext
	return &d
}
