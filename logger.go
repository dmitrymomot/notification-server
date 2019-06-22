package main

import (
	"fmt"
	"log"
)

type (
	// Logger interface
	Logger interface {
		Debug(v interface{})
		Debugf(format string, v ...interface{})
		Info(v interface{})
		Infof(format string, v ...interface{})
		Warn(v interface{})
		Warnf(format string, v ...interface{})
		Error(v interface{})
		Errorf(format string, v ...interface{})
		Fatal(v interface{})
		Fatalf(format string, v ...interface{})
		Panic(v interface{})
		Panicf(format string, v ...interface{})
	}

	logger struct {
	}
)

// NewLogger return new instance of logger
func NewLogger() Logger {
	return &logger{}
}

// Debug func
func (l *logger) Debug(v interface{}) {
	log.Println(fmt.Sprintf("[debug] %v", v))
}

// Debugf func
func (l *logger) Debugf(format string, v ...interface{}) {
	log.Printf("[debug] %s", fmt.Sprintf(format, v...))
}

// Info func
func (l *logger) Info(v interface{}) {
	log.Println(fmt.Sprintf("[info] %v", v))
}

// Infof func
func (l *logger) Infof(format string, v ...interface{}) {
	log.Printf("[info] %s", fmt.Sprintf(format, v...))
}

// Warn func
func (l *logger) Warn(v interface{}) {
	log.Println(fmt.Sprintf("[warn] %v", v))
}

// Warnf func
func (l *logger) Warnf(format string, v ...interface{}) {
	log.Printf("[warn] %s", fmt.Sprintf(format, v...))
}

// Error func
func (l *logger) Error(v interface{}) {
	log.Println(fmt.Sprintf("[error] %v", v))
}

// Errorf func
func (l *logger) Errorf(format string, v ...interface{}) {
	log.Printf("[error] %s", fmt.Sprintf(format, v...))
}

// Fatal func
func (l *logger) Fatal(v interface{}) {
	log.Fatalln(fmt.Sprintf("[fatal] %v", v))
}

// Fatalf func
func (l *logger) Fatalf(format string, v ...interface{}) {
	log.Fatalf("[fatal] %s", fmt.Sprintf(format, v...))
}

// Panic func
func (l *logger) Panic(v interface{}) {
	log.Panicln(fmt.Sprintf("[panic] %v", v))
}

// Panicf func
func (l *logger) Panicf(format string, v ...interface{}) {
	log.Panicf("[panic] %s", fmt.Sprintf(format, v...))
}
