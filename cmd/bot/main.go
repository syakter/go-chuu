package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/joho/godotenv"
	"github.com/syakter/go-chuu/internal/buildinfo"
	"github.com/syakter/go-chuu/internal/cache"
	"github.com/syakter/go-chuu/internal/charts"
	"github.com/syakter/go-chuu/internal/commands"
	"github.com/syakter/go-chuu/internal/config"
	"github.com/syakter/go-chuu/internal/lastfm"
	"github.com/syakter/go-chuu/internal/logger"
	"github.com/syakter/go-chuu/internal/slack"
)

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

	// Try loading with embedded keys first, fallback to regular loading
	cfg, err := config.LoadEmbedded()
	if err != nil {
		// If embedded loading fails, try regular loading for backwards compatibility
		cfg, err = config.Load()
	}
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// Setup logging
	logger := logger.New(logger.Config{
		Format: logger.ParseFormat(cfg.GetLogFormat()),
		Level:  cfg.GetLogLevel(),
		Output: os.Stdout,
	})
	slog.SetDefault(logger)

	// Log build information at startup
	buildInfo := buildinfo.Get()
	logger.Info("Starting go-chuu bot",
		"version", buildInfo.Version,
		"git_commit", buildInfo.GitCommit,
		"git_branch", buildInfo.GitBranch,
		"build_time", buildInfo.BuildTime,
		"go_version", buildInfo.GoVersion,
		"go_os", buildInfo.GoOS,
		"go_arch", buildInfo.GoArch,
		"log_level", cfg.LogLevel)

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

	// Create Slack handler
	slackHandler := slack.NewHandler(cfg, lastfmClient, chartGen, parser, logger)

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	// Use os.Interrupt for cross-platform compatibility (Ctrl+C on Windows, SIGINT on Unix)
	// Note: SIGTERM is not available on Windows, so we avoid it for better portability
	signal.Notify(sigChan, os.Interrupt)

	go func() {
		sig := <-sigChan
		logger.Info("Received shutdown signal", "signal", sig)
		cancel()
	}()

	// Start the bot
	errorChan := make(chan error, 1)
	go func() {
		if err := slackHandler.Start(ctx); err != nil {
			errorChan <- fmt.Errorf("slack handler error: %w", err)
		}
	}()

	// Wait for shutdown or error
	select {
	case err := <-errorChan:
		logger.Error("Bot stopped with error", "error", err)
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
