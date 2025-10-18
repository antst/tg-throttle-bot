package telegram

import (
	"context"
	"sync"
	"time"
)

// APIRateLimiter implements a token bucket rate limiter for Telegram API calls.
// Telegram's limit is 30 messages per second across all chats.
type APIRateLimiter struct {
	tokens         float64
	maxTokens      float64
	refillRate     float64 // tokens per second
	lastRefillTime time.Time
	mu             sync.Mutex
}

// NewAPIRateLimiter creates a new rate limiter for Telegram API calls.
// Telegram allows 30 messages per second, so we set maxTokens to 30
// and refillRate to 30 tokens per second.
func NewAPIRateLimiter() *APIRateLimiter {
	return &APIRateLimiter{
		tokens:         30.0, // Start with full bucket
		maxTokens:      30.0, // Maximum 30 tokens
		refillRate:     30.0, // Refill at 30 tokens per second
		lastRefillTime: time.Now(),
	}
}

// Wait blocks until a token is available for making an API call.
// It respects context cancellation and returns an error if context is cancelled.
func (r *APIRateLimiter) Wait(ctx context.Context) error {
	for {
		// Check if we can proceed
		if r.tryAcquire() {
			return nil
		}

		// Calculate how long to wait for the next token
		waitDuration := r.timeUntilNextToken()

		// Wait with context support
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(waitDuration):
			// Continue loop to try acquiring again
		}
	}
}

// tryAcquire attempts to acquire a token without blocking.
// Returns true if a token was acquired, false otherwise.
func (r *APIRateLimiter) tryAcquire() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Refill tokens based on elapsed time
	r.refill()

	// Check if we have tokens available
	if r.tokens >= 1.0 {
		r.tokens -= 1.0
		return true
	}

	return false
}

// refill adds tokens to the bucket based on elapsed time.
// Must be called with mutex held.
func (r *APIRateLimiter) refill() {
	now := time.Now()
	elapsed := now.Sub(r.lastRefillTime).Seconds()

	// Add tokens based on elapsed time and refill rate
	r.tokens += elapsed * r.refillRate

	// Cap at maximum tokens
	if r.tokens > r.maxTokens {
		r.tokens = r.maxTokens
	}

	r.lastRefillTime = now
}

// timeUntilNextToken calculates how long to wait until the next token is available.
func (r *APIRateLimiter) timeUntilNextToken() time.Duration {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.refill()

	if r.tokens >= 1.0 {
		return 0
	}

	// Calculate time needed to generate one token
	tokensNeeded := 1.0 - r.tokens
	secondsNeeded := tokensNeeded / r.refillRate

	return time.Duration(secondsNeeded * float64(time.Second))
}

// GetStats returns current rate limiter statistics.
// Useful for monitoring and debugging.
func (r *APIRateLimiter) GetStats() (availableTokens float64, maxTokens float64) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.refill()
	return r.tokens, r.maxTokens
}
