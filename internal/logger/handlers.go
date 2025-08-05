package logger

import (
	"context"
	"fmt"
	"io"
	"log"
	"log/slog"

	"github.com/fatih/color"
)

// PrettyHandler provides colored logging output
type PrettyHandler struct {
	slog.Handler
	logger *log.Logger
}

func (h *PrettyHandler) Handle(ctx context.Context, r slog.Record) error {
	level := r.Level.String() + ":"

	switch r.Level {
	case slog.LevelDebug:
		level = color.MagentaString(level)
	case slog.LevelInfo:
		level = color.BlueString(level)
	case slog.LevelWarn:
		level = color.YellowString(level)
	case slog.LevelError:
		level = color.RedString(level)
	}

	timeStr := r.Time.Format("[2006-01-02 15:04:05.000]")
	msg := color.CyanString(r.Message)

	// Format attributes
	attrs := make([]string, 0, r.NumAttrs())
	r.Attrs(func(a slog.Attr) bool {
		attrs = append(attrs, fmt.Sprintf("%s=%v", a.Key, a.Value.Any()))
		return true
	})

	if len(attrs) > 0 {
		for _, attr := range attrs {
			msg += " " + color.WhiteString(attr)
		}
	}

	h.logger.Println(timeStr, level, msg)
	return nil
}

// NewPrettyHandler creates a new PrettyHandler that outputs colored logs
func NewPrettyHandler(out io.Writer, level slog.Level) *PrettyHandler {
	return &PrettyHandler{
		Handler: slog.NewTextHandler(out, &slog.HandlerOptions{Level: level}),
		logger:  log.New(out, "", 0),
	}
}

// JSONHandler provides JSON logging output
type JSONHandler struct {
	*slog.JSONHandler
}

// NewJSONHandler creates a new JSONHandler that outputs structured JSON logs
func NewJSONHandler(out io.Writer, level slog.Level) *JSONHandler {
	return &JSONHandler{
		JSONHandler: slog.NewJSONHandler(out, &slog.HandlerOptions{
			Level:     level,
			AddSource: false,
		}),
	}
}
