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
	h.On(&f)

	all := h.GetAll()
	c := all[0]
	(*c)()
	assert.Equal(t, 1, count)

	h.Off(&f)
	all = h.GetAll()
	assert.Equal(t, 0, len(all))
}

func TestOnce(t *testing.T) {
	h := newHandlerStore[*testFn]()
	count := 0
	var f testFn = func() {
		count++
	}
	h.Once(&f)

	all := h.GetAll()
	c := all[0]
	(*c)()
	assert.Equal(t, 1, count)

	all = h.GetAll()
	assert.Equal(t, 0, len(all))

	h.Once(&f)
	h.Off(&f)

	all = h.GetAll()
	assert.Equal(t, 0, len(all))
}

func TestOffAll(t *testing.T) {
	h := newHandlerStore[*testFn]()
	var f testFn = func() {}
	h.On(&f)
	h.Once(&f)
	h.OffAll()

	all := h.GetAll()
	assert.Equal(t, 0, len(all))
}
