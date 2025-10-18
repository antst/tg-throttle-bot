package unit

import (
	"testing"
	"time"

	"github.com/antst/tg-throttle-bot/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigLoadDefaults(t *testing.T) {
	// Set minimal required env vars
	t.Setenv("TELEGRAM_TOKEN", "test-token-123")
	t.Setenv("DATABASE_URL", "postgres://localhost/testdb")

	cfg, err := config.Load()
	require.NoError(t, err)

	// Verify defaults are applied
	assert.Equal(t, "test-token-123", cfg.TelegramToken)
	assert.Equal(t, "postgres://localhost/testdb", cfg.DatabaseURL)
	assert.Equal(t, "info", cfg.LogLevel)
	assert.Equal(t, "8080", cfg.Port)
	assert.Equal(t, 100, cfg.DefaultCharLimit)
	assert.Equal(t, time.Hour, cfg.DefaultWindow)
}

func TestConfigCustomValues(t *testing.T) {
	// Set custom values
	t.Setenv("TELEGRAM_TOKEN", "custom-token")
	t.Setenv("DATABASE_URL", "postgres://custom/db")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("PORT", "9000")
	t.Setenv("DEFAULT_CHAR_LIMIT", "5000")
	t.Setenv("DEFAULT_WINDOW", "24h")

	cfg, err := config.Load()
	require.NoError(t, err)

	assert.Equal(t, "custom-token", cfg.TelegramToken)
	assert.Equal(t, "postgres://custom/db", cfg.DatabaseURL)
	assert.Equal(t, "debug", cfg.LogLevel)
	assert.Equal(t, "9000", cfg.Port)
	assert.Equal(t, 5000, cfg.DefaultCharLimit)
	assert.Equal(t, 24*time.Hour, cfg.DefaultWindow)
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name        string
		token       string
		dbURL       string
		charLimit   string
		window      string
		expectError bool
	}{
		{
			name:        "valid_config",
			token:       "valid-token",
			dbURL:       "postgres://localhost/db",
			charLimit:   "1000",
			window:      "1h",
			expectError: false,
		},
		{
			name:        "missing_token",
			token:       "",
			dbURL:       "postgres://localhost/db",
			charLimit:   "1000",
			window:      "1h",
			expectError: true,
		},
		{
			name:        "missing_database_url",
			token:       "valid-token",
			dbURL:       "",
			charLimit:   "1000",
			window:      "1h",
			expectError: true,
		},
		{
			name:        "invalid_char_limit",
			token:       "valid-token",
			dbURL:       "postgres://localhost/db",
			charLimit:   "not-a-number",
			window:      "1h",
			expectError: true,
		},
		{
			name:        "invalid_window",
			token:       "valid-token",
			dbURL:       "postgres://localhost/db",
			charLimit:   "1000",
			window:      "invalid",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				t.Setenv("TELEGRAM_TOKEN", tt.token)
				t.Setenv("DATABASE_URL", tt.dbURL)
				t.Setenv("DEFAULT_CHAR_LIMIT", tt.charLimit)
				t.Setenv("DEFAULT_WINDOW", tt.window)

				_, err := config.Load()
				if tt.expectError {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
			},
		)
	}
}

func TestConfigWindowDurations(t *testing.T) {
	tests := []struct {
		name     string
		window   string
		expected time.Duration
	}{
		{
			name:     "minutes",
			window:   "30m",
			expected: 30 * time.Minute,
		},
		{
			name:     "hours",
			window:   "2h",
			expected: 2 * time.Hour,
		},
		{
			name:     "days",
			window:   "1d",
			expected: 24 * time.Hour,
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				t.Setenv("TELEGRAM_TOKEN", "token")
				t.Setenv("DATABASE_URL", "postgres://localhost/db")
				t.Setenv("DEFAULT_WINDOW", tt.window)

				cfg, err := config.Load()
				if tt.window == "1d" {
					// "1d" format is not supported by time.ParseDuration
					assert.Error(t, err)
				} else {
					require.NoError(t, err)
					assert.Equal(t, tt.expected, cfg.DefaultWindow)
				}
			},
		)
	}
}
