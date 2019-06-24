package main

import (
	"net/http"
	"os"
)

func main() {
	if _, err := http.Get(os.Getenv("HEALTHCHECK_URL")); err != nil {
		os.Exit(1)
	}
}
