package main

import (
	"net/http"
	"os"
)

// If this value is empty authenticator will not check for a password.
var password = os.Getenv("AUTH_PASSWORD")

// This is a very simple function to demonstrate authentication.
// Of course, do not use this in production.
func authenticator(w http.ResponseWriter, r *http.Request) (ok bool) {
	// Bypass the auth check if a password is not provided.
	if password == "" {
		return true
	}

	p := r.Header.Get("Authorization")
	return p == password
}
