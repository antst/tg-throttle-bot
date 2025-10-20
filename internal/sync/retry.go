// Package sync provides proactive member synchronization for Telegram groups.
package sync

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"time"
)

const (
	// MaxRetryAttempts is the maximum number of retry attempts
	MaxRetryAttempts = 5

	// MaxBackoff is the maximum backoff duration
	MaxBackoff = 60 * time.Second

	// JitterPercent is the randomness added to backoff (±20%)
	JitterPercent = 0.2
)

// RetryableError indicates an error that can be retried
type RetryableError struct {
	Err        error
	RetryAfter time.Duration // Telegram API retry_after hint
}

func (e *RetryableError) Error() string {
	if e.RetryAfter > 0 {
		return fmt.Sprintf("retryable error (retry after %v): %v", e.RetryAfter, e.Err)
	}
	return fmt.Sprintf("retryable error: %v", e.Err)
}

// IsRetryable checks if an error is retryable
func IsRetryable(err error) bool {
	_, ok := err.(*RetryableError)
	return ok
}

// RetryWithBackoff executes an operation with exponential backoff retry logic.
// Respects context cancellation and honors Telegram API retry_after hints.
func RetryWithBackoff(ctx context.Context, operation func() error) error {
	var lastErr error

	for attempt := 1; attempt <= MaxRetryAttempts; attempt++ {
		// Execute operation
		err := operation()
		if err == nil {
			return nil // Success
		}

		lastErr = err

		// Check if error is retryable
		if !IsRetryable(err) {
			return err // Non-retryable error, fail immediately
		}

		// Last attempt - don't wait
		if attempt == MaxRetryAttempts {
			break
		}

		// Calculate backoff duration
		waitTime := calculateBackoff(attempt, err)

		// Wait with context cancellation support
		select {
		case <-time.After(waitTime):
			// Continue to next attempt
		case <-ctx.Done():
			return fmt.Errorf("retry cancelled: %w", ctx.Err())
		}
	}

	return fmt.Errorf("max retries (%d) exceeded: %w", MaxRetryAttempts, lastErr)
}

// calculateBackoff computes the backoff duration with exponential growth and jitter.
// If the error includes a retry_after hint from Telegram API, it uses that as the base.
func calculateBackoff(attempt int, err error) time.Duration {
	var baseDuration time.Duration

	// Check for Telegram API retry_after hint
	if retryErr, ok := err.(*RetryableError); ok && retryErr.RetryAfter > 0 {
		baseDuration = retryErr.RetryAfter
	} else {
		// Exponential backoff: 1s, 2s, 4s, 8s, 16s, ...
		baseDuration = time.Duration(math.Pow(2, float64(attempt-1))) * time.Second
	}

	// Cap at MaxBackoff
	if baseDuration > MaxBackoff {
		baseDuration = MaxBackoff
	}

	// Add jitter (±20%)
	jitter := time.Duration(rand.Float64()*JitterPercent*2-JitterPercent) * baseDuration
	waitTime := baseDuration + jitter

	// Ensure non-negative
	if waitTime < 0 {
		waitTime = baseDuration
	}

	return waitTime
}

// NewRetryableError creates a RetryableError with optional retry_after hint
func NewRetryableError(err error, retryAfter time.Duration) error {
	return &RetryableError{
		Err:        err,
		RetryAfter: retryAfter,
	}
}
