// Package telegram provides Telegram Bot API client wrapper and helper functions.
package telegram

import (
	"context"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Client defines the interface for interacting with Telegram Bot API.
// This interface allows for mocking in tests.
type Client interface {
	// GetUpdatesChan returns a channel for receiving updates
	GetUpdatesChan() tgbotapi.UpdatesChannel

	// GetBotAPI returns the underlying bot API instance
	GetBotAPI() *tgbotapi.BotAPI

	// DeleteMessage deletes a message from a chat
	DeleteMessage(chatID int64, messageID int) error

	// SendMessage sends a text message to a chat
	SendMessage(chatID int64, text string) error

	// SendReply sends a reply to a specific message
	SendReply(chatID int64, replyToMessageID int, text string) error

	// IsAdmin checks if a user is an admin in a chat
	IsAdmin(ctx context.Context, chatID int64, userID int64) (bool, error)

	// IsCreator checks if a user is the creator of a chat
	IsCreator(ctx context.Context, chatID int64, userID int64) (bool, error)

	// GetUserRoles returns the roles a user has in a chat
	GetUserRoles(ctx context.Context, chatID int64, userID int64) ([]string, error)

	// GetBot returns the underlying bot instance
	GetBot() *tgbotapi.BotAPI

	// GetChatMember gets information about a chat member
	GetChatMember(ctx context.Context, chatID int64, userID int64) (tgbotapi.ChatMember, error)

	// SendAdminNotification sends a notification message to chat administrators
	SendAdminNotification(ctx context.Context, chatID int64, text string) error

	// RestrictUser restricts a user in a chat (removes send message permissions)
	RestrictUser(ctx context.Context, chatID int64, userID int64) error

	// UnrestrictUser restores all permissions for a user in a chat
	UnrestrictUser(ctx context.Context, chatID int64, userID int64) error

	// CheckBotPermissions checks if the bot has required permissions
	CheckBotPermissions(ctx context.Context, chatID int64) (canRestrict bool, canDelete bool, err error)

	// SendEphemeralMessage sends a message that will be automatically deleted after a delay
	SendEphemeralMessage(chatID int64, text string, config *EphemeralMessageConfig) (tgbotapi.Message, error)
}
