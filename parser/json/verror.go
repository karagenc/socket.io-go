package jsonparser

import (
	"fmt"
	"reflect"
)

var (
	errNonInterfaceableValue = fmt.Errorf("non-interfaceable value")
	errNonSettableValue      = fmt.Errorf("non-settable value")
)

// This is a specific error type used with a reflect.Value.
type ValueError struct {
	err   error
	Value reflect.Value
}

func (e *ValueError) Error() string {
	typeString := "<invalid>"
	if e.Value.IsValid() {
		typeString = e.Value.Type().String()
	}
	return fmt.Sprintf("parser/json: error with value %s: %v", typeString, e.err)
}

func (e *ValueError) Unwrap() error {
	return e.err
}
