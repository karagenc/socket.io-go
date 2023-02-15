package sio

import (
	"fmt"
	"reflect"
	"sync"
)

type Auth struct {
	mu   sync.Mutex
	data any
}

func newAuth() *Auth {
	return new(Auth)
}

func (a *Auth) Set(data any) error {
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

func (a *Auth) Get() (data any) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.data
}
