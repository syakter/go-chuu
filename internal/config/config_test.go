package config

import (
	"os"
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
	// Save original env vars
	originalVars := map[string]string{
		"SLACK_BOT_TOKEN":   os.Getenv("SLACK_BOT_TOKEN"),
		"SLACK_APP_TOKEN":   os.Getenv("SLACK_APP_TOKEN"),
		"LASTFM_API_KEY":    os.Getenv("LASTFM_API_KEY"),
		"LASTFM_API_SECRET": os.Getenv("LASTFM_API_SECRET"),
		"LOG_LEVEL":         os.Getenv("LOG_LEVEL"),
		"USERS":             os.Getenv("USERS"),
	}

	// Cleanup function
	defer func() {
		for key, value := range originalVars {
			if value == "" {
				os.Unsetenv(key)
			} else {
				os.Setenv(key, value)
			}
		}
	}()

	t.Run("success with all env vars", func(t *testing.T) {
		os.Setenv("SLACK_BOT_TOKEN", "test-bot-token")
		os.Setenv("SLACK_APP_TOKEN", "test-app-token")
		os.Setenv("LASTFM_API_KEY", "test-api-key")
		os.Setenv("LASTFM_API_SECRET", "test-api-secret")
		os.Setenv("LOG_LEVEL", "debug")
		os.Setenv("USERS", "user1,user2,user3")

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if cfg.SlackBotToken != "test-bot-token" {
			t.Errorf("Expected SlackBotToken 'test-bot-token', got '%s'", cfg.SlackBotToken)
		}

		if len(cfg.Users) != 3 {
			t.Errorf("Expected 3 users, got %d", len(cfg.Users))
		}

		if cfg.LogLevel != "debug" {
			t.Errorf("Expected LogLevel 'debug', got '%s'", cfg.LogLevel)
		}
	})

	t.Run("failure with missing required env var", func(t *testing.T) {
		os.Unsetenv("SLACK_BOT_TOKEN")

		_, err := Load()
		if err == nil {
			t.Fatal("Expected error for missing SLACK_BOT_TOKEN, got nil")
		}
	})

	t.Run("defaults when optional vars missing", func(t *testing.T) {
		os.Setenv("SLACK_BOT_TOKEN", "test-bot-token")
		os.Setenv("SLACK_APP_TOKEN", "test-app-token")
		os.Setenv("LASTFM_API_KEY", "test-api-key")
		os.Setenv("LASTFM_API_SECRET", "test-api-secret")
		os.Unsetenv("LOG_LEVEL")
		os.Unsetenv("USERS")

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if cfg.LogLevel != "info" {
			t.Errorf("Expected default LogLevel 'info', got '%s'", cfg.LogLevel)
		}

		if len(cfg.Users) != len(DefaultUsers) {
			t.Errorf("Expected default users count %d, got %d", len(DefaultUsers), len(cfg.Users))
		}
	})
}

func TestValidate(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		cfg := &Config{
			Users:                 []string{"user1", "user2"},
			LogLevel:              "info",
			MaxConcurrentRequests: 5,
			CacheTTL:              5 * time.Minute,
		}

		if err := cfg.Validate(); err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
	})

	t.Run("invalid log level", func(t *testing.T) {
		cfg := &Config{
			Users:                 []string{"user1"},
			LogLevel:              "invalid",
			MaxConcurrentRequests: 5,
			CacheTTL:              5 * time.Minute,
		}

		if err := cfg.Validate(); err == nil {
			t.Error("Expected error for invalid log level, got nil")
		}
	})

	t.Run("empty users", func(t *testing.T) {
		cfg := &Config{
			Users:                 []string{},
			LogLevel:              "info",
			MaxConcurrentRequests: 5,
			CacheTTL:              5 * time.Minute,
		}

		if err := cfg.Validate(); err == nil {
			t.Error("Expected error for empty users, got nil")
		}
	})
}

func TestGetLogLevel(t *testing.T) {
	tests := []struct {
		level    string
		expected string
	}{
		{"debug", "DEBUG"},
		{"info", "INFO"},
		{"warn", "WARN"},
		{"error", "ERROR"},
		{"invalid", "INFO"}, // should default to INFO
	}

	for _, test := range tests {
		t.Run(test.level, func(t *testing.T) {
			cfg := &Config{LogLevel: test.level}
			level := cfg.GetLogLevel()

			if level.String() != test.expected {
				t.Errorf("Expected %s, got %s", test.expected, level.String())
			}
		})
	}
}
