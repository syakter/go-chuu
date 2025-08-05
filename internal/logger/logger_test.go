package logger

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
)

func TestParseFormat(t *testing.T) {
	tests := []struct {
		input    string
		expected Format
	}{
		{"json", FormatJSON},
		{"JSON", FormatJSON},
		{"pretty", FormatPretty},
		{"PRETTY", FormatPretty},
		{"invalid", FormatPretty}, // defaults to pretty
		{"", FormatPretty},        // defaults to pretty
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ParseFormat(tt.input)
			if result != tt.expected {
				t.Errorf("ParseFormat(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestPrettyHandlerOutput(t *testing.T) {
	var buf bytes.Buffer
	logger := New(Config{
		Format: FormatPretty,
		Level:  slog.LevelDebug,
		Output: &buf,
	})

	logger.Info("test message", "key", "value")

	output := buf.String()
	// Check that it contains expected elements
	if !strings.Contains(output, "test message") {
		t.Errorf("Output should contain 'test message', got: %s", output)
	}
	if !strings.Contains(output, "key=value") {
		t.Errorf("Output should contain 'key=value', got: %s", output)
	}
	if !strings.Contains(output, "INFO:") {
		t.Errorf("Output should contain 'INFO:', got: %s", output)
	}
	// Check for timestamp format
	if !strings.Contains(output, "[") || !strings.Contains(output, "]") {
		t.Errorf("Output should contain timestamp in brackets, got: %s", output)
	}
}

func TestJSONHandlerOutput(t *testing.T) {
	var buf bytes.Buffer
	logger := New(Config{
		Format: FormatJSON,
		Level:  slog.LevelDebug,
		Output: &buf,
	})

	logger.Info("test message", "key", "value", "number", 42)

	output := strings.TrimSpace(buf.String())

	// Parse as JSON to validate structure
	var logEntry map[string]interface{}
	if err := json.Unmarshal([]byte(output), &logEntry); err != nil {
		t.Fatalf("Output should be valid JSON, got error: %v, output: %s", err, output)
	}

	// Check required fields
	if logEntry["msg"] != "test message" {
		t.Errorf("Expected msg='test message', got: %v", logEntry["msg"])
	}
	if logEntry["level"] != "INFO" {
		t.Errorf("Expected level='INFO', got: %v", logEntry["level"])
	}
	if logEntry["key"] != "value" {
		t.Errorf("Expected key='value', got: %v", logEntry["key"])
	}
	if logEntry["number"] != float64(42) { // JSON numbers are float64
		t.Errorf("Expected number=42, got: %v", logEntry["number"])
	}
	if logEntry["time"] == nil {
		t.Errorf("Expected time field to be present")
	}
}

func TestLoggerLevels(t *testing.T) {
	tests := []struct {
		level       slog.Level
		logLevel    slog.Level
		shouldLog   bool
		description string
	}{
		{slog.LevelDebug, slog.LevelDebug, true, "debug message at debug level"},
		{slog.LevelInfo, slog.LevelDebug, true, "info message at debug level"},
		{slog.LevelWarn, slog.LevelDebug, true, "warn message at debug level"},
		{slog.LevelError, slog.LevelDebug, true, "error message at debug level"},
		{slog.LevelDebug, slog.LevelInfo, false, "debug message at info level"},
		{slog.LevelInfo, slog.LevelInfo, true, "info message at info level"},
		{slog.LevelDebug, slog.LevelError, false, "debug message at error level"},
		{slog.LevelError, slog.LevelError, true, "error message at error level"},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			var buf bytes.Buffer
			logger := New(Config{
				Format: FormatJSON, // Use JSON for easier parsing
				Level:  tt.logLevel,
				Output: &buf,
			})

			// Log at the specified level
			switch tt.level {
			case slog.LevelDebug:
				logger.Debug("test message")
			case slog.LevelInfo:
				logger.Info("test message")
			case slog.LevelWarn:
				logger.Warn("test message")
			case slog.LevelError:
				logger.Error("test message")
			}

			output := strings.TrimSpace(buf.String())
			hasOutput := len(output) > 0

			if hasOutput != tt.shouldLog {
				t.Errorf("%s: expected shouldLog=%v, got hasOutput=%v, output: %s",
					tt.description, tt.shouldLog, hasOutput, output)
			}
		})
	}
}
