package sio

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type testFn func()

func TestOnOff(t *testing.T) {
	h := newHandlerStore[*testFn]()
	count := 0
	var f testFn = func() {
		count++
	}
	h.on(&f)

	all := h.getAll()
	c := all[0]
	(*c)()
	assert.Equal(t, 1, count)

	h.off(&f)
	all = h.getAll()
	assert.Equal(t, 0, len(all))
}

func TestOnce(t *testing.T) {
	h := newHandlerStore[*testFn]()
	count := 0
	var f testFn = func() {
		count++
	}
	h.once(&f)

	all := h.getAll()
	c := all[0]
	(*c)()
	assert.Equal(t, 1, count)

	all = h.getAll()
	assert.Equal(t, 0, len(all))

	h.once(&f)
	h.off(&f)

	all = h.getAll()
	assert.Equal(t, 0, len(all))
}

func TestOffAll(t *testing.T) {
	h := newHandlerStore[*testFn]()
	var f testFn = func() {}
	h.on(&f)
	h.once(&f)
	h.offAll()

	all := h.getAll()
	assert.Equal(t, 0, len(all))
}
