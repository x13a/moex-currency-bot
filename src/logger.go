package main

import "log"

type Logger struct{}

func (l *Logger) Infof(format string, args ...any) {
	log.Printf(format, args...)
}

func (l *Logger) Errorf(format string, args ...any) {
	log.Printf(format, args...)
}

func (l *Logger) Fatalf(format string, args ...any) {
	log.Fatalf(format, args...)
}
