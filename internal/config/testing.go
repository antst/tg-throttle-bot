// Package config provides configuration loading and validation for the Throttle Bot application.
package config

import (
	"time"
)

// TestConfigOption is a function that configures a test Config.
type TestConfigOption func(*Config)

// NewTestConfig creates a Config for testing with only the specified options.
// This allows tests to create configs with only the fields they need,
// without triggering validation errors for missing fields.
//
// Example:
//
//	cfg := NewTestConfig(
//	  WithDatabaseURL("postgres://localhost/test"),
//	  WithDefaultCharLimit(1000),
//	)
func NewTestConfig(opts ...TestConfigOption) *Config {
	cfg := &Config{
		// Set reasonable defaults for tests
		LogLevel:         "info",
		Port:             "8080",
		DefaultCharLimit: 100,
		DefaultWindow:    time.Hour,
	}

	for _, opt := range opts {
		opt(cfg)
	}

	return cfg
}

// WithTelegramToken sets the Telegram token for test config.
func WithTelegramToken(token string) TestConfigOption {
	return func(cfg *Config) {
		cfg.TelegramToken = token
	}
}

// WithDatabaseURL sets the database URL for test config.
func WithDatabaseURL(url string) TestConfigOption {
	return func(cfg *Config) {
		cfg.DatabaseURL = url
	}
}

// WithLogLevel sets the log level for test config.
func WithLogLevel(level string) TestConfigOption {
	return func(cfg *Config) {
		cfg.LogLevel = level
	}
}

// WithDefaultCharLimit sets the default character limit for test config.
func WithDefaultCharLimit(limit int) TestConfigOption {
	return func(cfg *Config) {
		cfg.DefaultCharLimit = limit
	}
}

// WithDefaultWindow sets the default time window for test config.
func WithDefaultWindow(window time.Duration) TestConfigOption {
	return func(cfg *Config) {
		cfg.DefaultWindow = window
	}
}

// WithPort sets the port for test config.
func WithPort(port string) TestConfigOption {
	return func(cfg *Config) {
		cfg.Port = port
	}
}
