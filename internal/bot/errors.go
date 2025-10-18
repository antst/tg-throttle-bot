// Package bot provides Telegram bot handlers and command processing.
package bot

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// SanitizeError converts internal errors to user-friendly messages.
// It removes sensitive information like database connection strings, internal paths,
// and implementation details while preserving the error type for proper handling.
func SanitizeError(err error) string {
	if err == nil {
		return ""
	}

	// Check for context cancellation
	if errors.Is(err, context.Canceled) {
		return "⚠️ Operation was cancelled"
	}

	// Check for context timeout
	if errors.Is(err, context.DeadlineExceeded) {
		return "⚠️ Operation timed out. Please try again."
	}

	// Check for database connection errors
	if errors.Is(err, sql.ErrConnDone) || errors.Is(err, pgx.ErrNoRows) {
		return "⚠️ Database temporarily unavailable. Please try again in a moment."
	}

	// Check for pgconn errors (PostgreSQL-specific)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return sanitizePgError(pgErr)
	}

	// Check for no rows error (not really an error, but handle it)
	if errors.Is(err, sql.ErrNoRows) {
		return "❌ No data found"
	}

	// Generic error message - completely sanitized
	errStr := err.Error()

	// Remove common sensitive patterns
	errStr = removeSensitivePatterns(errStr)

	// If error is too technical or contains sensitive keywords, return generic message
	if containsSensitiveKeywords(errStr) {
		return "❌ An error occurred. Please try again or contact an administrator."
	}

	return fmt.Sprintf("❌ %s", errStr)
}

// sanitizePgError converts PostgreSQL-specific errors to user-friendly messages
func sanitizePgError(pgErr *pgconn.PgError) string {
	switch pgErr.Code {
	case "23505": // unique_violation
		return "❌ This action would create a duplicate entry"
	case "23503": // foreign_key_violation
		return "❌ Cannot perform this action due to related data"
	case "23502": // not_null_violation
		return "❌ Required information is missing"
	case "22001": // string_data_right_truncation
		return "❌ Input is too long"
	case "22003": // numeric_value_out_of_range
		return "❌ Number is out of valid range"
	case "08000", "08003", "08006": // connection_exception, connection_does_not_exist, connection_failure
		return "⚠️ Database connection issue. Please try again."
	case "57P03": // cannot_connect_now
		return "⚠️ Database is busy. Please try again in a moment."
	case "53300": // too_many_connections
		return "⚠️ Service is experiencing high load. Please try again."
	default:
		// Don't expose internal database error codes
		return "❌ A database error occurred. Please contact an administrator."
	}
}

// removeSensitivePatterns removes sensitive information from error strings
func removeSensitivePatterns(errStr string) string {
	// First, handle connection strings with a more aggressive approach
	// Replace entire connection strings with generic placeholders
	if strings.Contains(errStr, "postgres://") || strings.Contains(errStr, "postgresql://") {
		// Find and replace the entire connection string
		re := regexp.MustCompile(`postgres(?:ql)?://[^@\s]+@[^\s/]+(?:/[^\s]*)?`)
		errStr = re.ReplaceAllString(errStr, "[DATABASE_CONNECTION]")
	}

	patterns := []struct {
		keyword string
		replace string
	}{
		// Database connection components (in case the full string wasn't caught)
		{"password=", "password=[REDACTED] "},
		{"user=", "user=[REDACTED] "},
		{"host=", "host=[REDACTED] "},
		{"port=", "port=[REDACTED] "},
		{"dbname=", "dbname=[REDACTED] "},
		{"sslmode=", "sslmode=[REDACTED] "},

		// File paths
		{"/Users/", "[path]/"},
		{"/home/", "[path]/"},
		{"/var/", "[path]/"},
		{"/opt/", "[path]/"},
		{"/usr/", "[path]/"},
		{"C:\\", "[path]\\"},

		// Internal package references
		{"github.com/antst/tg-throttle-bot/internal/", ""},
		{"internal/", ""},

		// IP addresses and ports
		{".tcp(", ""},
		{":5432", ""},
		{":3306", ""},
	}

	result := errStr
	for _, p := range patterns {
		result = strings.ReplaceAll(result, p.keyword, p.replace)
	}

	return strings.TrimSpace(result)
}

// containsSensitiveKeywords checks if error string contains sensitive keywords
func containsSensitiveKeywords(errStr string) bool {
	lowerErr := strings.ToLower(errStr)
	sensitiveKeywords := []string{
		"panic",
		"stack trace",
		"goroutine",
		"runtime.",
		"sql:",
		"pgx:",
		"connection refused",
		"dial tcp",
		"syscall",
		"errno",
		"segmentation fault",
		"nil pointer",
		"index out of range",
	}

	for _, keyword := range sensitiveKeywords {
		if strings.Contains(lowerErr, keyword) {
			return true
		}
	}

	return false
}

// UserFriendlyError wraps an error with a user-friendly message while preserving the original error for logging
type UserFriendlyError struct {
	UserMessage string // Message to show to the user
	InternalErr error  // Original error for logging
}

// Error implements the error interface
func (e *UserFriendlyError) Error() string {
	return e.UserMessage
}

// Unwrap implements the unwrap interface for errors.Is and errors.As
func (e *UserFriendlyError) Unwrap() error {
	return e.InternalErr
}

// NewUserFriendlyError creates a new user-friendly error
func NewUserFriendlyError(userMsg string, internalErr error) error {
	return &UserFriendlyError{
		UserMessage: userMsg,
		InternalErr: internalErr,
	}
}
