package ratelimit

import (
	"context"
	"time"
)

// Exemption represents an exemption from rate limiting
type Exemption struct {
	ID      int64
	ChatID  int64
	UserID  *int64
	Role    *string
	AddedBy int64
	AddedAt time.Time
}

// ============================================================================
// Multi-Window Rate Limiting Types (Feature 006)
// ============================================================================

// WindowSlot represents the configuration for one of three window slots (A, B, C)
type WindowSlot struct {
	ID             int
	ChatID         int64
	SlotID         string // 'a', 'b', or 'c'
	CharLimit      int
	DurationValue  int    // Numeric part of duration (30, 2, 7)
	DurationUnit   string // 'minute', 'hour', or 'day'
	WindowDuration int    // Duration in seconds (calculated from value × unit)
	Enabled        bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// ViolatedWindow contains details about a specific window violation
type ViolatedWindow struct {
	SlotID        string
	Limit         int
	CurrentUse    int
	DurationValue int
	DurationUnit  string
	Percentage    int
}

// MultiWindowStorage provides all storage operations for multi-window rate limiting
// This is now the ONLY storage interface - legacy Storage interface removed
type MultiWindowStorage interface {
	// User management (username harvesting and resolution)
	EnsureUser(ctx context.Context, userID int64, username string) error
	GetUserByUsername(ctx context.Context, username string) (int64, error)
	GetUsernameByUserID(ctx context.Context, userID int64) (string, error)

	// Group management (unified metadata harvesting)
	GetGroupByUsername(ctx context.Context, username string) (int64, error)
	EnsureGroupMetadata(ctx context.Context, chatID int64, username, title string) error // Feature 014: unified metadata upsert

	// Group resolution (Feature 014: unified resolution with conflict detection)
	ResolveUsername(username string) (int64, error)
	ResolveTitle(title string) ([]int64, error)
	GetGroupInfo(chatID int64) (username, title *string, err error)

	// Window slot configuration
	CreateDefaultWindows(ctx context.Context, chatID int64) error
	GetEnabledWindows(ctx context.Context, chatID int64) ([]*WindowSlot, error)
	GetAllWindows(ctx context.Context, chatID int64) ([]*WindowSlot, error)
	GetWindowSlot(ctx context.Context, chatID int64, slotID string) (*WindowSlot, error)
	UpdateWindowSlot(ctx context.Context, chatID int64, slotID string, charLimit int, durationValue int, durationUnit string, windowDuration int, enabled bool) error
	SetWindowEnabled(ctx context.Context, chatID int64, slotID string, enabled bool) error

	// Convenience wrappers for command handlers
	SetWindowSlot(ctx context.Context, chatID int64, slotID string, charLimit int, durationValue int, durationUnit string, windowDuration int, enabled bool) error
	DisableWindow(ctx context.Context, chatID int64, slotID string) error
	EnableWindow(ctx context.Context, chatID int64, slotID string) error

	// Simple messages-based rate limiting (single message log shared across all windows)
	RecordMessage(ctx context.Context, userID, chatID int64, charCount int) error
	GetWindowUsage(ctx context.Context, userID, chatID int64, windowSeconds int) (int, error)
	CleanupOldMessages(ctx context.Context, retentionSeconds int) error
	ResetWindowMessages(ctx context.Context, userID, chatID int64) error
	ResetWindowMessagesForAll(ctx context.Context, chatID int64) error
	GetUserWindowStats(ctx context.Context, userID, chatID int64) ([]*UserWindowStat, error)

	// REMOVED: Window-specific exemptions (exemptions table removed in migration 000007)
	// REMOVED: IsUserExemptFromWindow - use GetUserOverride() for user exemptions instead

	// Group pause/resume (blacklist - temporary disable all rate limiting)
	IsGroupPaused(ctx context.Context, chatID int64) (bool, error)
	GetGroupPausedUntil(ctx context.Context, chatID int64) (*time.Time, error)
	SetGroupPaused(ctx context.Context, chatID int64, paused bool, resumeAt *time.Time) error

	// Manual user overrides (3-state: nil=follow rate limiter, true=whitelist, false=blacklist)
	GetUserOverride(ctx context.Context, chatID, userID int64) (*bool, error)
	SetUserOverride(ctx context.Context, chatID, userID int64, overrideState *bool, reason string, createdBy int64, expiresAt *time.Time) error
	RemoveUserOverride(ctx context.Context, chatID, userID int64) error
	GetAllOverrides(ctx context.Context, chatID int64) ([]*UserOverride, error)
	CleanupExpiredOverrides(ctx context.Context) error

	// Language configuration (Feature 007)
	GetGroupLanguage(ctx context.Context, chatID int64) (string, error)
	SetGroupLanguage(ctx context.Context, chatID int64, language string) error
	ListAllGroupLanguages(ctx context.Context) (map[int64]string, error)
	EnsureGroup(ctx context.Context, chatID int64) error

	// User language configuration (Feature 009)
	GetUserLanguage(ctx context.Context, userID int64) (*string, error)
	SetUserLanguage(ctx context.Context, userID int64, language string) error

	// Group management
	GetAllGroups(ctx context.Context) ([]int64, error)

	// MyGroups command queries (Feature 013)
	GetUserGroupsWithRole(ctx context.Context, userID int64, limit, offset int32) ([]UserGroupMembership, error)
	CountUserGroups(ctx context.Context, userID int64) (int64, error)
}

// UserWindowStat represents per-window usage statistics for /mystatus command
type UserWindowStat struct {
	SlotID         string
	CharLimit      int
	DurationValue  int
	DurationUnit   string
	WindowDuration int
	Enabled        bool
	CurrentUsage   int
}

// UserOverride represents a manual override for a user (3-state control)
type UserOverride struct {
	UserID        int64
	OverrideState *bool // nil=normal, true=whitelist, false=blacklist
	Reason        *string
	CreatedAt     time.Time
	CreatedBy     int64
	ExpiresAt     *time.Time
}

// UserGroupMembership represents a group where user is a member (Feature 013)
type UserGroupMembership struct {
	ChatID   int64
	Title    *string // May be NULL if group name not yet captured
	Username *string // May be NULL for private groups (Feature 014: enhanced display)
	IsAdmin  bool
	JoinedAt time.Time
}
