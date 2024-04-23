package main

import (
	"fmt"
	"io"
	"math"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gookit/color"
	sio "github.com/tomruk/socket.io-go"
	"golang.org/x/term"
)

var colors = []string{
	"#e21400", "#91580f", "#f8a700", "#f78b00",
	"#58dc00", "#287b00", "#a8f07a", "#4ae8c4",
	"#3b88eb", "#3824aa", "#a700ff", "#d300e7",
}

func getUsernameColor(username string) color.RGBColor {
	hash := 7
	for _, r := range username {
		hash = int(r) + (hash << 5) - hash
	}
	index := int(math.Abs(float64(hash % len(colors))))
	return color.Hex(colors[index])
}

func initTerm(socket sio.ClientSocket) (*term.Terminal, func(code int), error) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	oldState, err := term.MakeRaw(0)
	if err != nil {
		return nil, nil, err
	}

	exitFunc := func(code int) {
		term.Restore(0, oldState)
		fmt.Println()
		os.Exit(code)
	}
	go func() {
		<-c
		exitFunc(0)
	}()

	t := &typing{socket: socket, typing: make(chan struct{}, 1)}
	go t.loop()
	stdin := io.TeeReader(os.Stdin, t)
	screen := struct {
		io.Reader
		io.Writer
	}{stdin, os.Stdout}

	term := term.NewTerminal(screen, "")
	term.SetPrompt(string(term.Escape.Blue) + "> " + string(term.Escape.Reset))

	return term, exitFunc, nil
}

type typing struct {
	socket sio.ClientSocket
	typing chan struct{}
}

func (t *typing) Write(p []byte) (n int, _ error) {
	n = len(p)
	if n > 0 {
		if len(p) == 1 && p[0] == 0x0d {
			return
		}
		t.typing <- struct{}{}
	}
	return
}

func (t *typing) loop() {
	var (
		stopInterval = 400 * time.Millisecond
		last         time.Time
		isTyping     = false
	)
	for {
		select {
		case <-time.After(stopInterval):
			if isTyping {
				t.socket.Emit("stop typing")
				isTyping = false
			}
		case <-t.typing:
			if time.Since(last) > stopInterval {
				isTyping = true
				t.socket.Emit("typing")
			}
			last = time.Now()
		}
	}
}
