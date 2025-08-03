package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/joho/godotenv"
	"github.com/syakter/go-chuu/internal/cache"
	"github.com/syakter/go-chuu/internal/charts"
	"github.com/syakter/go-chuu/internal/commands"
	"github.com/syakter/go-chuu/internal/config"
	"github.com/syakter/go-chuu/internal/discord"
	"github.com/syakter/go-chuu/internal/lastfm"
	"github.com/syakter/go-chuu/internal/platform"
	"github.com/syakter/go-chuu/internal/slack"
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

	timeStr := r.Time.Format("[15:05:05.000]")
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

func NewPrettyHandler(out io.Writer, level slog.Level) *PrettyHandler {
	return &PrettyHandler{
		Handler: slog.NewTextHandler(out, &slog.HandlerOptions{Level: level}),
		logger:  log.New(out, "", 0),
	}
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Load .env file if it exists (ignore error if file doesn't exist)
	if err := godotenv.Load(); err != nil {
		// Only log debug message, don't fail - environment variables might be set directly
		if !os.IsNotExist(err) {
			fmt.Printf("Warning: error loading .env file: %v\n", err)
		}
	}

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// Setup logging
	handler := NewPrettyHandler(os.Stdout, cfg.GetLogLevel())
	logger := slog.New(handler)
	slog.SetDefault(logger)

	logger.Info("Starting go-chuu bot", "version", "2.0.0", "log_level", cfg.LogLevel)

	// Create cache
	var botCache cache.Cache
	if cfg.CacheEnabled {
		botCache = cache.NewInMemoryCache(1000) // Max 1000 entries
		logger.Info("Cache enabled", "max_entries", 1000, "ttl", cfg.CacheTTL)
	} else {
		botCache = cache.NewInMemoryCache(0) // Disabled cache
		logger.Info("Cache disabled")
	}

	// Create Last.fm client
	lastfmClient := lastfm.NewClient(cfg, botCache, logger)

	// Create charts generator
	tempDir := filepath.Join(os.TempDir(), "go-chuu-charts")
	chartGen := charts.NewGenerator(logger, tempDir, lastfmClient.GetAPI())
	if err := chartGen.EnsureTempDir(); err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}

	// Create command parser
	parser := commands.NewParser(cfg.Users)

	// Create platform handlers
	var handlers []platform.Handler

	if cfg.EnableSlack {
		slackHandler := slack.NewHandler(cfg, lastfmClient, chartGen, parser, logger)
		handlers = append(handlers, slackHandler)
		logger.Info("Slack handler enabled")
	}

	if cfg.EnableDiscord {
		discordHandler, err := discord.NewHandler(cfg, lastfmClient, chartGen, parser, logger)
		if err != nil {
			return fmt.Errorf("failed to create Discord handler: %w", err)
		}
		handlers = append(handlers, discordHandler)
		logger.Info("Discord handler enabled")
	}

	if len(handlers) == 0 {
		return fmt.Errorf("no platform handlers enabled")
	}

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		logger.Info("Received shutdown signal", "signal", sig)
		cancel()
	}()

	// Start the platform handlers
	errorChan := make(chan error, len(handlers))
	for _, handler := range handlers {
		go func(h platform.Handler) {
			if err := h.Start(ctx); err != nil {
				errorChan <- fmt.Errorf("%s handler error: %w", h.GetPlatformType(), err)
			}
		}(handler)
	}

	// Wait for shutdown or error
	select {
	case err := <-errorChan:
		logger.Error("Handler stopped with error", "error", err)
		return err
	case <-ctx.Done():
		logger.Info("Bot shutting down gracefully")

		// Give components time to shutdown
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer shutdownCancel()

		// Wait for shutdown or timeout
		select {
		case <-shutdownCtx.Done():
			logger.Warn("Shutdown timeout exceeded")
		case <-time.After(100 * time.Millisecond):
			// Small delay to allow final operations
		}

		logger.Info("Bot shutdown complete")
		return nil
	}
}
