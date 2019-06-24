package main

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
)

// Storage instance
var storageInstance Storage

// Waitgroup
var wg sync.WaitGroup

func main() {
	// Set max process number
	runtime.GOMAXPROCS(runtime.NumCPU())

	// Init in-memory storage
	storageInstance = NewMemStorage()

	// Set up router
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	logger := NewLogger()

	r.Mount("/", NewHandler(logger, NewSSE(storageInstance)).Router())

	// Garbage collection
	go func() {
		storageInstance.GC(os.Getenv("SSE_MAX_AGE"), &wg)
	}()

	// Server application
	wg.Add(1)
	go func() {
		logger.Fatalf("server: %+v", http.ListenAndServe(fmt.Sprintf(":%s", os.Getenv("APP_PORT")), r))
	}()

	// Graceful app shutdown
	var gracefulStop = make(chan os.Signal)
	signal.Notify(gracefulStop, syscall.SIGTERM)
	signal.Notify(gracefulStop, syscall.SIGINT)
	go func() {
		sig := <-gracefulStop
		logger.Infof("caught sig: %+v", sig)
		logger.Infof("Wait for 2 second to finish processing")
		time.Sleep(2 * time.Second)
		wg.Done()
	}()

	wg.Wait()
}
