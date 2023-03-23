package sio

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAuth(t *testing.T) {
	type S struct {
		Num int
	}
	s := &S{
		Num: 500,
	}
	auth := newAuth()
	err := auth.set(s)
	if err != nil {
		t.Fatal(err)
	}

	s, ok := auth.get().(*S)
	if !assert.True(t, ok) {
		t.Fail()
	}
	assert.Equal(t, s.Num, 500)

	err = auth.set("Donkey")
	assert.NotNil(t, err, "err must be non-nil for a string value")
}
