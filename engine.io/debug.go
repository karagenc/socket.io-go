package eio

import (
	"fmt"
	"io"
	"os"

	"github.com/tomruk/socket.io-go/internal/sync"
	"github.com/xiegeo/coloredgoroutine"
)

type (
	Debugger interface {
		Log(main string, v ...any)
		WithContext(context string) Debugger
		WithDynamicContext(context string, dynamicContext func() string) Debugger
	}

	noopDebugger struct{}

	printDebugger struct {
		stdout         io.Writer
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
	return &printDebugger{stdout: coloredgoroutine.Colors(os.Stdout)}
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
		fmt.Fprint(d.stdout, d.context)
		if len(dynamicContext) != 0 || len(main) != 0 || len(_v) != 0 {
			fmt.Fprint(d.stdout, ": ")
		}
	}
	if len(dynamicContext) != 0 {
		fmt.Fprint(d.stdout, dynamicContext)
		if len(main) != 0 || len(_v) != 0 {
			fmt.Fprint(d.stdout, ": ")
		}
	}
	if len(main) != 0 {
		fmt.Fprint(d.stdout, main)
		if len(_v) != 0 {
			fmt.Fprint(d.stdout, ": ")
		}
	}

	for i, v := range _v {
		if i != 0 {
			fmt.Fprint(d.stdout, ": ")
		}
		fmt.Fprint(d.stdout, v)
	}

	fmt.Fprint(d.stdout, "\n")
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
