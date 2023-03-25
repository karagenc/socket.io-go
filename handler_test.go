package sio

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAckCheckAckFunc(t *testing.T) {
	ackWithError := func(err error) {}
	ackWithInt := func(err int) {}
	ackWithoutError := func() {}
	ackWithReturn := func() string { return "123" }

	err := checkAckFunc(nil, false)
	require.Error(t, err)

	err = checkAckFunc(ackWithError, true)
	require.NoError(t, err)

	err = checkAckFunc(ackWithoutError, true)
	require.Error(t, err)

	err = checkAckFunc(ackWithInt, true)
	require.Error(t, err)

	err = checkAckFunc(ackWithReturn, false)
	require.Error(t, err)
}
