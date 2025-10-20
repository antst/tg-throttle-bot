package bot

import (
	"context"
	"fmt"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"
)

// isPrivateChatID validates that a chat ID is for a private chat.
// Telegram private chat IDs are positive integers.
func isPrivateChatID(chatID int64) bool {
	return chatID > 0
}

// isGroupChatID validates that a chat ID is for a group chat.
// Telegram group chat IDs are negative integers (including supergroups with -100 prefix).
func isGroupChatID(chatID int64) bool {
	return chatID < 0
}

// MessageRouter handles routing of bot messages according to the two-category model:
//
// Category #1: Group Notifications - Inform the group about events (policy changes, group-wide announcements)
// Category #2: Private Notifications - Send to user's private chat (warnings, violations, command responses)
//
// Behavior: NO FALLBACK LOGIC. Messages go to their intended destination only.
// - Group messages go to group only (fail if group unavailable)
// - Private messages go to private only (silent failure if private unavailable)
//
// Special case: Policy changes send BOTH a private confirmation AND a group notification separately.
type MessageRouter struct {
	bot    *tgbotapi.BotAPI
	logger *zap.Logger
}

// NewMessageRouter creates a new message router instance.
func NewMessageRouter(bot *tgbotapi.BotAPI, logger *zap.Logger) *MessageRouter {
	return &MessageRouter{
		bot:    bot,
		logger: logger,
	}
}

// SendGroupNotification sends a notification to a group chat (Category #1).
//
// Used for: Rate limit violations, policy changes, group-wide announcements.
// Behavior: Sends directly to the group, no fallback mechanism.
//
// Example: "⚠️ @username exceeded rate limit (1000 messages/30m)"
func (mr *MessageRouter) SendGroupNotification(ctx context.Context, groupID int64, text string) error {
	// Validate: groupID must be negative (group chat)
	if !isGroupChatID(groupID) {
		mr.logger.Error("SendGroupNotification called with non-group chat ID",
			zap.Int64("groupID", groupID),
		)
		return fmt.Errorf("invalid groupID: expected group chat ID (<0), got %d", groupID)
	}

	msg := tgbotapi.NewMessage(groupID, text)
	_, err := mr.bot.Send(msg)
	if err != nil {
		mr.logger.Error("Failed to send group notification",
			zap.Int64("groupID", groupID),
			zap.Error(err),
		)
		return fmt.Errorf("group notification failed: %w", err)
	}
	mr.logger.Debug("Group notification sent",
		zap.Int64("groupID", groupID),
	)
	return nil
}

// SendUserNotification sends a notification to user's private chat (Category #2).
//
// Used for: Warnings, violations, errors, command responses - anything user-specific.
// Behavior: Sends ONLY to private chat. If unavailable, fails silently (no group fallback).
//
// Examples:
//   - "⚠️ Rate Limit Warning (Window A): 80/100 chars (80%)"
//   - "❌ Error: Group '@home' not found in database"
//   - "✅ Window A configured: 1000 characters per 30 minutes"
//
// This method is designed for messages that are ONLY relevant to the user and should
// never appear in the group, even if private delivery fails.
func (mr *MessageRouter) SendUserNotification(ctx context.Context, userID int64, text string) error {
	// Validate: userID must be positive (private chat)
	if !isPrivateChatID(userID) {
		mr.logger.Error("SendUserNotification called with non-private chat ID",
			zap.Int64("userID", userID),
		)
		return fmt.Errorf("invalid userID: expected private chat ID (>0), got %d", userID)
	}

	msg := tgbotapi.NewMessage(userID, text)
	_, err := mr.bot.Send(msg)

	if err != nil {
		if isPrivateChatError(err) {
			// Silent failure: User hasn't started private chat, but we don't pollute group
			mr.logger.Debug("User notification not delivered (private chat unavailable)",
				zap.Int64("userID", userID),
				zap.Error(err),
			)
			return nil // Silent failure as designed
		}
		// Real error (not just unavailable chat)
		mr.logger.Error("Failed to send user notification",
			zap.Int64("userID", userID),
			zap.Error(err),
		)
		return fmt.Errorf("user notification failed: %w", err)
	}

	mr.logger.Debug("User notification sent",
		zap.Int64("userID", userID),
	)
	return nil
}

// SendCommandResponse sends a command response to user's private chat (Category #2).
//
// Used for: Direct answers to user commands (both from group and private chat).
// Behavior: Sends ONLY to private chat. If unavailable, fails silently (no group fallback).
//
// Example: "✅ Window A configured: 1000 messages/30m"
//
// This is a convenience wrapper around SendUserNotification for semantic clarity.
// Command responses are user-specific and should never pollute the group.
func (mr *MessageRouter) SendCommandResponse(ctx context.Context, userID int64, text string) error {
	return mr.SendUserNotification(ctx, userID, text)
}

// SendPolicyChangeMessages sends dual messages for policy-changing commands.
//
// Combines Category #1 (group notification) and Category #2 (private confirmation).
// Used for: setwindow, pause, resume, enablewindow, disablewindow, setlanguage.
//
// Behavior:
// - Sends private confirmation to the user (silent failure if unavailable)
// - Sends group notification to inform all members (fails if group unavailable)
// - Private message uses user's language preference, group uses group's language
// - NO fallback logic: each message goes to its intended destination only
//
// This ensures transparency: the user gets confirmation (if private chat available),
// and the group always sees what changed.
func (mr *MessageRouter) SendPolicyChangeMessages(ctx context.Context, req PolicyChangeRequest) error {
	// 1. Send private confirmation (silent failure if unavailable)
	err := mr.SendUserNotification(ctx, req.UserID, req.PrivateText)
	if err != nil {
		// Real error (not silent failure), log it
		mr.logger.Error("Failed to send private confirmation",
			zap.Int64("userID", req.UserID),
			zap.Error(err),
		)
	}

	// 2. Send group notification (continue even if private failed)
	groupErr := mr.SendGroupNotification(ctx, req.GroupID, req.GroupText)
	if groupErr != nil {
		return groupErr // Already logged in SendGroupNotification
	}

	mr.logger.Info("Policy change messages sent successfully",
		zap.Int64("userID", req.UserID),
		zap.Int64("groupID", req.GroupID),
	)

	return nil
}

// PolicyChangeRequest contains parameters for sending dual messages on policy changes.
type PolicyChangeRequest struct {
	UserID      int64  // Telegram user ID for private message
	GroupID     int64  // Telegram group ID for notification
	PrivateText string // Localized private confirmation text
	GroupText   string // Localized group notification text
}

// isPrivateChatError detects if an error indicates private chat is unavailable.
// Returns true for "chat not found" (user never started chat) or "bot was blocked" errors.
func isPrivateChatError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "chat not found") ||
		strings.Contains(errStr, "bot was blocked") ||
		strings.Contains(errStr, "Forbidden: bot was blocked by the user") ||
		strings.Contains(errStr, "Bad Request: chat not found")
}
