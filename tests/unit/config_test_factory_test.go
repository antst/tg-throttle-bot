// Package unit provides tests for config functionality.
package unit

import (
	"testing"
	"time"

	"github.com/antst/tg-throttle-bot/internal/config"
	"github.com/stretchr/testify/assert"
)

// TestNewTestConfig verifies the test config factory creates configs without validation.
func TestNewTestConfig(t *testing.T) {
	t.Run(
		"creates config with defaults", func(t *testing.T) {
			cfg := config.NewTestConfig()

			assert.Equal(t, "info", cfg.LogLevel)
			assert.Equal(t, "8080", cfg.Port)
			assert.Equal(t, 100, cfg.DefaultCharLimit)
			assert.Equal(t, time.Hour, cfg.DefaultWindow)
		},
	)

	t.Run(
		"creates config with only database URL", func(t *testing.T) {
			cfg := config.NewTestConfig(
				config.WithDatabaseURL("postgres://localhost/testdb"),
			)

			assert.Equal(t, "postgres://localhost/testdb", cfg.DatabaseURL)
			assert.Empty(t, cfg.TelegramToken, "token should remain empty")
		},
	)

	t.Run(
		"creates config with all options", func(t *testing.T) {
			cfg := config.NewTestConfig(
				config.WithTelegramToken("test-token-123"),
				config.WithDatabaseURL("postgres://localhost/testdb"),
				config.WithLogLevel("debug"),
				config.WithDefaultCharLimit(2000),
				config.WithDefaultWindow(24*time.Hour),
				config.WithPort("9090"),
			)

			assert.Equal(t, "test-token-123", cfg.TelegramToken)
			assert.Equal(t, "postgres://localhost/testdb", cfg.DatabaseURL)
			assert.Equal(t, "debug", cfg.LogLevel)
			assert.Equal(t, 2000, cfg.DefaultCharLimit)
			assert.Equal(t, 24*time.Hour, cfg.DefaultWindow)
			assert.Equal(t, "9090", cfg.Port)
		},
	)

	t.Run(
		"options can be applied in any order", func(t *testing.T) {
			cfg := config.NewTestConfig(
				config.WithPort("3000"),
				config.WithDatabaseURL("postgres://db"),
				config.WithLogLevel("error"),
			)

			assert.Equal(t, "3000", cfg.Port)
			assert.Equal(t, "postgres://db", cfg.DatabaseURL)
			assert.Equal(t, "error", cfg.LogLevel)
		},
	)
}
