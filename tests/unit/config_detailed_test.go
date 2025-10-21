package unit

import (
	"testing"

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
}

func TestConfigCustomValues(t *testing.T) {
	// Set custom values
	t.Setenv("TELEGRAM_TOKEN", "custom-token")
	t.Setenv("DATABASE_URL", "postgres://custom/db")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("PORT", "9000")

	cfg, err := config.Load()
	require.NoError(t, err)

	assert.Equal(t, "custom-token", cfg.TelegramToken)
	assert.Equal(t, "postgres://custom/db", cfg.DatabaseURL)
	assert.Equal(t, "debug", cfg.LogLevel)
	assert.Equal(t, "9000", cfg.Port)
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name        string
		token       string
		dbURL       string
		expectError bool
	}{
		{
			name:        "valid_config",
			token:       "valid-token",
			dbURL:       "postgres://localhost/db",
			expectError: false,
		},
		{
			name:        "missing_token",
			token:       "",
			dbURL:       "postgres://localhost/db",
			expectError: true,
		},
		{
			name:        "missing_database_url",
			token:       "valid-token",
			dbURL:       "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				t.Setenv("TELEGRAM_TOKEN", tt.token)
				t.Setenv("DATABASE_URL", tt.dbURL)

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
