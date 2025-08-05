package logger

import (
	"io"
	"log/slog"
	"os"
	"strings"
)

// Format represents the logging format type
type Format string

const (
	// FormatPretty outputs colored, human-readable logs
	FormatPretty Format = "pretty"
	// FormatJSON outputs structured JSON logs
	FormatJSON Format = "json"
)

// Config holds logger configuration
type Config struct {
	Format Format
	Level  slog.Level
	Output io.Writer
}

// New creates a new slog.Logger with the specified configuration
func New(cfg Config) *slog.Logger {
	if cfg.Output == nil {
		cfg.Output = os.Stdout
	}

	var handler slog.Handler
	switch cfg.Format {
	case FormatJSON:
		handler = NewJSONHandler(cfg.Output, cfg.Level)
	case FormatPretty:
		fallthrough
	default:
		handler = NewPrettyHandler(cfg.Output, cfg.Level)
	}

	return slog.New(handler)
}

// ParseFormat parses a string into a Format type
func ParseFormat(s string) Format {
	switch strings.ToLower(s) {
	case "json":
		return FormatJSON
	case "pretty":
		return FormatPretty
	default:
		return FormatPretty
	}
}
