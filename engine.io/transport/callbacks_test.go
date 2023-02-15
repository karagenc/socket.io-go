package transport

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCallbacks(t *testing.T) {
	callbacks := Callbacks{}
	callbacks.Set(nil, nil)

	v := reflect.ValueOf(callbacks)
	assert.Equal(t, 2, v.NumField(), "number of fields must be 2, if not, that means another field is added. add that field to the test and increase the number")

	assert.NotNil(t, callbacks.onPacket.Load())
	assert.NotNil(t, callbacks.onClose.Load())
}
