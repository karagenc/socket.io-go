package eio

import (
	"fmt"
	"os"
	"sync"
)

type (
	Debugger interface {
		Log(main string, v ...any)
		WithContext(context string) Debugger
		WithDynamicContext(context string, dynamicContext func() string) Debugger
	}

	noopDebugger struct{}

	printDebugger struct {
		context        string
		dynamicContext func() string
	}
)

func NewNoopDebugger() Debugger {
	return noopDebugger{}
}

func (d noopDebugger) Log(main string, _v ...any) {}

func (d noopDebugger) WithContext(context string) Debugger { return d }

func (d noopDebugger) WithDynamicContext(context string, _ func() string) Debugger { return d }

func NewPrintDebugger() Debugger {
	return new(printDebugger)
}

var printMu sync.Mutex

// Log each field, adding colon if there's a subsequent field.
func (d *printDebugger) Log(main string, _v ...any) {
	printMu.Lock()
	defer printMu.Unlock()

	dynamicContext := ""
	if d.dynamicContext != nil {
		dynamicContext = d.dynamicContext()
	}

	if len(d.context) != 0 {
		fmt.Print(d.context)
		if len(dynamicContext) != 0 || len(main) != 0 || len(_v) != 0 {
			fmt.Print(": ")
		}
	}
	if len(dynamicContext) != 0 {
		fmt.Print(dynamicContext)
		if len(main) != 0 || len(_v) != 0 {
			fmt.Print(": ")
		}
	}
	if len(main) != 0 {
		fmt.Print(main)
		if len(_v) != 0 {
			fmt.Print(": ")
		}
	}

	for i, v := range _v {
		if i != 0 {
			fmt.Print(": ")
		}
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
