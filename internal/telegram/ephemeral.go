// Package telegram provides Telegram Bot API client wrapper and helper functions.
package telegram

import (
	"context"
	"fmt"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"
)

// EphemeralMessageConfig configures ephemeral message behavior
type EphemeralMessageConfig struct {
	// DeleteAfter is the duration to wait before deleting the message
	// Default: 7 seconds (middle of 5-10s range per research.md)
	DeleteAfter time.Duration

	// Logger for logging deletion errors (optional)
	Logger *zap.Logger
}

// DefaultEphemeralConfig returns the default ephemeral message configuration
func DefaultEphemeralConfig() *EphemeralMessageConfig {
	return &EphemeralMessageConfig{
		DeleteAfter: 7 * time.Second,
		Logger:      nil, // Will use default logger if not set
	}
}

// SendEphemeralMessage sends a message that will be automatically deleted after a delay.
// This is a workaround for Telegram's lack of native ephemeral message support.
// The message is sent immediately, then a goroutine is spawned to delete it after the delay.
//
// Pattern: send → goroutine sleep → DeleteMessage (per research.md)
//
// Parameters:
//   - chatID: The chat to send the message to
//   - text: The message text
//   - config: Optional configuration (uses default if nil)
//
// Returns the sent message and any error from sending (deletion errors are logged, not returned)
func (c *BotClient) SendEphemeralMessage(chatID int64, text string, config *EphemeralMessageConfig) (tgbotapi.Message, error) {
	// Use default config if none provided
	if config == nil {
		config = DefaultEphemeralConfig()
	}

	// Send the message
	ctx := context.Background()
	if err := c.rateLimiter.Wait(ctx); err != nil {
		return tgbotapi.Message{}, fmt.Errorf("rate limiter error: %w", err)
	}

	msg := tgbotapi.NewMessage(chatID, text)
	sentMsg, err := c.bot.Send(msg)
	if err != nil {
		return tgbotapi.Message{}, fmt.Errorf("failed to send ephemeral message: %w", err)
	}

	// Spawn goroutine to delete message after delay
	go c.deleteMessageAfterDelay(chatID, sentMsg.MessageID, config)

	return sentMsg, nil
}

// SendEphemeralReply sends a reply to a specific message that will be automatically deleted
func (c *BotClient) SendEphemeralReply(chatID int64, replyToMessageID int, text string, config *EphemeralMessageConfig) (tgbotapi.Message, error) {
	// Use default config if none provided
	if config == nil {
		config = DefaultEphemeralConfig()
	}

	// Send the reply
	ctx := context.Background()
	if err := c.rateLimiter.Wait(ctx); err != nil {
		return tgbotapi.Message{}, fmt.Errorf("rate limiter error: %w", err)
	}

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyToMessageID = replyToMessageID
	sentMsg, err := c.bot.Send(msg)
	if err != nil {
		return tgbotapi.Message{}, fmt.Errorf("failed to send ephemeral reply: %w", err)
	}

	// Spawn goroutine to delete message after delay
	go c.deleteMessageAfterDelay(chatID, sentMsg.MessageID, config)

	return sentMsg, nil
}

// deleteMessageAfterDelay waits for the specified duration then deletes the message.
// This runs in a goroutine and does not block the caller.
// Deletion errors are logged but not returned (acceptable per research.md - message will remain visible)
func (c *BotClient) deleteMessageAfterDelay(chatID int64, messageID int, config *EphemeralMessageConfig) {
	// Wait for the configured delay
	time.Sleep(config.DeleteAfter)

	// Delete the message
	deleteMsg := tgbotapi.NewDeleteMessage(chatID, messageID)

	// Use a context with timeout for deletion
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := c.rateLimiter.Wait(ctx); err != nil {
		c.logDeletionError(chatID, messageID, fmt.Errorf("rate limiter error: %w", err), config.Logger)
		return
	}

	if _, err := c.bot.Request(deleteMsg); err != nil {
		c.logDeletionError(chatID, messageID, err, config.Logger)
	}
}

// logDeletionError logs an error that occurred during message deletion.
// Uses provided logger if available, otherwise falls back to standard log.
func (c *BotClient) logDeletionError(chatID int64, messageID int, err error, logger *zap.Logger) {
	if logger != nil {
		logger.Warn("failed to delete ephemeral message",
			zap.Int64("chat_id", chatID),
			zap.Int("message_id", messageID),
			zap.Error(err),
		)
	} else {
		// Fallback to standard logging if zap logger not provided
		fmt.Printf("Warning: failed to delete ephemeral message (chat: %d, msg: %d): %v\n", chatID, messageID, err)
	}
}

// SendMultiLineEphemeral sends a multi-line ephemeral message with Markdown support
func (c *BotClient) SendMultiLineEphemeral(chatID int64, lines []string, config *EphemeralMessageConfig) (tgbotapi.Message, error) {
	// Use default config if none provided
	if config == nil {
		config = DefaultEphemeralConfig()
	}

	// Build multi-line text
	text := ""
	for i, line := range lines {
		text += line
		if i < len(lines)-1 {
			text += "\n"
		}
	}

	// Send with Markdown parsing
	ctx := context.Background()
	if err := c.rateLimiter.Wait(ctx); err != nil {
		return tgbotapi.Message{}, fmt.Errorf("rate limiter error: %w", err)
	}

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "Markdown"
	sentMsg, err := c.bot.Send(msg)
	if err != nil {
		return tgbotapi.Message{}, fmt.Errorf("failed to send multi-line ephemeral message: %w", err)
	}

	// Spawn goroutine to delete message after delay
	go c.deleteMessageAfterDelay(chatID, sentMsg.MessageID, config)

	return sentMsg, nil
}
