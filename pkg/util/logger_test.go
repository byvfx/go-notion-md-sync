package util

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestLogLevel_String(t *testing.T) {
	tests := []struct {
		level    LogLevel
		expected string
	}{
		{DEBUG, "DEBUG"},
		{INFO, "INFO"},
		{WARN, "WARN"},
		{ERROR, "ERROR"},
		{FATAL, "FATAL"},
		{LogLevel(999), "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.level.String(); got != tt.expected {
				t.Errorf("LogLevel.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestNewLogger(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(INFO, &buf)

	if logger == nil {
		t.Fatal("NewLogger() returned nil")
	}

	if logger.level != INFO {
		t.Errorf("Expected level INFO, got %v", logger.level)
	}

	if logger.output != &buf {
		t.Error("Logger output not set correctly")
	}
}

func TestLogger_SetLevel(t *testing.T) {
	logger := NewDefaultLogger()

	logger.SetLevel(DEBUG)
	if logger.GetLevel() != DEBUG {
		t.Errorf("Expected DEBUG level, got %v", logger.GetLevel())
	}

	logger.SetLevel(ERROR)
	if logger.GetLevel() != ERROR {
		t.Errorf("Expected ERROR level, got %v", logger.GetLevel())
	}
}

func TestLogger_ShouldLog(t *testing.T) {
	tests := []struct {
		loggerLevel LogLevel
		msgLevel    LogLevel
		shouldLog   bool
	}{
		{DEBUG, DEBUG, true},
		{DEBUG, INFO, true},
		{DEBUG, ERROR, true},
		{INFO, DEBUG, false},
		{INFO, INFO, true},
		{INFO, ERROR, true},
		{ERROR, DEBUG, false},
		{ERROR, INFO, false},
		{ERROR, ERROR, true},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			logger := NewLogger(tt.loggerLevel, &bytes.Buffer{})
			if got := logger.shouldLog(tt.msgLevel); got != tt.shouldLog {
				t.Errorf("shouldLog(%v) with level %v = %v, want %v",
					tt.msgLevel, tt.loggerLevel, got, tt.shouldLog)
			}
		})
	}
}

func TestLogger_Debug(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(DEBUG, &buf)

	logger.Debug("test debug message")

	output := buf.String()
	if !strings.Contains(output, "DEBUG") {
		t.Error("Debug message should contain DEBUG level")
	}
	if !strings.Contains(output, "test debug message") {
		t.Error("Debug message should contain the message")
	}
}

func TestLogger_Info(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(INFO, &buf)

	logger.Info("test info message with %s", "formatting")

	output := buf.String()
	if !strings.Contains(output, "INFO") {
		t.Error("Info message should contain INFO level")
	}
	if !strings.Contains(output, "test info message with formatting") {
		t.Error("Info message should contain the formatted message")
	}
}

func TestLogger_Warn(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(WARN, &buf)

	logger.Warn("test warning")

	output := buf.String()
	if !strings.Contains(output, "WARN") {
		t.Error("Warning message should contain WARN level")
	}
	if !strings.Contains(output, "test warning") {
		t.Error("Warning message should contain the message")
	}
}

func TestLogger_Error(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(ERROR, &buf)

	logger.Error("test error")

	output := buf.String()
	if !strings.Contains(output, "ERROR") {
		t.Error("Error message should contain ERROR level")
	}
	if !strings.Contains(output, "test error") {
		t.Error("Error message should contain the message")
	}
}

func TestLogger_WithError(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(ERROR, &buf)

	err := errors.New("test error")
	logger.WithError(err, "operation failed")

	output := buf.String()
	if !strings.Contains(output, "ERROR") {
		t.Error("WithError should contain ERROR level")
	}
	if !strings.Contains(output, "operation failed") {
		t.Error("WithError should contain the message")
	}
	if !strings.Contains(output, "test error") {
		t.Error("WithError should contain the error details")
	}
}

func TestLogger_WithError_NilError(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(ERROR, &buf)

	logger.WithError(nil, "operation completed")

	output := buf.String()
	if !strings.Contains(output, "operation completed") {
		t.Error("WithError with nil should still log the message")
	}
}

func TestLogger_Progress(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(ERROR, &buf) // Set to ERROR to test that Progress ignores level

	logger.Progress("progress message")

	output := buf.String()
	if !strings.Contains(output, "progress message") {
		t.Error("Progress message should be shown regardless of log level")
	}
	// Progress messages shouldn't contain level or timestamp
	if strings.Contains(output, "ERROR") || strings.Contains(output, "[") {
		t.Error("Progress messages should be simple format")
	}
}

func TestLogger_Success(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(ERROR, &buf)

	logger.Success("operation completed")

	output := buf.String()
	if !strings.Contains(output, "✓") {
		t.Error("Success message should contain checkmark")
	}
	if !strings.Contains(output, "operation completed") {
		t.Error("Success message should contain the message")
	}
}

func TestLogger_Warning(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(ERROR, &buf)

	logger.Warning("warning message")

	output := buf.String()
	if !strings.Contains(output, "⚠️") {
		t.Error("Warning message should contain warning icon")
	}
	if !strings.Contains(output, "warning message") {
		t.Error("Warning message should contain the message")
	}
}

func TestLogger_ErrorMsg(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(ERROR, &buf)

	logger.ErrorMsg("error message")

	output := buf.String()
	if !strings.Contains(output, "❌") {
		t.Error("Error message should contain X icon")
	}
	if !strings.Contains(output, "error message") {
		t.Error("Error message should contain the message")
	}
}

func TestLogger_LevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(WARN, &buf)

	// These should not be logged
	logger.Debug("debug message")
	logger.Info("info message")

	// These should be logged
	logger.Warn("warning message")
	logger.Error("error message")

	output := buf.String()
	if strings.Contains(output, "debug message") {
		t.Error("Debug messages should be filtered out")
	}
	if strings.Contains(output, "info message") {
		t.Error("Info messages should be filtered out")
	}
	if !strings.Contains(output, "warning message") {
		t.Error("Warning messages should be logged")
	}
	if !strings.Contains(output, "error message") {
		t.Error("Error messages should be logged")
	}
}

func TestPackageLevelFunctions(t *testing.T) {
	// Test that package-level functions work
	// We can't easily test output without changing global state,
	// but we can at least verify they don't panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Package-level function panicked: %v", r)
		}
	}()

	Debug("test debug")
	Info("test info")
	Warn("test warn")
	Error("test error")
	WithError(errors.New("test"), "test with error")
	Progress("test progress")
	Success("test success")
	Warning("test warning")
	ErrorMsg("test error msg")

	SetLogLevel(DEBUG)
	if GetDefaultLogger().GetLevel() != DEBUG {
		t.Error("SetLogLevel didn't work")
	}
}
