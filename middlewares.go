package main

import (
	"net/http"
	"os"
)

func tokenFromQuery(r *http.Request) string {
	return r.URL.Query().Get("token")
}

func basicToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get("token")
		if token == "" {
			http.Error(w, http.StatusText(401), 401)
			return
		}
		if token != os.Getenv("BASIC_TOKEN") {
			http.Error(w, http.StatusText(401), 401)
			return
		}
		// Token is authenticated, pass it through
		next.ServeHTTP(w, r)
	})
}
