package main

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/cors"
)

// Storage instance
var storageInstance Storage

// Waitgroup
var wg sync.WaitGroup

func main() {
	// Set max process number
	n := 1
	if runtime.NumCPU() > 3 {
		n = runtime.NumCPU() / 2
	}
	runtime.GOMAXPROCS(n)

	logger := NewLogger()
	logger.Debugf("start running on %d cpu", n)

	// Init in-memory storage
	storageInstance = NewMemStorage()

	// Set up router
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	allowedOrigins := os.Getenv("ALLOWED_ORIGINS")
	ao := []string{"*"}
	if allowedOrigins != "" {
		ao = strings.Split(allowedOrigins, ",")
	}
	r.Use(cors.New(cors.Options{
		AllowedOrigins:   ao,
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "Last-Event-ID", "Origin"},
		AllowCredentials: true,
		MaxAge:           10080, // Maximum value not ignored by any of major browsers
	}).Handler)

	r.Mount("/", NewHandler(logger, NewSSE(storageInstance)).Router())

	// Garbage collection
	go func() {
		storageInstance.GC(os.Getenv("SSE_MAX_AGE"), os.Getenv("GC_PERIOD"), &wg)
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
