package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all application configuration
type Config struct {
	// Slack configuration
	SlackBotToken  string `json:"-"` // Hide from JSON serialization for security
	SlackAppToken  string `json:"-"`
	SlackChannelID string `json:"slack_channel_id"`

	// Last.fm configuration
	LastFMAPIKey    string `json:"-"`
	LastFMAPISecret string `json:"-"`

	// Application configuration
	Users           []string      `json:"users"`
	LogLevel        string        `json:"log_level"`
	Port            int           `json:"port"`
	ShutdownTimeout time.Duration `json:"shutdown_timeout"`

	// Performance configuration
	MaxConcurrentRequests int           `json:"max_concurrent_requests"`
	CacheEnabled          bool          `json:"cache_enabled"`
	CacheTTL              time.Duration `json:"cache_ttl"`
	RequestTimeout        time.Duration `json:"request_timeout"`
}

// DefaultUsers represents the default user group
var DefaultUsers = []string{
	"Codeine_turtle", "odesmut", "dudeactually",
	"z47Breezo", "itsalmostdry",
	"v0__", "Hirammj", "FrozenWaterz", "Silkmoney",
	"Mo98t", "BTGKM9_Redd", "colbster411", "FaRiddim", "Vadermaulkylo",
	"Schwarrtz", "Xutros", "Billy-Shakes", "maloboosie", "icy_twat", "junkiesRpeople", "rumnitty",
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	config := &Config{
		// Set defaults
		Users:                 DefaultUsers,
		LogLevel:              getEnv("LOG_LEVEL", "info"),
		Port:                  getEnvInt("PORT", 8080),
		ShutdownTimeout:       getEnvDuration("SHUTDOWN_TIMEOUT", "30s"),
		MaxConcurrentRequests: getEnvInt("MAX_CONCURRENT_REQUESTS", 10),
		CacheEnabled:          getEnvBool("CACHE_ENABLED", true),
		CacheTTL:              getEnvDuration("CACHE_TTL", "5m"),
		RequestTimeout:        getEnvDuration("REQUEST_TIMEOUT", "30s"),
		SlackChannelID:        getEnv("SLACK_CHANNEL_ID", "C0392543PUY"),
	}

	// Required environment variables
	requiredVars := map[string]*string{
		"SLACK_BOT_TOKEN":   &config.SlackBotToken,
		"SLACK_APP_TOKEN":   &config.SlackAppToken,
		"LASTFM_API_KEY":    &config.LastFMAPIKey,
		"LASTFM_API_SECRET": &config.LastFMAPISecret,
	}

	// Check required variables
	for envVar, configField := range requiredVars {
		value := os.Getenv(envVar)
		if value == "" {
			return nil, fmt.Errorf("required environment variable %s is not set", envVar)
		}
		*configField = value
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

// Validate validates the configuration
func (c *Config) Validate() error {
	if len(c.Users) == 0 {
		return fmt.Errorf("at least one user must be configured")
	}

	if c.MaxConcurrentRequests <= 0 {
		return fmt.Errorf("max concurrent requests must be positive")
	}

	if c.CacheTTL <= 0 {
		return fmt.Errorf("cache TTL must be positive")
	}

	// Validate log level
	validLogLevels := []string{"debug", "info", "warn", "error"}
	validLevel := false
	for _, level := range validLogLevels {
		if strings.ToLower(c.LogLevel) == level {
			validLevel = true
			break
		}
	}
	if !validLevel {
		return fmt.Errorf("invalid log level: %s (valid: %v)", c.LogLevel, validLogLevels)
	}

	return nil
}

// GetLogLevel returns the slog.Level for the configured log level
func (c *Config) GetLogLevel() slog.Level {
	switch strings.ToLower(c.LogLevel) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// Helper functions for environment variable parsing
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue string) time.Duration {
	value := getEnv(key, defaultValue)
	if duration, err := time.ParseDuration(value); err == nil {
		return duration
	}
	// If parsing fails, parse the default
	if duration, err := time.ParseDuration(defaultValue); err == nil {
		return duration
	}
	// Fallback to a reasonable default
	return 30 * time.Second
}
