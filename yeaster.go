package sio

import (
	"math"
	"sync"
	"time"
)

type yeaster struct {
	alphabet string
	length   int
	m        map[rune]int

	seed float64
	prev string
	mu   sync.Mutex
}

func newYeaster() *yeaster {
	const alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz-_"
	y := &yeaster{
		alphabet: alphabet,
		length:   len(alphabet),
		m:        make(map[rune]int),
	}
	for i, c := range y.alphabet {
		y.m[c] = i
	}
	return y
}

func (y *yeaster) yeast() string {
	y.mu.Lock()
	defer y.mu.Unlock()
	now := y.encode(float64(time.Now().Unix()))

	if now != y.prev {
		y.seed = 0
		y.prev = now
		return now
	}

	y.seed += 1
	return now + "." + y.encode(y.seed)
}

func (y *yeaster) encode(num float64) string {
	encoded := ""
	for {
		encoded = string(y.alphabet[int(num)%y.length]) + encoded
		num = math.Floor(num / float64(y.length))
		if num <= 0 {
			break
		}
	}
	return encoded
}

func (y *yeaster) decode(s string) int {
	decoded := 0
	for _, c := range s {
		decoded = decoded*y.length + y.m[c]
	}
	return decoded
}
