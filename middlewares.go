package main

import (
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
)

func tokenFromQuery(r *http.Request) string {
	return r.URL.Query().Get("token")
}

func basicToken(t string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := r.URL.Query().Get("token")
			if token == "" {
				http.Error(w, http.StatusText(401), 401)
				return
			}
			if token != t {
				http.Error(w, http.StatusText(401), 401)
				return
			}
			// Token is authenticated, pass it through
			next.ServeHTTP(w, r)
		})
	}
}

func basicAuth(user, password string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if checkAuth(r, user, password) != nil {
				w.Header().Set("Content-Type", "text/plain")
				w.Header().Set("WWW-Authenticate", `Basic realm="Authentication"`)
				w.WriteHeader(401)
				w.Write([]byte("Unauthorized"))
				return
			}
			// Token is authenticated, pass it through
			next.ServeHTTP(w, r)
		})
	}
}

func checkAuth(r *http.Request, user, password string) error {
	unauthErr := errors.New("Unauthorized")
	s := strings.SplitN(r.Header.Get("Authorization"), " ", 2)
	if len(s) != 2 || s[0] != "Basic" {
		return unauthErr
	}
	b, err := base64.StdEncoding.DecodeString(s[1])
	if err != nil {
		return unauthErr
	}
	pair := strings.SplitN(string(b), ":", 2)
	if len(pair) != 2 {
		return unauthErr
	}
	u, p := pair[0], pair[1]
	if user != u || password != p {
		return unauthErr
	}
	return nil
}
