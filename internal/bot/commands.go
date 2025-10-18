// Package bot provides Telegram bot handlers and command processing.
// Clean implementation with ONLY multi-window commands (no legacy code)
package bot

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/antst/tg-throttle-bot/internal/ratelimit"
	"github.com/antst/tg-throttle-bot/internal/telegram"
)

// Constants for repeated strings
const (
	WindowTypeDay             = "day"
	WindowTypeHour            = "hour"
	WindowTypeMinute          = "minute"
	ExemptTypeRole            = "role"
	ExemptTypeUser            = "user"
	ErrUsernameNotImplemented = "❌ Username resolution not yet implemented. Please use user ID."
	ErrInvalidUserID          = "❌ Invalid user ID"
)

// Command represents a parsed bot command
type Command struct {
	Name string
	Args []string
}

// CommandHandler handles all bot commands with clean multi-window implementation
type CommandHandler struct {
	store  ratelimit.MultiWindowStorage
	client telegram.Client
}

// NewCommandHandler creates a new command handler
func NewCommandHandler(store ratelimit.MultiWindowStorage, client telegram.Client) *CommandHandler {
	return &CommandHandler{
		store:  store,
		client: client,
	}
}

// ============================================================================
// Helper Functions
// ============================================================================

// ParseCommand extracts command name and arguments from message text.
func ParseCommand(text string) (*Command, error) {
	text = strings.TrimSpace(text)

	if !strings.HasPrefix(text, "/") {
		return nil, fmt.Errorf("not a command: must start with /")
	}

	// Remove leading slash
	text = text[1:]

	if len(text) == 0 {
		return nil, fmt.Errorf("empty command")
	}

	// Split by whitespace
	parts := strings.Fields(text)
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty command")
	}

	// Handle bot mentions (e.g., /command@botname)
	cmdPart := parts[0]
	if idx := strings.Index(cmdPart, "@"); idx != -1 {
		cmdPart = cmdPart[:idx]
	}

	return &Command{
		Name: strings.ToLower(cmdPart),
		Args: parts[1:],
	}, nil
}

// ParseUserIdentifier parses user identifier (ID or @username)
// Returns user_id (int64) and error if unable to resolve
// Username resolution works via automatic harvesting from messages (Feature 006 User Story 6)
func (h *CommandHandler) ParseUserIdentifier(ctx context.Context, chatID int64, identifier string) (int64, error) {
	identifier = strings.TrimSpace(identifier)

	// Check if it's a username (starts with @)
	if strings.HasPrefix(identifier, "@") {
		username := strings.TrimPrefix(identifier, "@")

		// Validate username format (alphanumeric + underscore, max 32 chars per Telegram spec)
		if len(username) > 32 {
			return 0, fmt.Errorf("invalid username '@%s' - usernames cannot exceed 32 characters", username)
		}
		if username == "" {
			return 0, fmt.Errorf("invalid username - cannot be empty")
		}

		// Resolve username via database (harvested from previous messages)
		userID, err := h.store.GetUserByUsername(ctx, username)
		if err != nil {
			// Provide helpful error message
			return 0, fmt.Errorf("cannot resolve @%s: %w. Note: User must have sent at least one message for username to be cached", username, err)
		}

		return userID, nil
	}

	// Parse as numeric user ID
	userID, err := strconv.ParseInt(identifier, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid user identifier '%s' - must be numeric user ID or @username", identifier)
	}

	return userID, nil
}

// formatUserDisplay formats user display string with username if available
// Returns "@username (user_id)" or just "user_id" if no username
func (h *CommandHandler) formatUserDisplay(ctx context.Context, userID int64) string {
	username, err := h.store.GetUsernameByUserID(ctx, userID)
	if err == nil && username != "" {
		return fmt.Sprintf("@%s (%d)", username, userID)
	}
	return fmt.Sprintf("%d", userID)
}

// ParseDurationString parses duration string like "30m", "2h", "7d"
func ParseDurationString(input string) (int, string, error) {
	input = strings.TrimSpace(input)

	if len(input) == 0 {
		return 0, "", fmt.Errorf("empty duration string")
	}

	var unit string
	var numStr string

	switch {
	case strings.HasSuffix(input, "m"):
		unit = "minute"
		numStr = strings.TrimSuffix(input, "m")
	case strings.HasSuffix(input, "h"):
		unit = "hour"
		numStr = strings.TrimSuffix(input, "h")
	case strings.HasSuffix(input, "d"):
		unit = "day"
		numStr = strings.TrimSuffix(input, "d")
	default:
		return 0, "", fmt.Errorf("invalid duration format: must end with m, h, or d")
	}

	value, err := strconv.Atoi(numStr)
	if err != nil {
		return 0, "", fmt.Errorf("invalid duration value: %w", err)
	}

	if value < 1 || value > 365 {
		return 0, "", fmt.Errorf("duration value must be between 1 and 365")
	}

	return value, unit, nil
}

// FormatWindowDuration formats duration for display
func FormatWindowDuration(value int, unit string) string {
	if value == 1 {
		// Singular
		switch unit {
		case "minute":
			return "1 minute"
		case "hour":
			return "1 hour"
		case "day":
			return "1 day"
		}
	}
	// Plural
	switch unit {
	case "minute":
		return fmt.Sprintf("%d minutes", value)
	case "hour":
		return fmt.Sprintf("%d hours", value)
	case "day":
		return fmt.Sprintf("%d days", value)
	}
	return fmt.Sprintf("%d %ss", value, unit)
}

// ============================================================================
// Essential Multi-Window Commands
// ============================================================================

// HandleHelp handles /help command
func (h *CommandHandler) HandleHelp(ctx context.Context, chatID, userID int64) (string, error) {
	// Check if user is admin
	isAdmin := false
	if chatID < 0 { // Group chat
		var err error
		isAdmin, err = h.client.IsAdmin(ctx, chatID, userID)
		if err != nil {
			// If permission check fails, show user help (safer default)
			isAdmin = false
		}
	}

	// User help (shown to everyone)
	userHelp := `🤖 Throttle Bot - Multi-Window Rate Limiter

👤 **Your Commands:**
• /help - Show this help message
• /mystatus - Check your current usage across all windows
• /windows - Show rate limiting configuration

💡 **How It Works:**
Each group has up to 3 rate limiting windows (A, B, C) with different:
- Character limits (e.g., 1000, 5000, 10000 chars)
- Time windows (e.g., 30 minutes, 2 hours, 7 days)

Your messages count toward all active windows. If you exceed ANY window's limit, your messages will be automatically deleted until the window resets.

🎯 **Override States:**
- **Normal**: Rate limiting applies (default)
- **Whitelisted**: Your messages always allowed ✅
- **Blacklisted**: Your messages always blocked ❌

� Contact an admin if you need help!`

	// Admin help (additional commands for admins)
	adminHelp := `

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
👑 **ADMIN COMMANDS**
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📝 **Window Configuration:**
• /setwindow <a|b|c> <limit> <duration> - Configure window
  Example: /setwindow a 1000 30m
• /enablewindow <a|b|c> - Enable a window
• /disablewindow <a|b|c> - Disable a window

🔧 **User Management:**
• /checkuser <user_id|@username> - Check specific user's usage
• /overrides - List all users with manual overrides
• /override <user_id|@username> <mode> [duration] - Manage user overrides
  **Modes**: whitelist | blacklist | clear
  **Examples**:
    /override 123456789 whitelist - Bypass all rate limiting
    /override @testuser whitelist 7d - Whitelist by username (7 days)
    /override 123456789 blacklist 2h - Block all messages (2 hours)
    /override @testuser clear - Return to normal

⏸️ **Group Controls:**
• /pause [duration] - Pause ALL rate limiting (optional: 30m, 2h, 7d)
• /resume - Resume rate limiting

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━`

	if isAdmin {
		return userHelp + adminHelp, nil
	}

	return userHelp, nil
}

// HandleExempt handles /exempt command
// HandleOverride manages user overrides (whitelist/blacklist/clear)
func (h *CommandHandler) HandleOverride(ctx context.Context, chatID, userID int64, args []string) (string, error) {
	// Check admin permission
	isAdmin, err := h.client.IsAdmin(ctx, chatID, userID)
	if err != nil {
		return "", fmt.Errorf("failed to check permissions: %w", err)
	}

	if !isAdmin {
		return "❌ You must be an admin to manage user overrides.", nil
	}

	// Validate arguments
	if len(args) < 2 {
		return "❌ Usage: /override <user_id|@username> <mode> [duration]\n\n" +
			"**Modes:**\n" +
			"• `whitelist` - Bypass all rate limiting (always allow)\n" +
			"• `blacklist` - Block all messages (always delete)\n" +
			"• `clear` - Remove override (return to normal rate limiting)\n\n" +
			"**Examples:**\n" +
			"• /override 123456789 whitelist - Permanent whitelist\n" +
			"• /override @testuser whitelist 7d - Temporary whitelist by username\n" +
			"• /override 123456789 blacklist 2h - Temporary blacklist\n" +
			"• /override @testuser clear - Reset to normal", nil
	}

	// Parse target user ID or @username
	targetUserID, err := h.ParseUserIdentifier(ctx, chatID, args[0])
	if err != nil {
		return fmt.Sprintf("❌ %v", err), nil
	}

	// Parse mode
	mode := strings.ToLower(args[1])

	// Handle clear mode (remove override)
	if mode == "clear" || mode == "reset" || mode == "off" {
		err = h.store.RemoveUserOverride(ctx, chatID, targetUserID)
		if err != nil {
			return "", fmt.Errorf("failed to remove user override: %w", err)
		}
		userDisplay := h.formatUserDisplay(ctx, targetUserID)
		return fmt.Sprintf("✅ User %s override cleared. Normal rate limiting will apply.", userDisplay), nil
	}

	// Validate mode
	if mode != "whitelist" && mode != "blacklist" {
		return "❌ Invalid mode. Must be: whitelist, blacklist, or clear", nil
	}

	// Parse optional duration
	var expiresAt *time.Time
	if len(args) >= 3 {
		durationValue, durationUnit, err := ParseDurationString(args[2])
		if err != nil {
			return fmt.Sprintf("❌ Invalid duration: %v\n\nExamples: 30m, 2h, 7d", err), nil
		}

		durationSeconds, err := ratelimit.CalculateWindowDuration(durationValue, durationUnit)
		if err != nil {
			return fmt.Sprintf("❌ %v", err), nil
		}

		expiryTime := time.Now().Add(time.Duration(durationSeconds) * time.Second)
		expiresAt = &expiryTime
	}

	// Set override state
	var overrideState *bool
	var reason string

	if mode == "whitelist" {
		trueValue := true
		overrideState = &trueValue
		reason = "whitelisted by admin"
	} else { // blacklist
		falseValue := false
		overrideState = &falseValue
		reason = "blacklisted by admin"
	}

	err = h.store.SetUserOverride(ctx, chatID, targetUserID, overrideState, reason, userID, expiresAt)
	if err != nil {
		return "", fmt.Errorf("failed to set user override: %w", err)
	}

	// Build response message
	var modeDesc, effect string
	if mode == "whitelist" {
		modeDesc = "whitelisted"
		effect = "bypasses all rate limiting"
	} else {
		modeDesc = "blacklisted"
		effect = "all messages will be blocked"
	}

	userDisplay := h.formatUserDisplay(ctx, targetUserID)

	if expiresAt != nil {
		durationValue, durationUnit, _ := ParseDurationString(args[2])
		durationStr := FormatWindowDuration(durationValue, durationUnit)
		return fmt.Sprintf("✅ User %s is now **%s** (%s) for %s.", userDisplay, modeDesc, effect, durationStr), nil
	}
	return fmt.Sprintf("✅ User %s is now **permanently %s** (%s).", userDisplay, modeDesc, effect), nil
}

// HandleMyStatus handles /mystatus command
func (h *CommandHandler) HandleMyStatus(ctx context.Context, chatID, userID int64) (string, error) {
	// Get user's window stats
	stats, err := h.store.GetUserWindowStats(ctx, userID, chatID)
	if err != nil {
		return "", fmt.Errorf("failed to get window stats: %w", err)
	}

	if len(stats) == 0 {
		return "📊 No rate limiting configured for this group yet.\nAdmin can use /setwindow to configure windows.", nil
	}

	var response strings.Builder
	response.WriteString("📊 **Your Rate Limit Status**\n\n")

	for _, stat := range stats {
		windowName := strings.ToUpper(stat.SlotID)
		status := "🔴 Disabled"
		if stat.Enabled {
			status = "🟢 Enabled"
		}

		usagePercent := 0
		if stat.CharLimit > 0 {
			usagePercent = (stat.CurrentUsage * 100) / stat.CharLimit
		}

		durationStr := FormatWindowDuration(stat.DurationValue, stat.DurationUnit)

		response.WriteString(fmt.Sprintf("**Window %s** %s\n", windowName, status))
		response.WriteString(fmt.Sprintf("├ Limit: %d chars in %s\n", stat.CharLimit, durationStr))
		response.WriteString(fmt.Sprintf("├ Usage: %d / %d chars (%d%%)\n", stat.CurrentUsage, stat.CharLimit, usagePercent))

		if stat.Enabled {
			if stat.CurrentUsage >= stat.CharLimit {
				response.WriteString("└ ⚠️ **LIMIT EXCEEDED** - Your messages will be deleted\n")
			} else if usagePercent >= 90 {
				response.WriteString("└ ⚠️ Warning: 90%+ usage\n")
			} else if usagePercent >= 80 {
				response.WriteString("└ ⚠️ Warning: 80%+ usage\n")
			} else {
				response.WriteString("└ ✅ Within limit\n")
			}
		} else {
			response.WriteString("└ Not enforced\n")
		}
		response.WriteString("\n")
	}

	response.WriteString("💡 Old messages automatically age out after the window duration.")

	return response.String(), nil
}

// HandleCheckUser handles /checkuser command (admin only)
func (h *CommandHandler) HandleCheckUser(ctx context.Context, chatID, adminID int64, args []string) (string, error) {
	// Check admin permission
	isAdmin, err := h.client.IsAdmin(ctx, chatID, adminID)
	if err != nil {
		return "", fmt.Errorf("failed to check permissions: %w", err)
	}

	if !isAdmin {
		return "❌ You must be an admin to check other users.", nil
	}

	// Validate arguments
	if len(args) < 1 {
		return "❌ Usage: /checkuser <user_id|@username>", nil
	}

	// Parse target user ID or @username
	targetUserID, err := h.ParseUserIdentifier(ctx, chatID, args[0])
	if err != nil {
		return fmt.Sprintf("❌ %v", err), nil
	}

	// Get user's window stats
	stats, err := h.store.GetUserWindowStats(ctx, targetUserID, chatID)
	if err != nil {
		return "", fmt.Errorf("failed to get window stats: %w", err)
	}

	if len(stats) == 0 {
		return "📊 No rate limiting configured for this group yet.", nil
	}

	// Format user display with username if available
	userDisplay := h.formatUserDisplay(ctx, targetUserID)

	var response strings.Builder
	response.WriteString(fmt.Sprintf("📊 **User %s Rate Limit Status**\n\n", userDisplay))

	for _, stat := range stats {
		windowName := strings.ToUpper(stat.SlotID)
		status := "🔴 Disabled"
		if stat.Enabled {
			status = "🟢 Enabled"
		}

		usagePercent := 0
		if stat.CharLimit > 0 {
			usagePercent = (stat.CurrentUsage * 100) / stat.CharLimit
		}

		durationStr := FormatWindowDuration(stat.DurationValue, stat.DurationUnit)

		response.WriteString(fmt.Sprintf("**Window %s** %s\n", windowName, status))
		response.WriteString(fmt.Sprintf("├ Limit: %d chars in %s\n", stat.CharLimit, durationStr))
		response.WriteString(fmt.Sprintf("├ Usage: %d / %d chars (%d%%)\n", stat.CurrentUsage, stat.CharLimit, usagePercent))

		if stat.Enabled {
			if stat.CurrentUsage >= stat.CharLimit {
				response.WriteString("└ ⚠️ **LIMIT EXCEEDED**\n")
			} else if usagePercent >= 90 {
				response.WriteString("└ ⚠️ Warning: 90%+ usage\n")
			} else if usagePercent >= 80 {
				response.WriteString("└ ⚠️ Warning: 80%+ usage\n")
			} else {
				response.WriteString("└ ✅ Within limit\n")
			}
		} else {
			response.WriteString("└ Not enforced\n")
		}
		response.WriteString("\n")
	}

	return response.String(), nil
}

// HandleListOverrides lists all users with manual overrides in the group
func (h *CommandHandler) HandleListOverrides(ctx context.Context, chatID, adminID int64) (string, error) {
	// Check admin permission
	isAdmin, err := h.client.IsAdmin(ctx, chatID, adminID)
	if err != nil {
		return "", fmt.Errorf("failed to check permissions: %w", err)
	}

	if !isAdmin {
		return "❌ You must be an admin to view overrides.", nil
	}

	// Get all overrides for this chat
	overrides, err := h.store.GetAllOverrides(ctx, chatID)
	if err != nil {
		return "", fmt.Errorf("failed to get overrides: %w", err)
	}

	if len(overrides) == 0 {
		return "📋 **Manual Overrides**\n\nNo manual overrides set for this group.\n\n💡 Use /override <user_id|@username> <whitelist|blacklist|clear> to manage overrides.", nil
	}

	var response strings.Builder
	response.WriteString(fmt.Sprintf("📋 **Manual Overrides** (%d users)\n\n", len(overrides)))

	for _, override := range overrides {
		// Get user display with username
		userDisplay := h.formatUserDisplay(ctx, override.UserID)

		// Format override state
		var stateIcon, stateText string
		if override.OverrideState == nil {
			stateIcon = "➖"
			stateText = "Normal (cleared)"
		} else if *override.OverrideState {
			stateIcon = "✅"
			stateText = "Whitelisted"
		} else {
			stateIcon = "❌"
			stateText = "Blacklisted"
		}

		response.WriteString(fmt.Sprintf("%s **%s** - %s\n", stateIcon, userDisplay, stateText))

		// Show reason if present
		if override.Reason != nil && *override.Reason != "" {
			response.WriteString(fmt.Sprintf("  └ Reason: %s\n", *override.Reason))
		}

		// Show expiration if present
		if override.ExpiresAt != nil {
			expiresAt := *override.ExpiresAt
			now := time.Now()
			if expiresAt.After(now) {
				duration := expiresAt.Sub(now)
				hours := int(duration.Hours())
				if hours < 24 {
					response.WriteString(fmt.Sprintf("  └ Expires in: %d hours\n", hours))
				} else {
					days := hours / 24
					response.WriteString(fmt.Sprintf("  └ Expires in: %d days\n", days))
				}
			} else {
				response.WriteString("  └ ⚠️ Expired (will be cleaned up)\n")
			}
		} else {
			response.WriteString("  └ Permanent (no expiration)\n")
		}

		response.WriteString("\n")
	}

	response.WriteString("💡 Use /checkuser <user_id|@username> to see detailed stats\n")
	response.WriteString("💡 Use /override <user_id|@username> clear to remove an override")

	return response.String(), nil
}

// HandleSetWindow handles /setwindow command
func (h *CommandHandler) HandleSetWindow(ctx context.Context, chatID, userID int64, args []string) (string, error) {
	// Check admin permission
	isAdmin, err := h.client.IsAdmin(ctx, chatID, userID)
	if err != nil {
		return "", fmt.Errorf("failed to check permissions: %w", err)
	}

	if !isAdmin {
		return "❌ You must be an admin to configure windows.", nil
	}

	// Validate arguments
	if len(args) < 3 {
		return "❌ Usage: /setwindow <a|b|c> <limit> <duration>\nExample: /setwindow a 1000 30m", nil
	}

	slotID := strings.ToLower(args[0])
	if slotID != "a" && slotID != "b" && slotID != "c" {
		return "❌ Window must be a, b, or c", nil
	}

	charLimit, err := strconv.Atoi(args[1])
	if err != nil || charLimit <= 0 {
		return "❌ Character limit must be a positive number", nil
	}

	durationValue, durationUnit, err := ParseDurationString(args[2])
	if err != nil {
		return fmt.Sprintf("❌ Invalid duration: %v", err), nil
	}

	// Calculate window duration in seconds
	windowDuration, err := ratelimit.CalculateWindowDuration(durationValue, durationUnit)
	if err != nil {
		return fmt.Sprintf("❌ Failed to calculate duration: %v", err), nil
	}

	// Ensure group and default windows exist
	if err := h.store.CreateDefaultWindows(ctx, chatID); err != nil {
		return "", fmt.Errorf("failed to initialize windows: %w", err)
	}

	// Update window slot configuration
	err = h.store.UpdateWindowSlot(ctx, chatID, slotID, charLimit, durationValue, durationUnit, windowDuration, true)
	if err != nil {
		return "", fmt.Errorf("failed to update window: %w", err)
	}

	// Reset all users' messages in this chat (affects all windows - shared message log)
	// Note: With single message log design, this clears ALL window history
	if err := h.store.ResetWindowMessagesForAll(ctx, chatID); err != nil {
		return "", fmt.Errorf("failed to reset window messages: %w", err)
	}

	durationStr := FormatWindowDuration(durationValue, durationUnit)
	return fmt.Sprintf("✅ Window %s configured:\n• Limit: %d characters\n• Duration: %s\n• Status: Enabled\n\nAll users' messages for this window have been reset.",
		strings.ToUpper(slotID), charLimit, durationStr), nil
}

// HandleDisableWindow handles /disablewindow command
func (h *CommandHandler) HandleDisableWindow(ctx context.Context, chatID, userID int64, args []string) (string, error) {
	// Check admin permission
	isAdmin, err := h.client.IsAdmin(ctx, chatID, userID)
	if err != nil {
		return "", fmt.Errorf("failed to check permissions: %w", err)
	}

	if !isAdmin {
		return "❌ You must be an admin to manage windows.", nil
	}

	// Validate arguments
	if len(args) < 1 {
		return "❌ Usage: /disablewindow <a|b|c>", nil
	}

	slotID := strings.ToLower(args[0])
	if slotID != "a" && slotID != "b" && slotID != "c" {
		return "❌ Window must be a, b, or c", nil
	}

	// Disable window
	err = h.store.SetWindowEnabled(ctx, chatID, slotID, false)
	if err != nil {
		return "", fmt.Errorf("failed to disable window: %w", err)
	}

	return fmt.Sprintf("✅ Window %s disabled. It will no longer enforce rate limits.", strings.ToUpper(slotID)), nil
}

// HandleEnableWindow handles /enablewindow command
func (h *CommandHandler) HandleEnableWindow(ctx context.Context, chatID, userID int64, args []string) (string, error) {
	// Check admin permission
	isAdmin, err := h.client.IsAdmin(ctx, chatID, userID)
	if err != nil {
		return "", fmt.Errorf("failed to check permissions: %w", err)
	}

	if !isAdmin {
		return "❌ You must be an admin to manage windows.", nil
	}

	// Validate arguments
	if len(args) < 1 {
		return "❌ Usage: /enablewindow <a|b|c>", nil
	}

	slotID := strings.ToLower(args[0])
	if slotID != "a" && slotID != "b" && slotID != "c" {
		return "❌ Window must be a, b, or c", nil
	}

	// Enable window
	err = h.store.SetWindowEnabled(ctx, chatID, slotID, true)
	if err != nil {
		return "", fmt.Errorf("failed to enable window: %w", err)
	}

	return fmt.Sprintf("✅ Window %s enabled. Rate limits will now be enforced.", strings.ToUpper(slotID)), nil
}

// HandleShowWindows handles /showwindows command
func (h *CommandHandler) HandleShowWindows(ctx context.Context, chatID, userID int64, args []string) (string, error) {
	// Check admin permission
	isAdmin, err := h.client.IsAdmin(ctx, chatID, userID)
	if err != nil {
		return "", fmt.Errorf("failed to check permissions: %w", err)
	}

	if !isAdmin {
		return "❌ You must be an admin to view window configurations.", nil
	}

	// Get all windows
	windows, err := h.store.GetAllWindows(ctx, chatID)
	if err != nil {
		return "", fmt.Errorf("failed to get windows: %w", err)
	}

	if len(windows) == 0 {
		return "📊 No windows configured yet.\nUse /setwindow to configure rate limiting.", nil
	}

	var response strings.Builder
	response.WriteString("📊 **Multi-Window Rate Limit Configuration**\n\n")

	for _, window := range windows {
		windowName := strings.ToUpper(window.SlotID)
		status := "🔴 Disabled"
		if window.Enabled {
			status = "🟢 Enabled"
		}

		durationStr := FormatWindowDuration(window.DurationValue, window.DurationUnit)

		response.WriteString(fmt.Sprintf("**Window %s** %s\n", windowName, status))
		response.WriteString(fmt.Sprintf("├ Limit: %d characters\n", window.CharLimit))
		response.WriteString(fmt.Sprintf("└ Duration: %s\n\n", durationStr))
	}

	response.WriteString("💡 Use /setwindow <a|b|c> <limit> <duration> to modify\n")
	response.WriteString("💡 Use /enablewindow or /disablewindow to toggle enforcement")

	return response.String(), nil
}

// ============================================================================
// Group Pause/Resume Commands
// ============================================================================

// HandlePause handles /pause command - pauses ALL rate limiting for the group
func (h *CommandHandler) HandlePause(ctx context.Context, chatID, adminID int64, args []string) (string, error) {
	// Check admin permission
	isAdmin, err := h.client.IsAdmin(ctx, chatID, adminID)
	if err != nil {
		return "", fmt.Errorf("failed to check permissions: %w", err)
	}

	if !isAdmin {
		return "❌ You must be an admin to pause rate limiting.", nil
	}

	// Parse optional duration argument
	var resumeAt *time.Time
	if len(args) > 0 {
		durationValue, durationUnit, err := ParseDurationString(args[0])
		if err != nil {
			return fmt.Sprintf("❌ Invalid duration: %v\nUsage: /pause [duration]\nExample: /pause 1h", err), nil
		}

		// Calculate resume time
		var duration time.Duration
		switch durationUnit {
		case "minute":
			duration = time.Duration(durationValue) * time.Minute
		case "hour":
			duration = time.Duration(durationValue) * time.Hour
		case "day":
			duration = time.Duration(durationValue) * 24 * time.Hour
		}

		resumeTime := time.Now().Add(duration)
		resumeAt = &resumeTime
	}

	// Pause the group
	if err := h.store.SetGroupPaused(ctx, chatID, true, resumeAt); err != nil {
		return "", fmt.Errorf("failed to pause group: %w", err)
	}

	if resumeAt != nil {
		return fmt.Sprintf("✅ Rate limiting paused until %s\nAll users can send unlimited messages during this period.", resumeAt.Format("2006-01-02 15:04:05")), nil
	}
	return "✅ Rate limiting paused indefinitely.\nAll users can send unlimited messages until you use /resume.", nil
}

// HandleResume handles /resume command - resumes rate limiting for the group
func (h *CommandHandler) HandleResume(ctx context.Context, chatID, adminID int64) (string, error) {
	// Check admin permission
	isAdmin, err := h.client.IsAdmin(ctx, chatID, adminID)
	if err != nil {
		return "", fmt.Errorf("failed to check permissions: %w", err)
	}

	if !isAdmin {
		return "❌ You must be an admin to resume rate limiting.", nil
	}

	// Check if group is paused
	isPaused, err := h.store.IsGroupPaused(ctx, chatID)
	if err != nil {
		return "", fmt.Errorf("failed to check pause status: %w", err)
	}

	if !isPaused {
		return "ℹ️ Rate limiting is not paused for this group.", nil
	}

	// Resume the group
	if err := h.store.SetGroupPaused(ctx, chatID, false, nil); err != nil {
		return "", fmt.Errorf("failed to resume group: %w", err)
	}

	return "✅ Rate limiting resumed.\nAll enabled windows are now enforcing limits again.", nil
}
