package sio

import (
	"fmt"
	"os"
)

type DebugFunc = func(main string, v ...any)

func NoopDebugFunc(main string, v ...any) {}

func PrintDebugFunc(main string, _v ...any) {
	fmt.Print(main)
	for _, v := range _v {
		fmt.Print(": ")
		fmt.Print(v)
	}
	fmt.Print("\n")
	os.Stdout.Sync()
}
