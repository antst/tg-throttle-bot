// Package storage provides database persistence layer implementations.
// Clean adapter with ONLY multi-window methods (no legacy code)
package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/antst/tg-throttle-bot/internal/ratelimit"
	"github.com/antst/tg-throttle-bot/internal/storage/sqlc"
	"github.com/jackc/pgx/v5/pgtype"
)

// RateLimitStorage adapts PostgresStore to implement ratelimit.MultiWindowStorage interface
type RateLimitStorage struct {
	store *PostgresStore
}

// NewRateLimitStorage creates a new rate limit storage adapter that wraps a PostgresStore.
func NewRateLimitStorage(store *PostgresStore) *RateLimitStorage {
	return &RateLimitStorage{store: store}
}

// ============================================================================
// User Management Methods (Username Harvesting - Feature 006 User Story 6)
// ============================================================================

// EnsureUser creates or updates a user record with username
// Called on EVERY message to harvest/update usernames automatically
func (s *RateLimitStorage) EnsureUser(ctx context.Context, userID int64, username string) error {
	var usernamePtr *string
	if username != "" {
		usernamePtr = &username
	}

	return s.store.EnsureUser(ctx, sqlc.EnsureUserParams{
		UserID:   userID,
		Username: usernamePtr,
	})
} // GetUserByUsername resolves @username to user_id (case-insensitive)
// Returns error if username not found or empty
func (s *RateLimitStorage) GetUserByUsername(ctx context.Context, username string) (int64, error) {
	if username == "" {
		return 0, fmt.Errorf("username cannot be empty")
	}

	userID, err := s.store.GetUserByUsername(ctx, username)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, fmt.Errorf("user with username '@%s' not found (user must have sent at least one message)", username)
		}
		return 0, fmt.Errorf("failed to resolve username: %w", err)
	}

	return userID, nil
}

// GetUsernameByUserID performs reverse lookup: user_id -> @username
// Returns empty string if user has no username set
func (s *RateLimitStorage) GetUsernameByUserID(ctx context.Context, userID int64) (string, error) {
	username, err := s.store.GetUsernameByUserID(ctx, userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil // User exists but has no username
		}
		return "", fmt.Errorf("failed to get username: %w", err)
	}

	if username == nil {
		return "", nil
	}

	return *username, nil
} // ============================================================================
// Simple Messages-Based Methods (Feature 006 Refactoring)
// ============================================================================

// RecordMessage records a single message for sliding window calculation
func (s *RateLimitStorage) RecordMessage(ctx context.Context, userID, chatID int64, charCount int) error {
	// Validate charCount fits in int32 to prevent overflow
	if charCount > 2147483647 || charCount < 0 {
		return fmt.Errorf("char count %d exceeds int32 range", charCount)
	}

	// Note: User record should already exist from EnsureUser() called in message handler
	// We don't call UpsertUser here to avoid overwriting telegram_username with NULL

	// Single message row serves all windows - no slot_id duplication!
	return s.store.RecordMessage(ctx, sqlc.RecordMessageParams{
		UserID:    userID,
		ChatID:    chatID,
		CharCount: int32(charCount), // #nosec G115 - validated above
	})
}

// GetWindowUsage calculates current usage for a window using sliding window
// All windows share the same message log, filtered by time window
func (s *RateLimitStorage) GetWindowUsage(ctx context.Context, userID, chatID int64, windowSeconds int) (int, error) {
	windowStr := fmt.Sprintf("%d", windowSeconds)
	total, err := s.store.GetWindowUsage(ctx, sqlc.GetWindowUsageParams{
		UserID:  userID,
		ChatID:  chatID,
		Column3: &windowStr,
	})
	if err != nil {
		return 0, fmt.Errorf("failed to get window usage: %w", err)
	}
	return int(total), nil
}

// CleanupOldMessages removes messages older than retention period
func (s *RateLimitStorage) CleanupOldMessages(ctx context.Context, retentionSeconds int) error {
	retentionStr := fmt.Sprintf("%d", retentionSeconds)
	return s.store.CleanupOldMessages(ctx, &retentionStr)
}

// ResetWindowMessages resets messages for a specific user and window
// ResetWindowMessages resets all messages for a user in a chat (affects all windows)
func (s *RateLimitStorage) ResetWindowMessages(ctx context.Context, userID, chatID int64) error {
	return s.store.ResetWindowMessages(ctx, sqlc.ResetWindowMessagesParams{
		UserID: userID,
		ChatID: chatID,
	})
}

// ResetWindowMessagesForAll resets all messages for all users in a chat (affects all windows)
func (s *RateLimitStorage) ResetWindowMessagesForAll(ctx context.Context, chatID int64) error {
	return s.store.ResetWindowMessagesForAll(ctx, chatID)
}

// GetUserWindowStats returns usage statistics for all windows for a user (for /mystatus)
func (s *RateLimitStorage) GetUserWindowStats(ctx context.Context, userID, chatID int64) ([]*ratelimit.UserWindowStat, error) {
	rows, err := s.store.GetUserWindowStats(ctx, sqlc.GetUserWindowStatsParams{
		UserID: userID,
		ChatID: chatID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get user window stats: %w", err)
	}

	stats := make([]*ratelimit.UserWindowStat, 0, len(rows))
	for _, row := range rows {
		stats = append(stats, &ratelimit.UserWindowStat{
			SlotID:         row.SlotID,
			CharLimit:      int(row.CharLimit),
			DurationValue:  int(row.DurationValue),
			DurationUnit:   row.DurationUnit,
			WindowDuration: int(row.WindowDuration),
			Enabled:        row.Enabled,
			CurrentUsage:   int(row.CurrentUsage),
		})
	}
	return stats, nil
}

// ============================================================================
// Window Slot Configuration Methods
// ============================================================================

// CreateDefaultWindows initializes default windows for a new group
func (s *RateLimitStorage) CreateDefaultWindows(ctx context.Context, chatID int64) error {
	// First, ensure the group record exists (required for foreign key constraint)
	if err := s.store.EnsureGroup(ctx, chatID); err != nil {
		return fmt.Errorf("failed to ensure group exists: %w", err)
	}

	// Then create default window slots
	return s.store.CreateDefaultWindows(ctx, chatID)
}

// GetEnabledWindows returns all enabled windows for a chat
func (s *RateLimitStorage) GetEnabledWindows(ctx context.Context, chatID int64) ([]*ratelimit.WindowSlot, error) {
	rows, err := s.store.GetEnabledWindows(ctx, chatID)
	if err != nil {
		return nil, fmt.Errorf("failed to get enabled windows: %w", err)
	}

	// If no windows exist, initialize group with defaults
	if len(rows) == 0 {
		log.Printf("No windows found for chat %d, initializing with defaults", chatID)

		// First, ensure the group exists
		if err := s.ensureGroupExists(ctx, chatID); err != nil {
			return nil, fmt.Errorf("failed to ensure group exists: %w", err)
		}

		// Create default windows
		if err := s.CreateDefaultWindows(ctx, chatID); err != nil {
			return nil, fmt.Errorf("failed to create default windows: %w", err)
		}

		// Retry getting windows
		rows, err = s.store.GetEnabledWindows(ctx, chatID)
		if err != nil {
			return nil, fmt.Errorf("failed to get enabled windows after initialization: %w", err)
		}
	}

	windows := make([]*ratelimit.WindowSlot, 0, len(rows))
	for _, row := range rows {
		windows = append(windows, &ratelimit.WindowSlot{
			ID:             int(row.ID),
			ChatID:         chatID,
			SlotID:         row.SlotID,
			CharLimit:      int(row.CharLimit),
			DurationValue:  int(row.DurationValue),
			DurationUnit:   row.DurationUnit,
			WindowDuration: int(row.WindowDuration),
			Enabled:        true,
		})
	}
	return windows, nil
}

// ensureGroupExists creates a group record if it doesn't exist
// In the new design, groups are created automatically via CreateDefaultWindows
// This is a no-op that just checks if group exists
func (s *RateLimitStorage) ensureGroupExists(ctx context.Context, chatID int64) error {
	_, err := s.store.GetGroupConfig(ctx, chatID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("failed to check group existence: %w", err)
	}
	// Group will be created when CreateDefaultWindows is called
	return nil
}

// GetAllWindows returns all windows (enabled and disabled) for a chat
func (s *RateLimitStorage) GetAllWindows(ctx context.Context, chatID int64) ([]*ratelimit.WindowSlot, error) {
	rows, err := s.store.GetAllWindows(ctx, chatID)
	if err != nil {
		return nil, fmt.Errorf("failed to get all windows: %w", err)
	}

	windows := make([]*ratelimit.WindowSlot, 0, len(rows))
	for _, row := range rows {
		windows = append(windows, &ratelimit.WindowSlot{
			ID:             int(row.ID),
			ChatID:         row.ChatID,
			SlotID:         row.SlotID,
			CharLimit:      int(row.CharLimit),
			DurationValue:  int(row.DurationValue),
			DurationUnit:   row.DurationUnit,
			WindowDuration: int(row.WindowDuration),
			Enabled:        row.Enabled,
			CreatedAt:      row.CreatedAt.Time,
			UpdatedAt:      row.UpdatedAt.Time,
		})
	}
	return windows, nil
}

// GetWindowSlot returns configuration for a specific window slot
func (s *RateLimitStorage) GetWindowSlot(ctx context.Context, chatID int64, slotID string) (*ratelimit.WindowSlot, error) {
	row, err := s.store.GetWindowSlot(ctx, sqlc.GetWindowSlotParams{
		ChatID: chatID,
		SlotID: slotID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get window slot: %w", err)
	}

	return &ratelimit.WindowSlot{
		ID:             int(row.ID),
		ChatID:         row.ChatID,
		SlotID:         row.SlotID,
		CharLimit:      int(row.CharLimit),
		DurationValue:  int(row.DurationValue),
		DurationUnit:   row.DurationUnit,
		WindowDuration: int(row.WindowDuration),
		Enabled:        row.Enabled,
		CreatedAt:      row.CreatedAt.Time,
		UpdatedAt:      row.UpdatedAt.Time,
	}, nil
}

// UpdateWindowSlot updates window slot configuration
func (s *RateLimitStorage) UpdateWindowSlot(ctx context.Context, chatID int64, slotID string, charLimit int, durationValue int, durationUnit string, windowDuration int, enabled bool) error {
	// Validate inputs fit in int32 to prevent overflow
	if charLimit > 2147483647 || charLimit < 0 {
		return fmt.Errorf("char limit %d exceeds int32 range", charLimit)
	}
	if windowDuration > 2147483647 || windowDuration < 0 {
		return fmt.Errorf("window duration %d exceeds int32 range", windowDuration)
	}
	if durationValue > 2147483647 || durationValue < 0 {
		return fmt.Errorf("duration value %d exceeds int32 range", durationValue)
	}

	return s.store.UpdateWindowSlot(ctx, sqlc.UpdateWindowSlotParams{
		ChatID:         chatID,
		SlotID:         slotID,
		CharLimit:      int32(charLimit),     // #nosec G115 - validated above
		DurationValue:  int32(durationValue), // #nosec G115 - validated above
		DurationUnit:   durationUnit,
		WindowDuration: int32(windowDuration), // #nosec G115 - validated above
		Enabled:        enabled,
	})
}

// SetWindowEnabled enables or disables a window
func (s *RateLimitStorage) SetWindowEnabled(ctx context.Context, chatID int64, slotID string, enabled bool) error {
	return s.store.SetWindowEnabled(ctx, sqlc.SetWindowEnabledParams{
		ChatID:  chatID,
		SlotID:  slotID,
		Enabled: enabled,
	})
}

// Convenience wrapper methods (required by MultiWindowStorage interface)

// SetWindowSlot is a convenience wrapper for UpdateWindowSlot
func (s *RateLimitStorage) SetWindowSlot(ctx context.Context, chatID int64, slotID string, charLimit int, durationValue int, durationUnit string, windowDuration int, enabled bool) error {
	return s.UpdateWindowSlot(ctx, chatID, slotID, charLimit, durationValue, durationUnit, windowDuration, enabled)
}

// DisableWindow is a convenience wrapper for SetWindowEnabled(false)
func (s *RateLimitStorage) DisableWindow(ctx context.Context, chatID int64, slotID string) error {
	return s.SetWindowEnabled(ctx, chatID, slotID, false)
}

// EnableWindow is a convenience wrapper for SetWindowEnabled(true)
func (s *RateLimitStorage) EnableWindow(ctx context.Context, chatID int64, slotID string) error {
	return s.SetWindowEnabled(ctx, chatID, slotID, true)
}

// ============================================================================
// REMOVED: Manual Moderation Methods (Exemptions) - migration 000007
// ============================================================================
// REMOVED: IsUserExemptFromWindow - exemptions table removed
// Note: Use GetUserOverride() for user-specific exemptions instead

// ============================================================================
// Group Pause/Resume Methods (Blacklist)
// ============================================================================

// IsGroupPaused checks if rate limiting is paused for a group
func (s *RateLimitStorage) IsGroupPaused(ctx context.Context, chatID int64) (bool, error) {
	result, err := s.store.IsGroupPaused(ctx, chatID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Group doesn't exist yet, not paused
			return false, nil
		}
		return false, fmt.Errorf("failed to check if group is paused: %w", err)
	}
	return result.Paused, nil
}

// SetGroupPaused sets the paused state for a group
func (s *RateLimitStorage) SetGroupPaused(ctx context.Context, chatID int64, paused bool, resumeAt *time.Time) error {
	// Ensure group exists first
	if err := s.ensureGroupExists(ctx, chatID); err != nil {
		return fmt.Errorf("failed to ensure group exists: %w", err)
	}

	// Convert time.Time to pgtype.Timestamptz
	var resumeAtPG pgtype.Timestamptz
	if resumeAt != nil {
		resumeAtPG = pgtype.Timestamptz{
			Time:  *resumeAt,
			Valid: true,
		}
	}

	return s.store.SetGroupPaused(ctx, sqlc.SetGroupPausedParams{
		ChatID:   chatID,
		Paused:   paused,
		ResumeAt: resumeAtPG,
	})
}

// GetAllGroups returns all configured groups (for permission checking)
func (s *RateLimitStorage) GetAllGroups(ctx context.Context) ([]int64, error) {
	chatIDs, err := s.store.GetAllGroups(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get all groups: %w", err)
	}
	return chatIDs, nil
}

// ============================================================================
// Manual User Overrides (3-State Control)
// ============================================================================

// GetUserOverride returns the manual override state for a user
// Returns: nil = follow rate limiter, true = whitelist (always allow), false = blacklist (always block)
func (s *RateLimitStorage) GetUserOverride(ctx context.Context, chatID, userID int64) (*bool, error) {
	row, err := s.store.GetUserOverride(ctx, sqlc.GetUserOverrideParams{
		ChatID: chatID,
		UserID: userID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// No override set, return nil (follow rate limiter)
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get user override: %w", err)
	}

	// SQLC returns *bool directly (nil if NULL in DB)
	return row.OverrideState, nil
}

// SetUserOverride sets the manual override state for a user
// overrideState: nil = follow rate limiter, true = whitelist, false = blacklist
func (s *RateLimitStorage) SetUserOverride(ctx context.Context, chatID, userID int64, overrideState *bool, reason string, createdBy int64, expiresAt *time.Time) error {
	// Ensure group exists (required for foreign key constraint)
	if err := s.store.EnsureGroup(ctx, chatID); err != nil {
		return fmt.Errorf("failed to ensure group exists: %w", err)
	}

	// Ensure user exists (required for foreign key constraint)
	var username *string // We don't have username here, will be nil (NULL in DB)
	if err := s.store.EnsureUser(ctx, sqlc.EnsureUserParams{
		UserID:   userID,
		Username: username,
	}); err != nil {
		return fmt.Errorf("failed to ensure user exists: %w", err)
	}

	// Convert reason to *string (empty string -> nil)
	var reasonPtr *string
	if reason != "" {
		reasonPtr = &reason
	}

	// Convert *time.Time to pgtype.Timestamptz
	var expiresAtPG pgtype.Timestamptz
	if expiresAt != nil {
		expiresAtPG = pgtype.Timestamptz{
			Time:  *expiresAt,
			Valid: true,
		}
	}

	return s.store.SetUserOverride(ctx, sqlc.SetUserOverrideParams{
		ChatID:        chatID,
		UserID:        userID,
		OverrideState: overrideState, // SQLC expects *bool directly
		Reason:        reasonPtr,     // SQLC expects *string
		CreatedBy:     createdBy,
		ExpiresAt:     expiresAtPG,
	})
}

// RemoveUserOverride removes the manual override for a user (returns to rate limiter control)
func (s *RateLimitStorage) RemoveUserOverride(ctx context.Context, chatID, userID int64) error {
	return s.store.RemoveUserOverride(ctx, sqlc.RemoveUserOverrideParams{
		ChatID: chatID,
		UserID: userID,
	})
}

// GetAllOverrides returns all manual overrides for a chat
func (s *RateLimitStorage) GetAllOverrides(ctx context.Context, chatID int64) ([]*ratelimit.UserOverride, error) {
	rows, err := s.store.GetAllOverrides(ctx, chatID)
	if err != nil {
		return nil, fmt.Errorf("failed to get all overrides: %w", err)
	}

	overrides := make([]*ratelimit.UserOverride, 0, len(rows))
	for _, row := range rows {
		var expiresAt *time.Time
		if row.ExpiresAt.Valid {
			expiresAt = &row.ExpiresAt.Time
		}

		overrides = append(overrides, &ratelimit.UserOverride{
			UserID:        row.UserID,
			OverrideState: row.OverrideState,
			Reason:        row.Reason,
			CreatedAt:     row.CreatedAt.Time,
			CreatedBy:     row.CreatedBy,
			ExpiresAt:     expiresAt,
		})
	}

	return overrides, nil
}

// CleanupExpiredOverrides removes expired overrides (where expires_at < NOW)
func (s *RateLimitStorage) CleanupExpiredOverrides(ctx context.Context) error {
	return s.store.CleanupExpiredOverrides(ctx)
}
