// Package telegram provides Telegram Bot API client wrapper and helper functions.
package telegram

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
)

// PermissionChecker handles bot permission verification
type PermissionChecker struct {
	client Client
	logger *zap.SugaredLogger
}

// NewPermissionChecker creates a new permission checker with the given client and logger.
func NewPermissionChecker(client Client, logger *zap.SugaredLogger) *PermissionChecker {
	return &PermissionChecker{
		client: client,
		logger: logger,
	}
}

// RequiredPermissions represents the permissions needed by the bot.
type RequiredPermissions struct {
	CanRestrictMembers bool
	CanDeleteMessages  bool
}

// PermissionStatus represents the current permission state.
type PermissionStatus struct {
	HasAllPermissions  bool
	MissingPermissions []string
	LastChecked        time.Time
}

// CheckBotPermissions verifies the bot has required permissions in a chat.
// Returns an error if the bot is not an admin or lacks necessary permissions.
func (p *PermissionChecker) CheckBotPermissions(ctx context.Context, chatID int64, botUserID int64) error {
	member, err := p.client.GetChatMember(ctx, chatID, botUserID)
	if err != nil {
		return fmt.Errorf("failed to get bot member info: %w", err)
	}

	// Check if bot is admin or owner
	if !member.IsAdministrator() && !member.IsCreator() {
		return fmt.Errorf("bot is not an administrator in chat %d", chatID)
	}

	// Chat creators/owners have all permissions
	if member.IsCreator() {
		p.logger.Infow("Bot is chat owner with all permissions", "chat_id", chatID)
		return nil
	}

	// For administrators, check specific permissions
	if member.Status != "administrator" {
		return fmt.Errorf("unable to check bot permissions")
	}

	// Check required permissions for administrators
	missingPerms := []string{}

	if !member.CanRestrictMembers {
		missingPerms = append(missingPerms, "restrict_members")
	}

	if !member.CanDeleteMessages {
		missingPerms = append(missingPerms, "delete_messages")
	}

	if len(missingPerms) > 0 {
		return fmt.Errorf("bot missing required permissions: %v", missingPerms)
	}

	p.logger.Infow("Bot has all required permissions", "chat_id", chatID)
	return nil
}

// CheckPermissionsDetailed returns detailed permission status for the bot.
// Includes information about missing permissions and check timestamp.
func (p *PermissionChecker) CheckPermissionsDetailed(
	ctx context.Context, chatID int64, botUserID int64,
) (*PermissionStatus, error) {
	status := &PermissionStatus{
		HasAllPermissions:  false,
		MissingPermissions: []string{},
		LastChecked:        time.Now(),
	}

	member, err := p.client.GetChatMember(ctx, chatID, botUserID)
	if err != nil {
		return nil, fmt.Errorf("failed to get bot member info: %w", err)
	}

	// Check if bot is admin or owner
	if !member.IsAdministrator() && !member.IsCreator() {
		status.MissingPermissions = append(status.MissingPermissions, "admin_status")
		return status, nil
	}

	// Chat creators/owners have all permissions
	if member.IsCreator() {
		status.HasAllPermissions = true
		return status, nil
	}

	// For administrators, check specific permissions
	if member.Status == "administrator" {
		if !member.CanRestrictMembers {
			status.MissingPermissions = append(status.MissingPermissions, "restrict_members")
		}

		if !member.CanDeleteMessages {
			status.MissingPermissions = append(status.MissingPermissions, "delete_messages")
		}
	}

	status.HasAllPermissions = len(status.MissingPermissions) == 0
	return status, nil
}

// PeriodicPermissionChecker manages periodic permission checks.
type PeriodicPermissionChecker struct {
	checker              *PermissionChecker
	client               Client
	logger               *zap.SugaredLogger
	checkInterval        time.Duration
	onPermissionLost     func(chatID int64, missing []string)
	onPermissionRestored func(chatID int64)
}

// NewPeriodicPermissionChecker creates a new periodic checker.
// Callbacks are invoked when permissions are lost or restored.
func NewPeriodicPermissionChecker(
	client Client,
	logger *zap.SugaredLogger,
	checkInterval time.Duration,
	onPermissionLost func(chatID int64, missing []string),
	onPermissionRestored func(chatID int64),
) *PeriodicPermissionChecker {
	return &PeriodicPermissionChecker{
		checker:              NewPermissionChecker(client, logger),
		client:               client,
		logger:               logger,
		checkInterval:        checkInterval,
		onPermissionLost:     onPermissionLost,
		onPermissionRestored: onPermissionRestored,
	}
}

// Start begins periodic permission checking (runs every 5 minutes per FR-021).
// Checks all provided chat IDs and invokes callbacks when permission status changes.
func (p *PeriodicPermissionChecker) Start(ctx context.Context, chatIDs []int64, botUserID int64) {
	ticker := time.NewTicker(p.checkInterval)
	defer ticker.Stop()

	// Track previous status to detect changes
	previousStatus := make(map[int64]bool)

	// Initial check
	p.checkAllChats(ctx, chatIDs, botUserID, previousStatus)

	for {
		select {
		case <-ctx.Done():
			p.logger.Info("Stopping periodic permission checker")
			return
		case <-ticker.C:
			p.checkAllChats(ctx, chatIDs, botUserID, previousStatus)
		}
	}
}

// checkAllChats checks permissions for all configured chats
func (p *PeriodicPermissionChecker) checkAllChats(
	ctx context.Context, chatIDs []int64, botUserID int64, previousStatus map[int64]bool,
) {
	for _, chatID := range chatIDs {
		status, err := p.checker.CheckPermissionsDetailed(ctx, chatID, botUserID)
		if err != nil {
			p.logger.Errorw("Failed to check permissions", "chat_id", chatID, "error", err)
			continue
		}

		hadPermissions, existed := previousStatus[chatID]
		currentHasPermissions := status.HasAllPermissions

		// Detect permission changes
		if existed {
			if hadPermissions && !currentHasPermissions {
				// Permissions lost - trigger degradation (FR-022)
				p.logger.Warnw("Bot permissions lost", "chat_id", chatID, "missing", status.MissingPermissions)
				if p.onPermissionLost != nil {
					p.onPermissionLost(chatID, status.MissingPermissions)
				}
			} else if !hadPermissions && currentHasPermissions {
				// Permissions restored - re-enable features (FR-023)
				p.logger.Infow("Bot permissions restored", "chat_id", chatID)
				if p.onPermissionRestored != nil {
					p.onPermissionRestored(chatID)
				}
			}
		}

		previousStatus[chatID] = currentHasPermissions
	}
}

// End of PermissionChecker methods
