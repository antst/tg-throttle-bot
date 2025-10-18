package unit

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/antst/tg-throttle-bot/internal/bot"
)

func TestSanitizeError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
		contains string // For partial matching
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: "",
		},
		{
			name:     "context cancelled",
			err:      context.Canceled,
			expected: "⚠️ Operation was cancelled",
		},
		{
			name:     "context deadline exceeded",
			err:      context.DeadlineExceeded,
			expected: "⚠️ Operation timed out. Please try again.",
		},
		{
			name:     "database connection done",
			err:      sql.ErrConnDone,
			expected: "⚠️ Database temporarily unavailable. Please try again in a moment.",
		},
		{
			name:     "no rows found",
			err:      sql.ErrNoRows,
			expected: "❌ No data found",
		},
		{
			name:     "pgx no rows",
			err:      pgx.ErrNoRows,
			expected: "⚠️ Database temporarily unavailable. Please try again in a moment.",
		},
		{
			name:     "error with database connection string",
			err:      fmt.Errorf("failed to connect to postgres://user:password@localhost:5432/dbname"),
			contains: "❌",
		},
		{
			name:     "error with file path",
			err:      fmt.Errorf("cannot open /Users/admin/secret/config.yaml"),
			contains: "❌",
		},
		{
			name:     "error with internal package path",
			err:      fmt.Errorf("error in github.com/antst/tg-throttle-bot/internal/storage/postgres.go:123"),
			contains: "❌",
		},
		{
			name:     "error with panic keyword",
			err:      fmt.Errorf("panic: runtime error"),
			expected: "❌ An error occurred. Please try again or contact an administrator.",
		},
		{
			name:     "error with stack trace keyword",
			err:      fmt.Errorf("stack trace: goroutine 1 [running]"),
			expected: "❌ An error occurred. Please try again or contact an administrator.",
		},
		{
			name:     "generic error without sensitive info",
			err:      fmt.Errorf("invalid input format"),
			expected: "❌ invalid input format",
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				result := bot.SanitizeError(tt.err)

				if tt.expected != "" {
					if result != tt.expected {
						t.Errorf("SanitizeError() = %q, want %q", result, tt.expected)
					}
				} else if tt.contains != "" {
					if !strings.Contains(result, tt.contains) {
						t.Errorf("SanitizeError() = %q, should contain %q", result, tt.contains)
					}
				}

				// Verify no sensitive information leaked
				sensitivePatterns := []string{
					"postgres://",
					"postgresql://",
					"password",
					"/Users/",
					"/home/",
					"github.com/antst/tg-throttle-bot/internal/",
					":5432",
				}

				for _, pattern := range sensitivePatterns {
					if strings.Contains(result, pattern) {
						t.Errorf("SanitizeError() leaked sensitive information: %q found in %q", pattern, result)
					}
				}
			},
		)
	}
}

func TestSanitizePgError(t *testing.T) {
	tests := []struct {
		name     string
		pgCode   string
		expected string
	}{
		{
			name:     "unique violation",
			pgCode:   "23505",
			expected: "❌ This action would create a duplicate entry",
		},
		{
			name:     "foreign key violation",
			pgCode:   "23503",
			expected: "❌ Cannot perform this action due to related data",
		},
		{
			name:     "not null violation",
			pgCode:   "23502",
			expected: "❌ Required information is missing",
		},
		{
			name:     "string too long",
			pgCode:   "22001",
			expected: "❌ Input is too long",
		},
		{
			name:     "numeric out of range",
			pgCode:   "22003",
			expected: "❌ Number is out of valid range",
		},
		{
			name:     "connection exception",
			pgCode:   "08000",
			expected: "⚠️ Database connection issue. Please try again.",
		},
		{
			name:     "connection does not exist",
			pgCode:   "08003",
			expected: "⚠️ Database connection issue. Please try again.",
		},
		{
			name:     "connection failure",
			pgCode:   "08006",
			expected: "⚠️ Database connection issue. Please try again.",
		},
		{
			name:     "cannot connect now",
			pgCode:   "57P03",
			expected: "⚠️ Database is busy. Please try again in a moment.",
		},
		{
			name:     "too many connections",
			pgCode:   "53300",
			expected: "⚠️ Service is experiencing high load. Please try again.",
		},
		{
			name:     "unknown error code",
			pgCode:   "99999",
			expected: "❌ A database error occurred. Please contact an administrator.",
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				pgErr := &pgconn.PgError{
					Code:    tt.pgCode,
					Message: "internal database error message that should not be exposed",
				}

				result := bot.SanitizeError(pgErr)

				if result != tt.expected {
					t.Errorf("SanitizeError(pgError) = %q, want %q", result, tt.expected)
				}

				// Verify internal error message is not leaked
				if strings.Contains(result, "internal database error message") {
					t.Errorf("SanitizeError() leaked internal error message: %q", result)
				}
			},
		)
	}
}

func TestUserFriendlyError(t *testing.T) {
	t.Run(
		"creates user friendly error", func(t *testing.T) {
			internalErr := errors.New("sql: database connection lost at postgres://user:pass@localhost:5432/db")
			userMsg := "Database temporarily unavailable"

			err := bot.NewUserFriendlyError(userMsg, internalErr)

			// Check user message
			if err.Error() != userMsg {
				t.Errorf("UserFriendlyError.Error() = %q, want %q", err.Error(), userMsg)
			}

			// Check unwrap
			if !errors.Is(err, internalErr) {
				t.Error("UserFriendlyError should unwrap to internal error")
			}
		},
	)

	t.Run(
		"unwrap preserves original error", func(t *testing.T) {
			internalErr := fmt.Errorf("wrapped: %w", sql.ErrConnDone)
			err := bot.NewUserFriendlyError("friendly message", internalErr)

			// Should be able to check for original error
			if !errors.Is(err, sql.ErrConnDone) {
				t.Error("Should be able to unwrap to sql.ErrConnDone")
			}
		},
	)
}

func TestContainsSensitiveKeywords(t *testing.T) {
	tests := []struct {
		name         string
		errString    string
		hasSensitive bool
	}{
		{
			name:         "contains panic",
			errString:    "panic: runtime error: invalid memory address",
			hasSensitive: true,
		},
		{
			name:         "contains stack trace",
			errString:    "error with stack trace information",
			hasSensitive: true,
		},
		{
			name:         "contains goroutine",
			errString:    "goroutine 15 [running]",
			hasSensitive: true,
		},
		{
			name:         "contains connection refused",
			errString:    "dial tcp: connection refused",
			hasSensitive: true,
		},
		{
			name:         "contains nil pointer",
			errString:    "runtime error: nil pointer dereference",
			hasSensitive: true,
		},
		{
			name:         "safe error message",
			errString:    "invalid input format",
			hasSensitive: false,
		},
		{
			name:         "safe user message",
			errString:    "rate limit exceeded",
			hasSensitive: false,
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				result := bot.SanitizeError(errors.New(tt.errString))

				// If error contains sensitive keywords, it should return generic message
				if tt.hasSensitive {
					expectedGeneric := "❌ An error occurred. Please try again or contact an administrator."
					if result != expectedGeneric {
						t.Errorf("Expected generic message for sensitive error, got %q", result)
					}
				} else if !strings.Contains(result, "❌") {
					// Safe errors should still be sanitized but preserve some meaning
					t.Errorf("Expected error marker in result, got %q", result)
				}
			},
		)
	}
}

func TestRemoveSensitivePatterns(t *testing.T) {
	tests := []struct {
		name             string
		input            string
		shouldNotContain []string
	}{
		{
			name:             "removes postgres connection string",
			input:            "failed to connect: postgres://user:password@localhost:5432/mydb",
			shouldNotContain: []string{"postgres://", "password", "localhost", ":5432"},
		},
		{
			name:             "removes postgresql connection string",
			input:            "error: postgresql://admin:secret@db.example.com:5432/prod",
			shouldNotContain: []string{"postgresql://", "secret", "db.example.com"},
		},
		{
			name:             "removes file paths",
			input:            "cannot read /Users/admin/secret/config.yaml",
			shouldNotContain: []string{"/Users/"},
		},
		{
			name:             "removes internal package paths",
			input:            "error in github.com/antst/tg-throttle-bot/internal/storage/postgres.go:123",
			shouldNotContain: []string{"github.com/antst/tg-throttle-bot/internal/", "internal/"},
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				result := bot.SanitizeError(errors.New(tt.input))

				for _, pattern := range tt.shouldNotContain {
					if strings.Contains(result, pattern) {
						t.Errorf("Result should not contain %q, but got: %q", pattern, result)
					}
				}
			},
		)
	}
}
