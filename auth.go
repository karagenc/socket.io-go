package sio

import (
	"fmt"
	"reflect"
	"sync"
)

type auth struct {
	mu   sync.Mutex
	data any
}

func newAuth() *auth {
	return new(auth)
}

func (a *auth) set(data any) error {
	if data != nil {
		rt := reflect.TypeOf(data)
		k := rt.Kind()

		if k == reflect.Ptr {
			rt = rt.Elem()
			k = rt.Kind()
		}

		if k != reflect.Struct && k != reflect.Map {
			return fmt.Errorf("Auth.Set(): non-JSON data cannot be accepted. please provide a struct or map")
		}
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	a.data = data
	return nil
}

func (a *auth) get() (data any) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.data
}
