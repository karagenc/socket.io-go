package main

import (
	"net/http"
	"os"
)

// Used by the CORS middleware. If this value is empty, corsMiddleware will not be used.
var allowOrigin = os.Getenv("ALLOW_ORIGIN")

// This is a very basic middleware for this example. On a production environment you're recommended to use a better one.
func corsMiddleware(next http.Handler, allowOrigin string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", allowOrigin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}

		next.ServeHTTP(w, r)
	})
}
