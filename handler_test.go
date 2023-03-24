package sio

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAckCheckAckFunc(t *testing.T) {
	ackWithError := func(err error) {}
	ackWithInt := func(err int) {}
	ackWithoutError := func() {}
	ackWithReturn := func() string { return "123" }

	err := checkAckFunc(nil, false)
	assert.NotNil(t, err)

	err = checkAckFunc(ackWithError, true)
	assert.Nil(t, err)

	err = checkAckFunc(ackWithoutError, true)
	assert.NotNil(t, err)

	err = checkAckFunc(ackWithInt, true)
	assert.NotNil(t, err)

	err = checkAckFunc(ackWithReturn, false)
	assert.NotNil(t, err)
}
