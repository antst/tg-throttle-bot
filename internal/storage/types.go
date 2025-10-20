package storage

import "time"

// GroupMembership represents a user's membership in a group
type GroupMembership struct {
	ID              int64      `db:"id"`
	ChatID          int64      `db:"chat_id"`
	UserID          int64      `db:"user_id"`
	Status          string     `db:"status"` // active, left, kicked, banned
	JoinedAt        time.Time  `db:"joined_at"`
	LeftAt          *time.Time `db:"left_at"`
	UpdatedAt       time.Time  `db:"updated_at"`
	IsAdmin         bool       `db:"is_admin"`
	CanSendMessages bool       `db:"can_send_messages"`
}

// SyncMetadata tracks sync status for a group
type SyncMetadata struct {
	ID             int64      `db:"id"`
	ChatID         int64      `db:"chat_id"`
	SyncStatus     string     `db:"sync_status"` // pending, in_progress, completed, failed
	LastSyncAt     *time.Time `db:"last_sync_at"`
	NextSyncAt     *time.Time `db:"next_sync_at"`
	CreatedAt      time.Time  `db:"created_at"`
	UpdatedAt      time.Time  `db:"updated_at"`
	TotalMembers   int        `db:"total_members"`
	FailedAttempts int        `db:"failed_attempts"`
	LastError      *string    `db:"last_error"`
}

// SyncEvent is an audit trail entry for a sync operation
type SyncEvent struct {
	ID               int64      `db:"id"`
	MetadataID       int64      `db:"metadata_id"`
	EventType        string     `db:"event_type"` // initial_sync, periodic_sync, notification_processed
	StartedAt        time.Time  `db:"started_at"`
	CompletedAt      *time.Time `db:"completed_at"`
	Status           string     `db:"status"` // started, completed, failed
	ErrorMessage     *string    `db:"error_message"`
	MembersProcessed int        `db:"members_processed"`
	MembersAdded     int        `db:"members_added"`
	MembersUpdated   int        `db:"members_updated"`
	MembersRemoved   int        `db:"members_removed"`
}

// MembershipStatus constants
const (
	MembershipActive = "active"
	MembershipLeft   = "left"
	MembershipKicked = "kicked"
	MembershipBanned = "banned"
)

// SyncStatus constants
const (
	SyncPending    = "pending"
	SyncInProgress = "in_progress"
	SyncCompleted  = "completed"
	SyncFailed     = "failed"
)

// SyncEventType constants
const (
	SyncEventInitial      = "initial_sync"
	SyncEventPeriodic     = "periodic_sync"
	SyncEventNotification = "notification_processed"
)

// SyncEventStatus constants
const (
	SyncEventStarted   = "started"
	SyncEventCompleted = "completed"
	SyncEventFailed    = "failed"
)
