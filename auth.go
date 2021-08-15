package sio

import (
	"fmt"
	"reflect"
	"sync"
)

type Auth struct {
	mu   sync.Mutex
	data interface{}
}

var errAuthInvalidValue = fmt.Errorf("Auth.Set(): non-JSON data cannot be accepted. Please provide a struct or map")

func newAuth() *Auth {
	return new(Auth)
}

func (a *Auth) Set(data interface{}) error {
	if data != nil {
		rt := reflect.TypeOf(data)
		k := rt.Kind()
		if k != reflect.Struct && k != reflect.Map {
			return errAuthInvalidValue
		}
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	a.data = data
	return nil
}

func (a *Auth) Get() (data interface{}) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.data
}
