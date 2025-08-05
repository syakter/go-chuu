package config

import (
	"os"
	"testing"
	"time"

	"github.com/joho/godotenv"
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
			LogFormat:             "pretty",
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
			LogFormat:             "pretty",
			MaxConcurrentRequests: 5,
			CacheTTL:              5 * time.Minute,
		}

		if err := cfg.Validate(); err == nil {
			t.Error("Expected error for invalid log level, got nil")
		}
	})

	t.Run("invalid log format", func(t *testing.T) {
		cfg := &Config{
			Users:                 []string{"user1"},
			LogLevel:              "info",
			LogFormat:             "invalid",
			MaxConcurrentRequests: 5,
			CacheTTL:              5 * time.Minute,
		}

		if err := cfg.Validate(); err == nil {
			t.Error("Expected error for invalid log format, got nil")
		}
	})

	t.Run("empty users", func(t *testing.T) {
		cfg := &Config{
			Users:                 []string{},
			LogLevel:              "info",
			LogFormat:             "pretty",
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

func TestGetLogFormat(t *testing.T) {
	tests := []struct {
		format   string
		expected string
	}{
		{"pretty", "pretty"},
		{"PRETTY", "pretty"},
		{"json", "json"},
		{"JSON", "json"},
		{"Mixed", "mixed"},
	}

	for _, test := range tests {
		t.Run(test.format, func(t *testing.T) {
			cfg := &Config{LogFormat: test.format}
			format := cfg.GetLogFormat()

			if format != test.expected {
				t.Errorf("Expected %s, got %s", test.expected, format)
			}
		})
	}
}

func TestLoadWithDotEnv(t *testing.T) {
	// Save original environment
	originalSlackBot := os.Getenv("SLACK_BOT_TOKEN")
	originalSlackApp := os.Getenv("SLACK_APP_TOKEN")
	originalLastFMKey := os.Getenv("LASTFM_API_KEY")
	originalLastFMSecret := os.Getenv("LASTFM_API_SECRET")

	// Clear environment variables
	os.Unsetenv("SLACK_BOT_TOKEN")
	os.Unsetenv("SLACK_APP_TOKEN")
	os.Unsetenv("LASTFM_API_KEY")
	os.Unsetenv("LASTFM_API_SECRET")

	// Restore environment at the end
	defer func() {
		if originalSlackBot != "" {
			os.Setenv("SLACK_BOT_TOKEN", originalSlackBot)
		}
		if originalSlackApp != "" {
			os.Setenv("SLACK_APP_TOKEN", originalSlackApp)
		}
		if originalLastFMKey != "" {
			os.Setenv("LASTFM_API_KEY", originalLastFMKey)
		}
		if originalLastFMSecret != "" {
			os.Setenv("LASTFM_API_SECRET", originalLastFMSecret)
		}
	}()

	// Create a test .env content
	testEnv := map[string]string{
		"SLACK_BOT_TOKEN":   "xoxb-test-token",
		"SLACK_APP_TOKEN":   "xapp-test-token",
		"LASTFM_API_KEY":    "test-api-key",
		"LASTFM_API_SECRET": "test-api-secret",
	}

	// Set environment variables manually (simulating .env loading)
	for key, value := range testEnv {
		os.Setenv(key, value)
	}

	// Test loading configuration
	config, err := Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify tokens are loaded correctly
	if config.SlackBotToken != "xoxb-test-token" {
		t.Errorf("Expected SlackBotToken 'xoxb-test-token', got '%s'", config.SlackBotToken)
	}

	if config.SlackAppToken != "xapp-test-token" {
		t.Errorf("Expected SlackAppToken 'xapp-test-token', got '%s'", config.SlackAppToken)
	}

	if config.LastFMAPIKey != "test-api-key" {
		t.Errorf("Expected LastFMAPIKey 'test-api-key', got '%s'", config.LastFMAPIKey)
	}

	if config.LastFMAPISecret != "test-api-secret" {
		t.Errorf("Expected LastFMAPISecret 'test-api-secret', got '%s'", config.LastFMAPISecret)
	}
}

func TestDefaultUsersCount(t *testing.T) {
	t.Run("default users list has 24 users", func(t *testing.T) {
		expectedCount := 24
		actualCount := len(DefaultUsers)

		if actualCount != expectedCount {
			t.Errorf("Expected DefaultUsers to contain %d users, but got %d users", expectedCount, actualCount)
		}

		// Verify no empty usernames
		for i, user := range DefaultUsers {
			if user == "" {
				t.Errorf("DefaultUsers[%d] is empty", i)
			}
		}

		// Verify no duplicate usernames
		userSet := make(map[string]bool)
		for i, user := range DefaultUsers {
			if userSet[user] {
				t.Errorf("DefaultUsers[%d] = %q is a duplicate", i, user)
			}
			userSet[user] = true
		}
	})
}

func TestGodotenvIntegration(t *testing.T) {
	// Test that godotenv can load from a string (simulating .env file)
	envContent := `SLACK_BOT_TOKEN=xoxb-from-dotenv
SLACK_APP_TOKEN=xapp-from-dotenv
LASTFM_API_KEY=key-from-dotenv
LASTFM_API_SECRET=secret-from-dotenv`

	// Parse the content
	envMap, err := godotenv.Unmarshal(envContent)
	if err != nil {
		t.Fatalf("Failed to parse env content: %v", err)
	}

	// Verify parsing worked
	expected := map[string]string{
		"SLACK_BOT_TOKEN":   "xoxb-from-dotenv",
		"SLACK_APP_TOKEN":   "xapp-from-dotenv",
		"LASTFM_API_KEY":    "key-from-dotenv",
		"LASTFM_API_SECRET": "secret-from-dotenv",
	}

	for key, expectedValue := range expected {
		if envMap[key] != expectedValue {
			t.Errorf("Expected %s='%s', got '%s'", key, expectedValue, envMap[key])
		}
	}
}
