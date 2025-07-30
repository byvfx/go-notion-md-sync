package util

import (
	"fmt"
	"io"
	"log"
	"os"
	"runtime"
	"strings"
	"time"
)

// LogLevel represents different logging levels
type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
	FATAL
)

// String returns the string representation of the log level
func (l LogLevel) String() string {
	switch l {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	case FATAL:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// Logger provides structured logging functionality
type Logger struct {
	level  LogLevel
	output io.Writer
	logger *log.Logger
}

// NewLogger creates a new logger instance
func NewLogger(level LogLevel, output io.Writer) *Logger {
	if output == nil {
		output = os.Stdout
	}

	return &Logger{
		level:  level,
		output: output,
		logger: log.New(output, "", 0), // We'll handle our own formatting
	}
}

// NewDefaultLogger creates a logger with sensible defaults
func NewDefaultLogger() *Logger {
	return NewLogger(INFO, os.Stdout)
}

// SetLevel sets the minimum log level
func (l *Logger) SetLevel(level LogLevel) {
	l.level = level
}

// GetLevel returns the current log level
func (l *Logger) GetLevel() LogLevel {
	return l.level
}

// shouldLog determines if a message should be logged based on level
func (l *Logger) shouldLog(level LogLevel) bool {
	return level >= l.level
}

// formatMessage formats a log message with timestamp, level, and caller info
func (l *Logger) formatMessage(level LogLevel, msg string, includeStack bool) string {
	timestamp := time.Now().Format("2006-01-02 15:04:05")

	// Get caller information
	_, file, line, ok := runtime.Caller(3) // Skip formatMessage, log method, and public method
	if !ok {
		file = "unknown"
		line = 0
	}

	// Extract just the filename from the full path
	if idx := strings.LastIndex(file, "/"); idx != -1 {
		file = file[idx+1:]
	}

	location := fmt.Sprintf("%s:%d", file, line)

	// Format the message
	formatted := fmt.Sprintf("[%s] %s %s - %s",
		timestamp,
		level.String(),
		location,
		msg,
	)

	return formatted
}

// log is the internal logging method
func (l *Logger) log(level LogLevel, format string, args ...interface{}) {
	if !l.shouldLog(level) {
		return
	}

	msg := format
	if len(args) > 0 {
		msg = fmt.Sprintf(format, args...)
	}

	formatted := l.formatMessage(level, msg, level >= ERROR)
	l.logger.Println(formatted)

	// For FATAL level, terminate the program
	if level == FATAL {
		os.Exit(1)
	}
}

// Debug logs a debug message
func (l *Logger) Debug(format string, args ...interface{}) {
	l.log(DEBUG, format, args...)
}

// Info logs an info message
func (l *Logger) Info(format string, args ...interface{}) {
	l.log(INFO, format, args...)
}

// Warn logs a warning message
func (l *Logger) Warn(format string, args ...interface{}) {
	l.log(WARN, format, args...)
}

// Error logs an error message
func (l *Logger) Error(format string, args ...interface{}) {
	l.log(ERROR, format, args...)
}

// Fatal logs a fatal message and exits
func (l *Logger) Fatal(format string, args ...interface{}) {
	l.log(FATAL, format, args...)
}

// WithError logs an error with the error details
func (l *Logger) WithError(err error, format string, args ...interface{}) {
	if err == nil {
		l.log(ERROR, format, args...)
		return
	}

	msg := format
	if len(args) > 0 {
		msg = fmt.Sprintf(format, args...)
	}

	l.log(ERROR, "%s: %v", msg, err)
}

// Progress logs a progress message (always shown regardless of level)
func (l *Logger) Progress(format string, args ...interface{}) {
	msg := format
	if len(args) > 0 {
		msg = fmt.Sprintf(format, args...)
	}

	// Progress messages are always shown with a simple format
	_, _ = fmt.Fprintf(l.output, "%s\n", msg)
}

// Success logs a success message with green checkmark
func (l *Logger) Success(format string, args ...interface{}) {
	msg := format
	if len(args) > 0 {
		msg = fmt.Sprintf(format, args...)
	}

	l.Progress("✓ %s", msg)
}

// Warning logs a warning with yellow warning icon
func (l *Logger) Warning(format string, args ...interface{}) {
	msg := format
	if len(args) > 0 {
		msg = fmt.Sprintf(format, args...)
	}

	l.Progress("⚠️  %s", msg)
}

// ErrorMsg logs an error message with red X icon
func (l *Logger) ErrorMsg(format string, args ...interface{}) {
	msg := format
	if len(args) > 0 {
		msg = fmt.Sprintf(format, args...)
	}

	l.Progress("❌ %s", msg)
}

// Global logger instance
var defaultLogger = NewDefaultLogger()

// Package-level logging functions for convenience
func Debug(format string, args ...interface{}) {
	defaultLogger.Debug(format, args...)
}

func Info(format string, args ...interface{}) {
	defaultLogger.Info(format, args...)
}

func Warn(format string, args ...interface{}) {
	defaultLogger.Warn(format, args...)
}

func Error(format string, args ...interface{}) {
	defaultLogger.Error(format, args...)
}

func Fatal(format string, args ...interface{}) {
	defaultLogger.Fatal(format, args...)
}

func WithError(err error, format string, args ...interface{}) {
	defaultLogger.WithError(err, format, args...)
}

func Progress(format string, args ...interface{}) {
	defaultLogger.Progress(format, args...)
}

func Success(format string, args ...interface{}) {
	defaultLogger.Success(format, args...)
}

func Warning(format string, args ...interface{}) {
	defaultLogger.Warning(format, args...)
}

func ErrorMsg(format string, args ...interface{}) {
	defaultLogger.ErrorMsg(format, args...)
}

// SetLogLevel sets the global log level
func SetLogLevel(level LogLevel) {
	defaultLogger.SetLevel(level)
}

// GetDefaultLogger returns the default logger instance
func GetDefaultLogger() *Logger {
	return defaultLogger
}
