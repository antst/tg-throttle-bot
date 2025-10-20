// Package telegram provides Telegram Bot API client wrapper and helper functions.
// It handles bot authentication, user permissions, and message sending.
package telegram

import (
	"context"
	"fmt"
	"log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// BotClient wraps the Telegram Bot API
type BotClient struct {
	bot         *tgbotapi.BotAPI
	rateLimiter *APIRateLimiter
}

// NewClient creates a new Telegram client with the given bot token.
// Debug mode enables detailed API logging.
func NewClient(token string, debug bool) (*BotClient, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot: %w", err)
	}

	bot.Debug = debug
	log.Printf("Authorized on account %s", bot.Self.UserName)

	return &BotClient{
		bot:         bot,
		rateLimiter: NewAPIRateLimiter(),
	}, nil
}

// GetUpdatesChan returns a channel for receiving updates from Telegram.
// Configures a 60-second timeout for long polling.
// Includes my_chat_member and chat_member updates for proactive member tracking (Feature 011).
func (c *BotClient) GetUpdatesChan() tgbotapi.UpdatesChannel {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	u.AllowedUpdates = []string{
		"message",        // Regular messages (existing)
		"my_chat_member", // Bot status changes (Feature 011 - proactive sync)
		"chat_member",    // Member status changes (Feature 011 - proactive sync)
	}

	return c.bot.GetUpdatesChan(u)
}

// GetBotAPI returns the underlying bot API instance.
// Used by worker for direct API access.
func (c *BotClient) GetBotAPI() *tgbotapi.BotAPI {
	return c.bot
}

// DeleteMessage deletes a message from a chat.
func (c *BotClient) DeleteMessage(chatID int64, messageID int) error {
	ctx := context.Background()
	if err := c.rateLimiter.Wait(ctx); err != nil {
		return fmt.Errorf("rate limiter error: %w", err)
	}

	deleteMsg := tgbotapi.NewDeleteMessage(chatID, messageID)
	_, err := c.bot.Request(deleteMsg)
	return err
}

// SendMessage sends a text message to a chat.
func (c *BotClient) SendMessage(chatID int64, text string) error {
	ctx := context.Background()
	if err := c.rateLimiter.Wait(ctx); err != nil {
		return fmt.Errorf("rate limiter error: %w", err)
	}

	msg := tgbotapi.NewMessage(chatID, text)
	_, err := c.bot.Send(msg)
	return err
}

// SendReply sends a reply to a specific message.
func (c *BotClient) SendReply(chatID int64, replyToMessageID int, text string) error {
	ctx := context.Background()
	if err := c.rateLimiter.Wait(ctx); err != nil {
		return fmt.Errorf("rate limiter error: %w", err)
	}

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyToMessageID = replyToMessageID
	_, err := c.bot.Send(msg)
	return err
}

// IsAdmin checks if a user is an admin in a chat (context-aware).
func (c *BotClient) IsAdmin(ctx context.Context, chatID int64, userID int64) (bool, error) {
	if err := c.rateLimiter.Wait(ctx); err != nil {
		return false, fmt.Errorf("rate limiter error: %w", err)
	}

	member, err := c.bot.GetChatMember(
		tgbotapi.GetChatMemberConfig{
			ChatConfigWithUser: tgbotapi.ChatConfigWithUser{
				ChatID: chatID,
				UserID: userID,
			},
		},
	)
	if err != nil {
		return false, err
	}

	return member.IsAdministrator() || member.IsCreator(), nil
}

// IsCreator checks if a user is the creator of a chat.
func (c *BotClient) IsCreator(ctx context.Context, chatID int64, userID int64) (bool, error) {
	if err := c.rateLimiter.Wait(ctx); err != nil {
		return false, fmt.Errorf("rate limiter error: %w", err)
	}

	member, err := c.bot.GetChatMember(
		tgbotapi.GetChatMemberConfig{
			ChatConfigWithUser: tgbotapi.ChatConfigWithUser{
				ChatID: chatID,
				UserID: userID,
			},
		},
	)
	if err != nil {
		return false, err
	}

	return member.IsCreator(), nil
}

// GetUserRoles returns the roles a user has in a chat (e.g., "creator", "admin").
func (c *BotClient) GetUserRoles(ctx context.Context, chatID int64, userID int64) ([]string, error) {
	if err := c.rateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limiter error: %w", err)
	}

	member, err := c.bot.GetChatMember(
		tgbotapi.GetChatMemberConfig{
			ChatConfigWithUser: tgbotapi.ChatConfigWithUser{
				ChatID: chatID,
				UserID: userID,
			},
		},
	)
	if err != nil {
		return nil, err
	}

	var roles []string
	if member.IsCreator() {
		roles = append(roles, "creator")
	}
	if member.IsAdministrator() {
		roles = append(roles, "admin")
	}

	return roles, nil
}

// GetBot returns the underlying bot instance.
func (c *BotClient) GetBot() *tgbotapi.BotAPI {
	return c.bot
}

// GetChatMember gets information about a chat member.
func (c *BotClient) GetChatMember(ctx context.Context, chatID int64, userID int64) (tgbotapi.ChatMember, error) {
	if err := c.rateLimiter.Wait(ctx); err != nil {
		return tgbotapi.ChatMember{}, fmt.Errorf("rate limiter error: %w", err)
	}

	member, err := c.bot.GetChatMember(
		tgbotapi.GetChatMemberConfig{
			ChatConfigWithUser: tgbotapi.ChatConfigWithUser{
				ChatID: chatID,
				UserID: userID,
			},
		},
	)
	if err != nil {
		return tgbotapi.ChatMember{}, fmt.Errorf("failed to get chat member: %w", err)
	}
	return member, nil
}

// RestrictUser restricts a user in a chat (removes send message permissions)
func (c *BotClient) RestrictUser(ctx context.Context, chatID int64, userID int64) error {
	if err := c.rateLimiter.Wait(ctx); err != nil {
		return fmt.Errorf("rate limiter error: %w", err)
	}

	permissions := tgbotapi.ChatPermissions{
		CanSendMessages:       false,
		CanSendMediaMessages:  false,
		CanSendPolls:          false,
		CanSendOtherMessages:  false,
		CanAddWebPagePreviews: false,
	}

	restrictConfig := tgbotapi.RestrictChatMemberConfig{
		ChatMemberConfig: tgbotapi.ChatMemberConfig{
			ChatID: chatID,
			UserID: userID,
		},
		Permissions: &permissions,
	}

	_, err := c.bot.Request(restrictConfig)
	if err != nil {
		return fmt.Errorf("failed to restrict user: %w", err)
	}
	return nil
}

// UnrestrictUser restores a user's permissions in a chat
func (c *BotClient) UnrestrictUser(ctx context.Context, chatID int64, userID int64) error {
	if err := c.rateLimiter.Wait(ctx); err != nil {
		return fmt.Errorf("rate limiter error: %w", err)
	}

	permissions := tgbotapi.ChatPermissions{
		CanSendMessages:       true,
		CanSendMediaMessages:  true,
		CanSendPolls:          true,
		CanSendOtherMessages:  true,
		CanAddWebPagePreviews: true,
	}

	restrictConfig := tgbotapi.RestrictChatMemberConfig{
		ChatMemberConfig: tgbotapi.ChatMemberConfig{
			ChatID: chatID,
			UserID: userID,
		},
		Permissions: &permissions,
	}

	_, err := c.bot.Request(restrictConfig)
	if err != nil {
		return fmt.Errorf("failed to unrestrict user: %w", err)
	}
	return nil
}

// CheckBotPermissions checks if the bot has required permissions in a chat
func (c *BotClient) CheckBotPermissions(ctx context.Context, chatID int64) (
	canRestrict bool, canDelete bool, err error,
) {
	// Get bot's user ID
	botUserID := int64(c.bot.Self.ID)

	// Get bot's chat member info (this already includes rate limiting via GetChatMember)
	member, err := c.GetChatMember(ctx, chatID, botUserID)
	if err != nil {
		return false, false, fmt.Errorf("failed to get bot member info: %w", err)
	}

	// Check if bot is admin
	if !member.IsAdministrator() && !member.IsCreator() {
		return false, false, nil
	}

	// For administrators, check specific permissions
	canRestrict = member.CanRestrictMembers
	canDelete = member.CanDeleteMessages
	return canRestrict, canDelete, nil
}

// GetChatAdministrators gets the list of administrators in a chat.
// Used for initial group sync and periodic member validation.
func (c *BotClient) GetChatAdministrators(ctx context.Context, chatID int64) ([]tgbotapi.ChatMember, error) {
	if err := c.rateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limiter error: %w", err)
	}

	config := tgbotapi.ChatAdministratorsConfig{
		ChatConfig: tgbotapi.ChatConfig{
			ChatID: chatID,
		},
	}

	admins, err := c.bot.GetChatAdministrators(config)
	if err != nil {
		return nil, fmt.Errorf("failed to get chat administrators: %w", err)
	}

	return admins, nil
}

// GetChatMembersCount gets the number of members in a chat.
// Used for tracking group size and estimating sync time.
func (c *BotClient) GetChatMembersCount(ctx context.Context, chatID int64) (int, error) {
	if err := c.rateLimiter.Wait(ctx); err != nil {
		return 0, fmt.Errorf("rate limiter error: %w", err)
	}

	config := tgbotapi.ChatMemberCountConfig{
		ChatConfig: tgbotapi.ChatConfig{
			ChatID: chatID,
		},
	}

	count, err := c.bot.GetChatMembersCount(config)
	if err != nil {
		return 0, fmt.Errorf("failed to get chat members count: %w", err)
	}

	return count, nil
}
