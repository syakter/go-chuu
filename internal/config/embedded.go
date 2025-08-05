package config

import (
	"fmt"
	"os"
	"strings"
)

// Embedded API keys set at build time
// These will be populated via -ldflags during build
var (
	EmbeddedSlackBotToken  string
	EmbeddedSlackAppToken  string
	EmbeddedLastFMAPIKey   string
	EmbeddedLastFMSecret   string
	EmbeddedSlackChannelID string
)

// LoadEmbedded loads configuration with embedded keys as fallback
func LoadEmbedded() (*Config, error) {
	config := &Config{
		// Set defaults
		Users:                 DefaultUsers,
		LogLevel:              getEnv("LOG_LEVEL", "info"),
		LogFormat:             getEnv("LOG_FORMAT", "pretty"),
		Port:                  getEnvInt("PORT", 8080),
		ShutdownTimeout:       getEnvDuration("SHUTDOWN_TIMEOUT", "30s"),
		MaxConcurrentRequests: getEnvInt("MAX_CONCURRENT_REQUESTS", 10),
		CacheEnabled:          getEnvBool("CACHE_ENABLED", true),
		CacheTTL:              getEnvDuration("CACHE_TTL", "5m"),
		RequestTimeout:        getEnvDuration("REQUEST_TIMEOUT", "30s"),
	}

	// Load API tokens with embedded fallback
	config.SlackBotToken = getEnvWithFallback("SLACK_BOT_TOKEN", EmbeddedSlackBotToken)
	config.SlackAppToken = getEnvWithFallback("SLACK_APP_TOKEN", EmbeddedSlackAppToken)
	config.LastFMAPIKey = getEnvWithFallback("LASTFM_API_KEY", EmbeddedLastFMAPIKey)
	config.LastFMAPISecret = getEnvWithFallback("LASTFM_API_SECRET", EmbeddedLastFMSecret)
	config.SlackChannelID = getEnvWithFallback("SLACK_CHANNEL_ID", EmbeddedSlackChannelID)

	// Use default channel if neither env var nor embedded value is set
	if config.SlackChannelID == "" {
		config.SlackChannelID = "C0392543PUY"
	}

	// Validate required tokens
	if config.SlackBotToken == "" {
		return nil, fmt.Errorf("SLACK_BOT_TOKEN is required (set via environment variable or embedded at build time)")
	}
	if config.SlackAppToken == "" {
		return nil, fmt.Errorf("SLACK_APP_TOKEN is required (set via environment variable or embedded at build time)")
	}
	if config.LastFMAPIKey == "" {
		return nil, fmt.Errorf("LASTFM_API_KEY is required (set via environment variable or embedded at build time)")
	}
	if config.LastFMAPISecret == "" {
		return nil, fmt.Errorf("LASTFM_API_SECRET is required (set via environment variable or embedded at build time)")
	}

	// Optional user list override
	if usersList := os.Getenv("USERS"); usersList != "" {
		config.Users = strings.Split(usersList, ",")
		// Trim whitespace from each user
		for i, user := range config.Users {
			config.Users[i] = strings.TrimSpace(user)
		}
	}

	return config, nil
}

// getEnvWithFallback returns environment variable value or fallback if empty
func getEnvWithFallback(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
