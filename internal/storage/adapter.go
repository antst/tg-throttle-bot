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
}

// GetGroupByUsername resolves @groupname to group_id (case-insensitive)
// Returns error if groupname not found or empty
func (s *RateLimitStorage) GetGroupByUsername(ctx context.Context, username string) (int64, error) {
	if username == "" {
		return 0, fmt.Errorf("group username cannot be empty")
	}

	row, err := s.store.GetGroupByUsername(ctx, username)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, fmt.Errorf("group with username '@%s' not found (you must be managing this group)", username)
		}
		return 0, fmt.Errorf("failed to resolve group username: %w", err)
	}

	return row.ChatID, nil
}

// ============================================================================
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

// GetGroupPausedUntil returns the timestamp when rate limiting will resume (if paused)
func (s *RateLimitStorage) GetGroupPausedUntil(ctx context.Context, chatID int64) (*time.Time, error) {
	result, err := s.store.IsGroupPaused(ctx, chatID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get group paused until: %w", err)
	}
	if !result.Paused || !result.ResumeAt.Valid {
		return nil, nil
	}
	return &result.ResumeAt.Time, nil
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

// ============================================================================
// Language Configuration Methods (Feature 007)
// ============================================================================

// GetGroupLanguage returns the configured language for a group
func (s *RateLimitStorage) GetGroupLanguage(ctx context.Context, chatID int64) (string, error) {
	language, err := s.store.GetGroupLanguage(ctx, chatID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Group doesn't exist yet, return default language
			return "en", nil
		}
		return "", fmt.Errorf("failed to get group language: %w", err)
	}
	return language, nil
}

// SetGroupLanguage sets the language preference for a group
func (s *RateLimitStorage) SetGroupLanguage(ctx context.Context, chatID int64, language string) error {
	// Ensure group exists first
	if err := s.ensureGroupExists(ctx, chatID); err != nil {
		return fmt.Errorf("failed to ensure group exists: %w", err)
	}

	return s.store.SetGroupLanguage(ctx, sqlc.SetGroupLanguageParams{
		ChatID:   chatID,
		Language: language,
	})
}

// ListAllGroupLanguages returns language preferences for all groups (for cache initialization)
func (s *RateLimitStorage) ListAllGroupLanguages(ctx context.Context) (map[int64]string, error) {
	rows, err := s.store.ListAllGroupLanguages(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list group languages: %w", err)
	}

	languages := make(map[int64]string, len(rows))
	for _, row := range rows {
		languages[row.ChatID] = row.Language
	}
	return languages, nil
}

// EnsureGroup creates a group record if it doesn't exist
func (s *RateLimitStorage) EnsureGroup(ctx context.Context, chatID int64) error {
	return s.store.EnsureGroup(ctx, chatID)
}

// GetUserLanguage returns the configured language for a user
// Returns nil if user has no language preference set
func (s *RateLimitStorage) GetUserLanguage(ctx context.Context, userID int64) (*string, error) {
	return s.store.GetUserLanguage(ctx, userID)
}

// SetUserLanguage sets the language preference for a user
func (s *RateLimitStorage) SetUserLanguage(ctx context.Context, userID int64, language string) error {
	return s.store.SetUserLanguage(ctx, sqlc.SetUserLanguageParams{
		UserID:   userID,
		Language: &language,
	})
}

// EnsureGroupWithUsername creates or updates a group record with username (Feature 011)
func (s *RateLimitStorage) EnsureGroupWithUsername(ctx context.Context, chatID int64, username string) error {
	return s.store.EnsureGroupWithUsername(ctx, sqlc.EnsureGroupWithUsernameParams{
		ChatID:   chatID,
		Username: &username,
	})
}

// ============================================================================
// Proactive Member Sync Methods (Feature 011)
// ============================================================================

// UpsertGroupMembership creates or updates a group membership record
func (s *RateLimitStorage) UpsertGroupMembership(ctx context.Context, params UpsertGroupMembershipParams) (GroupMembership, error) {
	sqlcParams := sqlc.UpsertGroupMembershipParams{
		ChatID:          params.ChatID,
		UserID:          params.UserID,
		Status:          params.Status,
		JoinedAt:        pgtype.Timestamptz{Time: params.JoinedAt, Valid: true},
		LeftAt:          pgtype.Timestamptz{},
		IsAdmin:         params.IsAdmin,
		CanSendMessages: params.CanSendMessages,
	}

	if params.LeftAt != nil {
		sqlcParams.LeftAt = pgtype.Timestamptz{Time: *params.LeftAt, Valid: true}
	}

	membership, err := s.store.UpsertGroupMembership(ctx, sqlcParams)
	if err != nil {
		return GroupMembership{}, fmt.Errorf("failed to upsert group membership: %w", err)
	}

	result := GroupMembership{
		ID:              membership.ID,
		ChatID:          membership.ChatID,
		UserID:          membership.UserID,
		Status:          membership.Status,
		JoinedAt:        membership.JoinedAt.Time,
		LeftAt:          nil,
		UpdatedAt:       membership.UpdatedAt.Time,
		IsAdmin:         membership.IsAdmin,
		CanSendMessages: membership.CanSendMessages,
	}

	if membership.LeftAt.Valid {
		result.LeftAt = &membership.LeftAt.Time
	}

	return result, nil
}

// CreateSyncMetadata initializes sync metadata for a new group
func (s *RateLimitStorage) CreateSyncMetadata(ctx context.Context, chatID int64) (SyncMetadata, error) {
	metadata, err := s.store.CreateSyncMetadata(ctx, chatID)
	if err != nil {
		return SyncMetadata{}, fmt.Errorf("failed to create sync metadata: %w", err)
	}

	result := SyncMetadata{
		ID:             metadata.ID,
		ChatID:         metadata.ChatID,
		SyncStatus:     metadata.SyncStatus,
		LastSyncAt:     nil,
		NextSyncAt:     nil,
		CreatedAt:      metadata.CreatedAt.Time,
		UpdatedAt:      metadata.UpdatedAt.Time,
		TotalMembers:   0,
		FailedAttempts: 0,
		LastError:      nil,
	}

	if metadata.LastSyncAt.Valid {
		result.LastSyncAt = &metadata.LastSyncAt.Time
	}
	if metadata.NextSyncAt.Valid {
		result.NextSyncAt = &metadata.NextSyncAt.Time
	}
	if metadata.TotalMembers != nil {
		result.TotalMembers = int(*metadata.TotalMembers)
	}
	if metadata.FailedAttempts != nil {
		result.FailedAttempts = int(*metadata.FailedAttempts)
	}
	if metadata.LastError != nil {
		result.LastError = metadata.LastError
	}

	return result, nil
}

// UpdateSyncMetadata updates sync metadata after a sync operation
func (s *RateLimitStorage) UpdateSyncMetadata(ctx context.Context, params UpdateSyncMetadataParams) error {
	var totalMembers *int32
	if params.TotalMembers > 0 {
		val := int32(params.TotalMembers)
		totalMembers = &val
	}

	var failedAttempts *int32
	if params.FailedAttempts > 0 {
		val := int32(params.FailedAttempts)
		failedAttempts = &val
	}

	sqlcParams := sqlc.UpdateSyncMetadataParams{
		ChatID:         params.ChatID,
		SyncStatus:     params.SyncStatus,
		LastSyncAt:     pgtype.Timestamptz{},
		NextSyncAt:     pgtype.Timestamptz{},
		TotalMembers:   totalMembers,
		FailedAttempts: failedAttempts,
		LastError:      params.LastError,
	}

	if params.LastSyncAt != nil {
		sqlcParams.LastSyncAt = pgtype.Timestamptz{Time: *params.LastSyncAt, Valid: true}
	}
	if params.NextSyncAt != nil {
		sqlcParams.NextSyncAt = pgtype.Timestamptz{Time: *params.NextSyncAt, Valid: true}
	}

	return s.store.UpdateSyncMetadata(ctx, sqlcParams)
}

// RecordSyncEvent creates an audit trail entry for a sync operation
func (s *RateLimitStorage) RecordSyncEvent(ctx context.Context, params RecordSyncEventParams) (SyncEvent, error) {
	var membersProcessed, membersAdded, membersUpdated, membersRemoved *int32

	if params.MembersProcessed > 0 {
		val := int32(params.MembersProcessed)
		membersProcessed = &val
	}
	if params.MembersAdded > 0 {
		val := int32(params.MembersAdded)
		membersAdded = &val
	}
	if params.MembersUpdated > 0 {
		val := int32(params.MembersUpdated)
		membersUpdated = &val
	}
	if params.MembersRemoved > 0 {
		val := int32(params.MembersRemoved)
		membersRemoved = &val
	}

	sqlcParams := sqlc.RecordSyncEventParams{
		MetadataID:       params.MetadataID,
		EventType:        params.EventType,
		StartedAt:        pgtype.Timestamptz{Time: params.StartedAt, Valid: true},
		CompletedAt:      pgtype.Timestamptz{},
		Status:           params.Status,
		ErrorMessage:     params.ErrorMessage,
		MembersProcessed: membersProcessed,
		MembersAdded:     membersAdded,
		MembersUpdated:   membersUpdated,
		MembersRemoved:   membersRemoved,
	}

	if params.CompletedAt != nil {
		sqlcParams.CompletedAt = pgtype.Timestamptz{Time: *params.CompletedAt, Valid: true}
	}

	event, err := s.store.RecordSyncEvent(ctx, sqlcParams)
	if err != nil {
		return SyncEvent{}, fmt.Errorf("failed to record sync event: %w", err)
	}

	result := SyncEvent{
		ID:               event.ID,
		MetadataID:       event.MetadataID,
		EventType:        event.EventType,
		StartedAt:        event.StartedAt.Time,
		CompletedAt:      nil,
		Status:           event.Status,
		ErrorMessage:     event.ErrorMessage,
		MembersProcessed: 0,
		MembersAdded:     0,
		MembersUpdated:   0,
		MembersRemoved:   0,
	}

	if event.CompletedAt.Valid {
		result.CompletedAt = &event.CompletedAt.Time
	}
	if event.MembersProcessed != nil {
		result.MembersProcessed = int(*event.MembersProcessed)
	}
	if event.MembersAdded != nil {
		result.MembersAdded = int(*event.MembersAdded)
	}
	if event.MembersUpdated != nil {
		result.MembersUpdated = int(*event.MembersUpdated)
	}
	if event.MembersRemoved != nil {
		result.MembersRemoved = int(*event.MembersRemoved)
	}

	return result, nil
}

// GetGroupsNeedingSync retrieves groups that need periodic sync (next_sync_at <= NOW)
func (s *RateLimitStorage) GetGroupsNeedingSync(ctx context.Context) ([]SyncMetadata, error) {
	rows, err := s.store.GetGroupsNeedingSync(ctx, 1000) // Limit to 1000 groups per iteration
	if err != nil {
		return nil, fmt.Errorf("failed to get groups needing sync: %w", err)
	}

	results := make([]SyncMetadata, 0, len(rows))
	for _, row := range rows {
		metadata := SyncMetadata{
			ID:             row.ID,
			ChatID:         row.ChatID,
			SyncStatus:     row.SyncStatus,
			LastSyncAt:     nil,
			NextSyncAt:     nil,
			CreatedAt:      row.CreatedAt.Time,
			UpdatedAt:      row.UpdatedAt.Time,
			TotalMembers:   0,
			FailedAttempts: 0,
			LastError:      nil,
		}

		if row.LastSyncAt.Valid {
			metadata.LastSyncAt = &row.LastSyncAt.Time
		}
		if row.NextSyncAt.Valid {
			metadata.NextSyncAt = &row.NextSyncAt.Time
		}
		if row.TotalMembers != nil {
			metadata.TotalMembers = int(*row.TotalMembers)
		}
		if row.FailedAttempts != nil {
			metadata.FailedAttempts = int(*row.FailedAttempts)
		}
		if row.LastError != nil {
			metadata.LastError = row.LastError
		}

		results = append(results, metadata)
	}

	return results, nil
}

// GetStaleGroupMemberships retrieves memberships that haven't been updated recently
func (s *RateLimitStorage) GetStaleGroupMemberships(ctx context.Context, chatID int64, staleDuration int32) ([]GroupMembership, error) {
	// Note: Current SQLC query doesn't support parameters, returns all stale memberships
	// We filter by chatID in memory for now
	rows, err := s.store.GetStaleGroupMemberships(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get stale group memberships: %w", err)
	}

	results := make([]GroupMembership, 0)
	for _, row := range rows {
		// Filter by chat_id
		if row.ChatID != chatID {
			continue
		}

		membership := GroupMembership{
			ID:              row.ID,
			ChatID:          row.ChatID,
			UserID:          row.UserID,
			Status:          row.Status,
			JoinedAt:        row.JoinedAt.Time,
			LeftAt:          nil,
			IsAdmin:         row.IsAdmin,
			CanSendMessages: row.CanSendMessages,
			UpdatedAt:       row.UpdatedAt.Time,
		}

		if row.LeftAt.Valid {
			membership.LeftAt = &row.LeftAt.Time
		}

		results = append(results, membership)
	}

	return results, nil
}

// UpsertGroupMembershipParams holds parameters for creating/updating a membership
type UpsertGroupMembershipParams struct {
	ChatID          int64
	UserID          int64
	Status          string
	JoinedAt        time.Time
	LeftAt          *time.Time
	IsAdmin         bool
	CanSendMessages bool
}

// UpdateSyncMetadataParams holds parameters for updating sync metadata
type UpdateSyncMetadataParams struct {
	ChatID         int64
	SyncStatus     string
	LastSyncAt     *time.Time
	NextSyncAt     *time.Time
	TotalMembers   int
	FailedAttempts int
	LastError      *string
}

// RecordSyncEventParams holds parameters for recording a sync event
type RecordSyncEventParams struct {
	MetadataID       int64
	EventType        string
	StartedAt        time.Time
	CompletedAt      *time.Time
	Status           string
	ErrorMessage     *string
	MembersProcessed int
	MembersAdded     int
	MembersUpdated   int
	MembersRemoved   int
}
