package transport

import (
	"reflect"
	"testing"
)

func TestCallbacks(t *testing.T) {
	callbacks := Callbacks{}
	callbacks.Set(nil, nil)

	v := reflect.ValueOf(callbacks)

	// Ensure that the all callbacks are non-nil.
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)

		if field.IsNil() {
			name := reflect.TypeOf(callbacks).Field(i).Name
			t.Errorf("%s is nil. It shouldn't be", name)
		}
	}
}
