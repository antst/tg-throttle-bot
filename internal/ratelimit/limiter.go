// Package ratelimit provides rate limiting functionality for the Telegram bot.
// Clean implementation with ONLY multi-window methods (no legacy code)
package ratelimit

import (
	"context"
	"fmt"
)

// Limiter handles multi-window rate limiting with sliding window algorithm
// All windows share a single message log for efficiency
type Limiter struct {
	storage MultiWindowStorage
}

// NewLimiter creates a new multi-window rate limiter
func NewLimiter(storage MultiWindowStorage) *Limiter {
	return &Limiter{
		storage: storage,
	}
}

// CheckMessageMultiWindow validates if a message exceeds ANY enabled window's rate limit.
// Returns evaluation results for all windows including violated ones.
// Sequential evaluation: A → B → C for predictable behavior
func (l *Limiter) CheckMessageMultiWindow(ctx context.Context, chatID, userID int64, text string) (
	*MultiWindowEvaluationResult, error,
) {
	charCount := len([]rune(text))

	result := &MultiWindowEvaluationResult{
		Windows:         make([]*WindowEvaluationResult, 0),
		ViolatedWindows: make([]string, 0),
		AllowMessage:    true,
		NewWarnings:     make([]*WindowEvaluationResult, 0),
	}

	// Get all enabled windows
	windows, err := l.storage.GetEnabledWindows(ctx, chatID)
	if err != nil {
		return nil, fmt.Errorf("get enabled windows: %w", err)
	}

	// If no windows enabled, allow message
	if len(windows) == 0 {
		return result, nil
	}

	// Sequential evaluation: A → B → C
	for _, window := range windows {
		// Get current usage from shared message log (filtered by time window)
		currentUsage, err := l.storage.GetWindowUsage(ctx, userID, chatID, window.WindowDuration)
		if err != nil {
			return nil, fmt.Errorf("calculate usage for window %s: %w", window.SlotID, err)
		}

		// Calculate projected usage and percentage
		projectedUsage := currentUsage + charCount
		percentage := 0
		if window.CharLimit > 0 {
			percentage = (projectedUsage * 100) / window.CharLimit
		}

		// Determine violation and warning status
		violated := projectedUsage > window.CharLimit
		shouldWarn := percentage >= 95 // Spam warning at 95%+

		windowResult := &WindowEvaluationResult{
			SlotID:     window.SlotID,
			CharCount:  projectedUsage,
			CharLimit:  window.CharLimit,
			Percentage: percentage,
			Violated:   violated,
			ShouldWarn: shouldWarn,
		}

		result.Windows = append(result.Windows, windowResult)

		// Track violated windows
		if violated {
			result.ViolatedWindows = append(result.ViolatedWindows, window.SlotID)
			result.AllowMessage = false
		}

		// Track windows requiring warnings (≥95%)
		if shouldWarn {
			result.NewWarnings = append(result.NewWarnings, windowResult)
		}
	}

	return result, nil
}

// RecordMessageForAllWindows records message usage for all windows
// Inserts ONE row into simple_messages table - shared across all windows
// Each window calculates its own usage by filtering this shared log by time
func (l *Limiter) RecordMessageForAllWindows(ctx context.Context, chatID, userID int64, charCount int) error {
	return l.storage.RecordMessage(ctx, userID, chatID, charCount)
}

// MultiWindowEvaluationResult contains results from evaluating all windows
type MultiWindowEvaluationResult struct {
	Windows         []*WindowEvaluationResult
	ViolatedWindows []string
	AllowMessage    bool
	NewWarnings     []*WindowEvaluationResult
}

// WindowEvaluationResult contains the result of evaluating a single window
type WindowEvaluationResult struct {
	SlotID     string
	CharCount  int
	CharLimit  int
	Percentage int
	Violated   bool
	ShouldWarn bool // True when ≥95% (spam warnings on every message)
}
