// Package config provides configuration loading and validation for the Throttle Bot application.
// It reads environment variables and provides sensible defaults.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

// Config holds the application configuration loaded from environment variables.
type Config struct {
	TelegramToken    string
	DatabaseURL      string
	LogLevel         string
	DefaultCharLimit int
	DefaultWindow    time.Duration
	Port             string
}

// Load reads configuration from environment variables and returns a validated Config.
// It loads .env file if present and validates required fields.
func Load() (*Config, error) {
	// Load .env file if it exists
	_ = godotenv.Load()

	cfg := &Config{
		TelegramToken: os.Getenv("TELEGRAM_TOKEN"),
		DatabaseURL:   os.Getenv("DATABASE_URL"),
		LogLevel:      getEnvOrDefault("LOG_LEVEL", "info"),
		Port:          getEnvOrDefault("PORT", "8080"),
	}

	// Parse char limit
	charLimit, err := strconv.Atoi(getEnvOrDefault("DEFAULT_CHAR_LIMIT", "100"))
	if err != nil {
		return nil, fmt.Errorf("invalid DEFAULT_CHAR_LIMIT: %w", err)
	}
	cfg.DefaultCharLimit = charLimit

	// Parse window duration
	windowStr := getEnvOrDefault("DEFAULT_WINDOW", "1h")
	window, err := time.ParseDuration(windowStr)
	if err != nil {
		return nil, fmt.Errorf("invalid DEFAULT_WINDOW: %w", err)
	}
	cfg.DefaultWindow = window

	// Validate required fields
	if cfg.TelegramToken == "" {
		return nil, fmt.Errorf("TELEGRAM_TOKEN is required")
	}
	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	return cfg, nil
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
