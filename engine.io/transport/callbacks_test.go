package transport

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCallbacks(t *testing.T) {
	callbacks := Callbacks{}
	callbacks.Set(nil, nil)

	v := reflect.ValueOf(callbacks)
	require.Equal(t, 2, v.NumField(), "number of fields must be 2, if not, that means another field is added. add that field to the test and increase the number")

	require.NotNil(t, callbacks.onPacket.Load())
	require.NotNil(t, callbacks.onClose.Load())
}
