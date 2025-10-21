// Package config provides configuration loading and validation for the Throttle Bot application.
// It reads environment variables and provides sensible defaults.
package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

// Config holds the application configuration loaded from environment variables.
type Config struct {
	TelegramToken string
	DatabaseURL   string
	LogLevel      string
	Port          string
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
