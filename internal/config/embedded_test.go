package config

import (
	"os"
	"testing"
)

func TestLoadEmbedded(t *testing.T) {
	// Save original environment and embedded values
	originalEnv := map[string]string{
		"SLACK_BOT_TOKEN":   os.Getenv("SLACK_BOT_TOKEN"),
		"SLACK_APP_TOKEN":   os.Getenv("SLACK_APP_TOKEN"),
		"LASTFM_API_KEY":    os.Getenv("LASTFM_API_KEY"),
		"LASTFM_API_SECRET": os.Getenv("LASTFM_API_SECRET"),
	}
	originalEmbedded := map[string]string{
		"SlackBot":  EmbeddedSlackBotToken,
		"SlackApp":  EmbeddedSlackAppToken,
		"LastFMKey": EmbeddedLastFMAPIKey,
		"LastFMSec": EmbeddedLastFMSecret,
	}

	// Clear environment variables
	for key := range originalEnv {
		os.Unsetenv(key)
	}

	// Restore everything at the end
	defer func() {
		for key, value := range originalEnv {
			if value != "" {
				os.Setenv(key, value)
			} else {
				os.Unsetenv(key)
			}
		}
		EmbeddedSlackBotToken = originalEmbedded["SlackBot"]
		EmbeddedSlackAppToken = originalEmbedded["SlackApp"]
		EmbeddedLastFMAPIKey = originalEmbedded["LastFMKey"]
		EmbeddedLastFMSecret = originalEmbedded["LastFMSec"]
	}()

	t.Run("embedded keys only", func(t *testing.T) {
		// Set embedded values
		EmbeddedSlackBotToken = "embedded-bot-token"
		EmbeddedSlackAppToken = "embedded-app-token"
		EmbeddedLastFMAPIKey = "embedded-api-key"
		EmbeddedLastFMSecret = "embedded-api-secret"

		config, err := LoadEmbedded()
		if err != nil {
			t.Fatalf("LoadEmbedded failed: %v", err)
		}

		if config.SlackBotToken != "embedded-bot-token" {
			t.Errorf("Expected SlackBotToken 'embedded-bot-token', got '%s'", config.SlackBotToken)
		}
		if config.SlackAppToken != "embedded-app-token" {
			t.Errorf("Expected SlackAppToken 'embedded-app-token', got '%s'", config.SlackAppToken)
		}
		if config.LastFMAPIKey != "embedded-api-key" {
			t.Errorf("Expected LastFMAPIKey 'embedded-api-key', got '%s'", config.LastFMAPIKey)
		}
		if config.LastFMAPISecret != "embedded-api-secret" {
			t.Errorf("Expected LastFMAPISecret 'embedded-api-secret', got '%s'", config.LastFMAPISecret)
		}
	})

	t.Run("environment variables override embedded", func(t *testing.T) {
		// Set embedded values
		EmbeddedSlackBotToken = "embedded-bot-token"
		EmbeddedSlackAppToken = "embedded-app-token"
		EmbeddedLastFMAPIKey = "embedded-api-key"
		EmbeddedLastFMSecret = "embedded-api-secret"

		// Set environment variables (should override embedded)
		os.Setenv("SLACK_BOT_TOKEN", "env-bot-token")
		os.Setenv("LASTFM_API_KEY", "env-api-key")

		config, err := LoadEmbedded()
		if err != nil {
			t.Fatalf("LoadEmbedded failed: %v", err)
		}

		// Environment variables should override embedded
		if config.SlackBotToken != "env-bot-token" {
			t.Errorf("Expected SlackBotToken 'env-bot-token', got '%s'", config.SlackBotToken)
		}
		if config.LastFMAPIKey != "env-api-key" {
			t.Errorf("Expected LastFMAPIKey 'env-api-key', got '%s'", config.LastFMAPIKey)
		}

		// Embedded values should be used where env vars are not set
		if config.SlackAppToken != "embedded-app-token" {
			t.Errorf("Expected SlackAppToken 'embedded-app-token', got '%s'", config.SlackAppToken)
		}
		if config.LastFMAPISecret != "embedded-api-secret" {
			t.Errorf("Expected LastFMAPISecret 'embedded-api-secret', got '%s'", config.LastFMAPISecret)
		}

		// Clean up
		os.Unsetenv("SLACK_BOT_TOKEN")
		os.Unsetenv("LASTFM_API_KEY")
	})

	t.Run("missing required keys should fail", func(t *testing.T) {
		// Clear embedded values
		EmbeddedSlackBotToken = ""
		EmbeddedSlackAppToken = ""
		EmbeddedLastFMAPIKey = ""
		EmbeddedLastFMSecret = ""

		_, err := LoadEmbedded()
		if err == nil {
			t.Error("Expected LoadEmbedded to fail with missing keys")
		}
	})
}

func TestGetEnvWithFallback(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		envValue string
		fallback string
		expected string
	}{
		{
			name:     "environment variable exists",
			key:      "TEST_KEY",
			envValue: "env-value",
			fallback: "fallback-value",
			expected: "env-value",
		},
		{
			name:     "environment variable empty, use fallback",
			key:      "TEST_KEY_EMPTY",
			envValue: "",
			fallback: "fallback-value",
			expected: "fallback-value",
		},
		{
			name:     "environment variable not set, use fallback",
			key:      "TEST_KEY_NOT_SET",
			envValue: "", // will not be set
			fallback: "fallback-value",
			expected: "fallback-value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up
			original := os.Getenv(tt.key)
			defer func() {
				if original != "" {
					os.Setenv(tt.key, original)
				} else {
					os.Unsetenv(tt.key)
				}
			}()

			// Set up test environment
			if tt.envValue != "" {
				os.Setenv(tt.key, tt.envValue)
			} else {
				os.Unsetenv(tt.key)
			}

			result := getEnvWithFallback(tt.key, tt.fallback)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}
