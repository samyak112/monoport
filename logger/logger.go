package logger

import (
	"fmt"
	"log"
)

// SimpleLogger implements LeveledLogger by printing to standard log
type SimpleLogger struct{}

// helper to print message with prefix + static message appended
func (l *SimpleLogger) log(level, msg string) {
	staticMsg := " [this is from custom logger]"
	log.Printf("[%s] %s%s", level, msg, staticMsg)
}

// helper to print formatted message with prefix + static message appended
func (l *SimpleLogger) logf(level, format string, args ...interface{}) {
	// Format the original message
	msg := fmt.Sprintf(format, args...)
	// Append static message
	staticMsg := " [this is from custom logger]"
	log.Printf("[%s] %s%s", level, msg, staticMsg)
}

func (l *SimpleLogger) Trace(msg string) { l.log("TRACE", msg) }
func (l *SimpleLogger) Tracef(format string, args ...interface{}) {
	l.logf("TRACE", format, args...)
}
func (l *SimpleLogger) Debug(msg string) { l.log("DEBUG", msg) }
func (l *SimpleLogger) Debugf(format string, args ...interface{}) {
	l.logf("DEBUG", format, args...)
}
func (l *SimpleLogger) Info(msg string) { l.log("INFO", msg) }
func (l *SimpleLogger) Infof(format string, args ...interface{}) {
	l.logf("INFO", format, args...)
}
func (l *SimpleLogger) Warn(msg string) { l.log("WARN", msg) }
func (l *SimpleLogger) Warnf(format string, args ...interface{}) {
	l.logf("WARN", format, args...)
}
func (l *SimpleLogger) Error(msg string) { l.log("ERROR", msg) }
func (l *SimpleLogger) Errorf(format string, args ...interface{}) {
	l.logf("ERROR", format, args...)
}
