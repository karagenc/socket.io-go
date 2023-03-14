package sio

import (
	"fmt"
	"os"
)

type DebugFunc = func(main string, context string, v ...any)

func NoopDebugFunc(main string, context string, v ...any) {}

func PrintDebugFunc(main string, context string, _v ...any) {
	fmt.Print(main)
	if len(context) != 0 {
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
