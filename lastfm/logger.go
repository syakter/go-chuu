package lastfm

import (
	"log/slog"
	"os"
)

var logger *slog.Logger

func init() {
	// Default to info level
	opts := &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}

	// Check for debug mode
	if os.Getenv("LASTFM_DEBUG") == "true" {
		opts.Level = slog.LevelDebug
	}

	logger = slog.New(slog.NewJSONHandler(os.Stdout, opts))
}
