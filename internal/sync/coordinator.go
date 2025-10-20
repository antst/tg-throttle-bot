// Package sync provides proactive member synchronization for Telegram groups.
package sync

import (
	"context"
	"fmt"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"

	"github.com/antst/tg-throttle-bot/internal/storage"
	"github.com/antst/tg-throttle-bot/internal/telegram"
)

// Sync event types (constants for audit trail)
const (
	SyncEventInitial   = "initial_sync"           // Bot added to group
	SyncEventPeriodic  = "periodic_sync"          // 24h background sync
	SyncEventManual    = "notification_processed" // Admin-triggered sync / member notification
	SyncEventCompleted = "completed"              // Sync finished successfully
	SyncEventFailed    = "failed"                 // Sync encountered errors
)

// SyncCoordinator orchestrates proactive member synchronization operations.
type SyncCoordinator struct {
	client telegram.Client
	store  SyncStorage
	logger *zap.Logger
}

// SyncStorage defines the storage interface needed for sync operations.
type SyncStorage interface {
	// Group operations
	EnsureGroup(ctx context.Context, chatID int64) error

	// User operations
	EnsureUser(ctx context.Context, userID int64, username string) error

	// Membership operations
	UpsertGroupMembership(ctx context.Context, params storage.UpsertGroupMembershipParams) (storage.GroupMembership, error)

	// Sync metadata operations
	CreateSyncMetadata(ctx context.Context, chatID int64) (storage.SyncMetadata, error)
	UpdateSyncMetadata(ctx context.Context, params storage.UpdateSyncMetadataParams) error

	// Sync events
	RecordSyncEvent(ctx context.Context, params storage.RecordSyncEventParams) (storage.SyncEvent, error)
}

// NewSyncCoordinator creates a new sync coordinator instance.
func NewSyncCoordinator(client telegram.Client, store SyncStorage, logger *zap.Logger) *SyncCoordinator {
	return &SyncCoordinator{
		client: client,
		store:  store,
		logger: logger,
	}
}

// InitialGroupSync performs initial sync when bot is added to a group.
// Fetches admin list and stores membership records.
func (sc *SyncCoordinator) InitialGroupSync(ctx context.Context, chatID int64) error {
	startTime := time.Now()
	sc.logger.Info("Starting initial group sync",
		zap.Int64("chat_id", chatID),
	)

	// Create sync metadata first
	metadata, err := sc.store.CreateSyncMetadata(ctx, chatID)
	if err != nil {
		sc.logger.Error("Failed to create sync metadata",
			zap.Int64("chat_id", chatID),
			zap.Error(err),
		)
		MemberSyncFailures.WithLabelValues(SyncTypeInitial, FailureReasonOther).Inc()
		return fmt.Errorf("failed to create sync metadata: %w", err)
	}

	// Fetch administrators with retry logic for rate limits
	var admins []tgbotapi.ChatMember
	err = RetryWithBackoff(ctx, func() error {
		var fetchErr error
		admins, fetchErr = sc.client.GetChatAdministrators(ctx, chatID)
		if fetchErr != nil {
			// Check if it's a rate limit error (429)
			if isRateLimitError(fetchErr) {
				return NewRetryableError(fetchErr, extractRetryAfter(fetchErr))
			}
			// Check if it's a network error (should retry)
			if isNetworkError(fetchErr) {
				return NewRetryableError(fetchErr, 0)
			}
			// Non-retryable error
			return fetchErr
		}
		return nil
	})

	if err != nil {
		sc.logger.Error("Failed to get chat administrators",
			zap.Int64("chat_id", chatID),
			zap.Error(err),
		)

		// Record failed sync event
		_ = sc.recordSyncEvent(ctx, metadata.ID, SyncEventInitial, startTime, err, 0, 0, 0, 0)

		failureReason := categorizeFailureReason(err)
		MemberSyncFailures.WithLabelValues(SyncTypeInitial, failureReason).Inc()
		return fmt.Errorf("failed to get chat administrators: %w", err)
	}

	sc.logger.Info("Fetched administrators",
		zap.Int64("chat_id", chatID),
		zap.Int("count", len(admins)),
	)

	// Store admin memberships
	membersAdded := 0
	for _, admin := range admins {
		// Ensure user record exists
		if err := sc.store.EnsureUser(ctx, int64(admin.User.ID), admin.User.UserName); err != nil {
			sc.logger.Warn("Failed to ensure user",
				zap.Int64("user_id", int64(admin.User.ID)),
				zap.Error(err),
			)
			continue
		}

		// Create membership record
		isAdmin := admin.IsAdministrator() || admin.IsCreator()
		_, err := sc.store.UpsertGroupMembership(ctx, storage.UpsertGroupMembershipParams{
			ChatID:          chatID,
			UserID:          int64(admin.User.ID),
			Status:          storage.MembershipActive,
			JoinedAt:        time.Now(),
			LeftAt:          nil,
			IsAdmin:         isAdmin,
			CanSendMessages: true,
		})
		if err != nil {
			sc.logger.Warn("Failed to upsert membership",
				zap.Int64("chat_id", chatID),
				zap.Int64("user_id", int64(admin.User.ID)),
				zap.Error(err),
			)
			continue
		}
		membersAdded++
	}

	// Record successful sync event
	if err := sc.recordSyncEvent(ctx, metadata.ID, SyncEventInitial, startTime, nil, membersAdded, membersAdded, 0, 0); err != nil {
		sc.logger.Warn("Failed to record sync event", zap.Error(err))
	}

	// Update sync metadata with final statistics
	now := time.Now()
	nextSync := now.Add(24 * time.Hour)
	if err := sc.store.UpdateSyncMetadata(ctx, storage.UpdateSyncMetadataParams{
		ChatID:         chatID,
		SyncStatus:     storage.SyncCompleted,
		LastSyncAt:     &now,
		NextSyncAt:     &nextSync,
		TotalMembers:   membersAdded,
		FailedAttempts: 0,
		LastError:      nil,
	}); err != nil {
		sc.logger.Warn("Failed to update sync metadata", zap.Error(err))
	}

	// Update metrics
	MemberSyncTotal.WithLabelValues(SyncTypeInitial, SyncStatusSuccess).Inc()

	sc.logger.Info("Initial group sync completed",
		zap.Int64("chat_id", chatID),
		zap.Int("members_added", membersAdded),
		zap.Duration("duration", time.Since(startTime)),
	)

	return nil
}

// recordSyncEvent records a sync event in the database with member statistics.
func (sc *SyncCoordinator) recordSyncEvent(ctx context.Context, metadataID int64, eventType string, startTime time.Time, err error, membersProcessed, membersAdded, membersUpdated, membersRemoved int) error {
	status := SyncEventCompleted
	var errorMsg *string
	completedAt := time.Now()

	if err != nil {
		status = SyncEventFailed
		errStr := err.Error()
		errorMsg = &errStr
	}

	_, recordErr := sc.store.RecordSyncEvent(ctx, storage.RecordSyncEventParams{
		MetadataID:       metadataID,
		EventType:        eventType,
		StartedAt:        startTime,
		CompletedAt:      &completedAt,
		Status:           status,
		ErrorMessage:     errorMsg,
		MembersProcessed: membersProcessed,
		MembersAdded:     membersAdded,
		MembersUpdated:   membersUpdated,
		MembersRemoved:   membersRemoved,
	})

	return recordErr
}

// ProcessMemberJoin handles a member joining a group (Phase 4 - User Story 2).
func (sc *SyncCoordinator) ProcessMemberJoin(ctx context.Context, chatID int64, member tgbotapi.ChatMember) error {
	userID := int64(member.User.ID)
	isAdmin := member.IsAdministrator() || member.IsCreator()

	sc.logger.Debug("Processing member join",
		zap.Int64("chat_id", chatID),
		zap.Int64("user_id", userID),
		zap.Bool("is_admin", isAdmin),
	)

	// Ensure user record exists
	if err := sc.store.EnsureUser(ctx, userID, member.User.UserName); err != nil {
		sc.logger.Warn("Failed to ensure user", zap.Error(err))
	}

	// Create or update membership record
	_, err := sc.store.UpsertGroupMembership(ctx, storage.UpsertGroupMembershipParams{
		ChatID:          chatID,
		UserID:          userID,
		Status:          storage.MembershipActive,
		JoinedAt:        time.Now(),
		LeftAt:          nil,
		IsAdmin:         isAdmin,
		CanSendMessages: true,
	})

	if err != nil {
		MemberSyncTotal.WithLabelValues(SyncTypeNotification, SyncStatusFailure).Inc()
		return fmt.Errorf("failed to record member join: %w", err)
	}

	MemberSyncTotal.WithLabelValues(SyncTypeNotification, SyncStatusSuccess).Inc()
	sc.logger.Info("Member join processed", zap.Int64("chat_id", chatID), zap.Int64("user_id", userID))
	return nil
}

// ProcessMemberLeave handles a member leaving a group (Phase 4 - User Story 2).
func (sc *SyncCoordinator) ProcessMemberLeave(ctx context.Context, chatID int64, userID int64) error {
	sc.logger.Debug("Processing member leave",
		zap.Int64("chat_id", chatID),
		zap.Int64("user_id", userID),
	)

	now := time.Now()
	_, err := sc.store.UpsertGroupMembership(ctx, storage.UpsertGroupMembershipParams{
		ChatID:          chatID,
		UserID:          userID,
		Status:          storage.MembershipLeft,
		JoinedAt:        time.Now(), // Will be preserved by DB if exists
		LeftAt:          &now,
		IsAdmin:         false,
		CanSendMessages: false,
	})

	if err != nil {
		MemberSyncTotal.WithLabelValues(SyncTypeNotification, SyncStatusFailure).Inc()
		return fmt.Errorf("failed to record member leave: %w", err)
	}

	MemberSyncTotal.WithLabelValues(SyncTypeNotification, SyncStatusSuccess).Inc()
	sc.logger.Info("Member leave processed", zap.Int64("chat_id", chatID), zap.Int64("user_id", userID))
	return nil
}

// ProcessMemberKicked handles a member being kicked/banned from a group (Phase 4 - User Story 2).
func (sc *SyncCoordinator) ProcessMemberKicked(ctx context.Context, chatID int64, userID int64, reason string) error {
	sc.logger.Debug("Processing member kick/ban",
		zap.Int64("chat_id", chatID),
		zap.Int64("user_id", userID),
		zap.String("reason", reason),
	)

	status := storage.MembershipKicked
	if reason == "banned" {
		status = storage.MembershipBanned
	}

	now := time.Now()
	_, err := sc.store.UpsertGroupMembership(ctx, storage.UpsertGroupMembershipParams{
		ChatID:          chatID,
		UserID:          userID,
		Status:          status,
		JoinedAt:        time.Now(), // Will be preserved by DB if exists
		LeftAt:          &now,
		IsAdmin:         false,
		CanSendMessages: false,
	})

	if err != nil {
		MemberSyncTotal.WithLabelValues(SyncTypeNotification, SyncStatusFailure).Inc()
		return fmt.Errorf("failed to record member kick/ban: %w", err)
	}

	MemberSyncTotal.WithLabelValues(SyncTypeNotification, SyncStatusSuccess).Inc()
	sc.logger.Info("Member kick/ban processed",
		zap.Int64("chat_id", chatID),
		zap.Int64("user_id", userID),
		zap.String("status", status),
	)
	return nil
}

// ProcessMemberRoleChange handles a member's role change (promotion/demotion) (Phase 4 - User Story 2).
func (sc *SyncCoordinator) ProcessMemberRoleChange(ctx context.Context, chatID int64, member tgbotapi.ChatMember) error {
	userID := int64(member.User.ID)
	isAdmin := member.IsAdministrator() || member.IsCreator()

	sc.logger.Debug("Processing member role change",
		zap.Int64("chat_id", chatID),
		zap.Int64("user_id", userID),
		zap.Bool("is_admin", isAdmin),
	)

	// Update membership record with new role
	_, err := sc.store.UpsertGroupMembership(ctx, storage.UpsertGroupMembershipParams{
		ChatID:          chatID,
		UserID:          userID,
		Status:          storage.MembershipActive,
		JoinedAt:        time.Now(), // Will be preserved by DB if exists
		LeftAt:          nil,
		IsAdmin:         isAdmin,
		CanSendMessages: true,
	})

	if err != nil {
		MemberSyncTotal.WithLabelValues(SyncTypeNotification, SyncStatusFailure).Inc()
		return fmt.Errorf("failed to record role change: %w", err)
	}

	MemberSyncTotal.WithLabelValues(SyncTypeNotification, SyncStatusSuccess).Inc()
	sc.logger.Info("Member role change processed",
		zap.Int64("chat_id", chatID),
		zap.Int64("user_id", userID),
		zap.Bool("is_admin", isAdmin),
	)
	return nil
}

// PeriodicGroupSync performs a full group sync for periodic background sync (Phase 5 - User Story 3).
func (sc *SyncCoordinator) PeriodicGroupSync(ctx context.Context, chatID int64) error {
	startTime := time.Now()

	sc.logger.Info("Starting periodic group sync", zap.Int64("chat_id", chatID))

	// Update sync metadata to in_progress
	if err := sc.store.UpdateSyncMetadata(ctx, storage.UpdateSyncMetadataParams{
		ChatID:         chatID,
		SyncStatus:     storage.SyncInProgress,
		LastSyncAt:     nil,
		NextSyncAt:     nil,
		TotalMembers:   0,
		FailedAttempts: 0,
		LastError:      nil,
	}); err != nil {
		sc.logger.Warn("Failed to update sync metadata to in_progress", zap.Error(err))
	}

	// Fetch all administrators with retry logic for rate limits
	var admins []tgbotapi.ChatMember
	var err error
	err = RetryWithBackoff(ctx, func() error {
		var fetchErr error
		admins, fetchErr = sc.client.GetChatAdministrators(ctx, chatID)
		if fetchErr != nil {
			// Check if it's a rate limit error (429)
			if isRateLimitError(fetchErr) {
				return NewRetryableError(fetchErr, extractRetryAfter(fetchErr))
			}
			// Check if it's a network error (should retry)
			if isNetworkError(fetchErr) {
				return NewRetryableError(fetchErr, 0)
			}
			// Non-retryable error
			return fetchErr
		}
		return nil
	})

	if err != nil {
		sc.logger.Error("Failed to get chat administrators",
			zap.Int64("chat_id", chatID),
			zap.Error(err),
		)

		// Record failure and update metadata
		errMsg := err.Error()
		now := time.Now()
		_ = sc.store.UpdateSyncMetadata(ctx, storage.UpdateSyncMetadataParams{
			ChatID:         chatID,
			SyncStatus:     storage.SyncFailed,
			LastSyncAt:     &now,
			NextSyncAt:     nil, // Will be set by failure handler
			TotalMembers:   0,
			FailedAttempts: 1, // Increment would be better
			LastError:      &errMsg,
		})

		failureReason := categorizeFailureReason(err)
		MemberSyncFailures.WithLabelValues(SyncTypePeriodic, failureReason).Inc()
		return fmt.Errorf("failed to get chat administrators: %w", err)
	}

	sc.logger.Info("Fetched administrators for periodic sync",
		zap.Int64("chat_id", chatID),
		zap.Int("count", len(admins)),
	)

	// Process all administrators
	membersProcessed := 0
	membersAdded := 0

	for _, admin := range admins {
		// Ensure user record exists
		if err := sc.store.EnsureUser(ctx, int64(admin.User.ID), admin.User.UserName); err != nil {
			sc.logger.Warn("Failed to ensure user",
				zap.Int64("user_id", int64(admin.User.ID)),
				zap.Error(err),
			)
			continue
		}

		// Update membership record
		isAdmin := admin.IsAdministrator() || admin.IsCreator()
		_, err := sc.store.UpsertGroupMembership(ctx, storage.UpsertGroupMembershipParams{
			ChatID:          chatID,
			UserID:          int64(admin.User.ID),
			Status:          storage.MembershipActive,
			JoinedAt:        time.Now(),
			LeftAt:          nil,
			IsAdmin:         isAdmin,
			CanSendMessages: true,
		})
		if err != nil {
			sc.logger.Warn("Failed to upsert membership",
				zap.Int64("chat_id", chatID),
				zap.Int64("user_id", int64(admin.User.ID)),
				zap.Error(err),
			)
			continue
		}
		membersProcessed++
		membersAdded++ // Simplified: assumes all are new (could check DB first)
	}

	// Update sync metadata with success
	now := time.Now()
	nextSync := now.Add(24 * time.Hour)
	if err := sc.store.UpdateSyncMetadata(ctx, storage.UpdateSyncMetadataParams{
		ChatID:         chatID,
		SyncStatus:     storage.SyncCompleted,
		LastSyncAt:     &now,
		NextSyncAt:     &nextSync,
		TotalMembers:   membersProcessed,
		FailedAttempts: 0,
		LastError:      nil,
	}); err != nil {
		sc.logger.Warn("Failed to update sync metadata", zap.Error(err))
	}

	// Update metrics
	MemberSyncTotal.WithLabelValues(SyncTypePeriodic, SyncStatusSuccess).Inc()

	sc.logger.Info("Periodic group sync completed",
		zap.Int64("chat_id", chatID),
		zap.Int("members_processed", membersProcessed),
		zap.Duration("duration", time.Since(startTime)),
	)

	return nil
}

// isRateLimitError checks if an error is a 429 rate limit error from Telegram API
func isRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	// Check for common rate limit indicators in error message
	errMsg := strings.ToLower(err.Error())
	return strings.Contains(errMsg, "429") ||
		strings.Contains(errMsg, "too many requests") ||
		strings.Contains(errMsg, "rate limit")
}

// isNetworkError checks if an error is a network-related error
func isNetworkError(err error) bool {
	if err == nil {
		return false
	}
	errMsg := strings.ToLower(err.Error())
	return strings.Contains(errMsg, "connection") ||
		strings.Contains(errMsg, "timeout") ||
		strings.Contains(errMsg, "network") ||
		strings.Contains(errMsg, "dial") ||
		strings.Contains(errMsg, "eof") ||
		strings.Contains(errMsg, "reset by peer")
}

// categorizeFailureReason determines the failure reason from an error
func categorizeFailureReason(err error) string {
	if err == nil {
		return FailureReasonOther
	}

	if isRateLimitError(err) {
		return FailureReasonRateLimit
	}

	if isNetworkError(err) {
		return FailureReasonTimeout
	}

	errMsg := strings.ToLower(err.Error())
	if strings.Contains(errMsg, "forbidden") ||
		strings.Contains(errMsg, "not enough rights") ||
		strings.Contains(errMsg, "chat not found") {
		return FailureReasonPermission
	}

	return FailureReasonOther
}

// extractRetryAfter attempts to extract retry_after duration from error
// Returns 0 if no hint is available
func extractRetryAfter(err error) time.Duration {
	// This is a simplified version - in production you'd parse the actual API response
	// The Telegram API returns retry_after in seconds in the error response
	// For now, return 0 to use exponential backoff
	return 0
}
