// Package bot provides Telegram bot handlers and command processing.
// Clean implementation with ONLY multi-window commands (no legacy code)
package bot

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	i18nPkg "github.com/antst/tg-throttle-bot/internal/i18n"
	"github.com/antst/tg-throttle-bot/internal/metrics"
	"github.com/antst/tg-throttle-bot/internal/ratelimit"
	"github.com/antst/tg-throttle-bot/internal/telegram"
)

// ValidationError wraps user-facing validation error messages
// These are distinguished from system errors and should not trigger group notifications
type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

// CommandResult contains the result of a command execution
type CommandResult struct {
	Response          string // Private message to user
	GroupNotification string // Optional notification to send to group (empty = no notification)
}

// Context keys for passing metadata between command handlers and router
type contextKey string

const (
	contextKeyTargetGroupID contextKey = "targetGroupID"
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

// CommandContext contains execution context for a command
type CommandContext struct {
	ChatID       int64  // Source chat ID (where command was sent)
	UserID       int64  // User who sent the command
	ChatType     string // "private", "group", or "supergroup"
	TargetGroup  int64  // Target group ID (may differ from ChatID for private→group commands)
	UserLanguage string // User's preferred language (en/ru/nl) - fetched once at command start
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

// localize is a helper to localize messages using user's language from CommandContext
func localize(cmdCtx CommandContext, key string, data map[string]interface{}) string {
	localizer := i18nPkg.GetLocalizer(cmdCtx.UserLanguage)
	return i18nPkg.Localize(localizer, key, data)
}

// ParseCommand extracts command name and arguments from message text.
// Supports quoted strings for multi-word arguments (Feature 014).
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

	// Parse arguments with quote support (Feature 014: multi-word group titles)
	parts, err := parseCommandArgs(text)
	if err != nil {
		return nil, err
	}

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

// parseCommandArgs splits command text into arguments, handling quoted strings.
// Supports straight quotes (', "), curly quotes (", ", ', '), and guillemets («, »).
// Example: `cmd arg1 "multi word arg" arg3` → ["cmd", "arg1", "multi word arg", "arg3"]
func parseCommandArgs(text string) ([]string, error) {
	var args []string
	var current strings.Builder
	var inQuote rune // 0 = not in quote, else the opening quote character

	// Map of closing quotes to their opening counterparts
	closeQuotes := map[rune]rune{
		'"':      '"',      // straight double quote
		'\'':     '\'',     // straight single quote
		'\u201d': '\u201c', // curly double quote " → "
		'\u2019': '\u2018', // curly single quote ' → '
		'\u00bb': '\u00ab', // guillemet » → «
	}

	// Opening quote characters (straight and curly)
	openingQuotes := map[rune]bool{
		'"':      true,
		'\'':     true,
		'\u201c': true, // "
		'\u2018': true, // '
		'\u00ab': true, // «
	}

	for i, r := range text {
		switch {
		case inQuote != 0:
			// Inside quoted string - check if this is the closing quote
			if (r == inQuote) || (closeQuotes[r] == inQuote) {
				// Closing quote found
				args = append(args, current.String())
				current.Reset()
				inQuote = 0
			} else {
				current.WriteRune(r)
			}

		case openingQuotes[r]:
			// Opening quote (straight or curly)
			if current.Len() > 0 {
				// Save any accumulated non-quoted text first
				args = append(args, current.String())
				current.Reset()
			}
			inQuote = r

		case r == ' ' || r == '\t' || r == '\n':
			// Whitespace - end of argument
			if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}

		default:
			// Regular character
			current.WriteRune(r)
		}

		// Check for unclosed quote at end
		if i == len(text)-1 && inQuote != 0 {
			return nil, fmt.Errorf("unclosed quote in command")
		}
	}

	// Add final argument if any
	if current.Len() > 0 {
		args = append(args, current.String())
	}

	return args, nil
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
func (h *CommandHandler) HandleHelp(ctx context.Context, cmdCtx CommandContext, args []string) (string, error) {
	// Check if user requested admin help explicitly
	showAdminHelp := false
	if len(args) > 0 && strings.ToLower(args[0]) == "admin" {
		showAdminHelp = true
	}

	// Get localized help text
	userHelp := localize(cmdCtx, "cmd_help_user", nil)

	if showAdminHelp {
		adminHelp := localize(cmdCtx, "cmd_help_admin", nil)
		return userHelp + "\n\n" + adminHelp, nil
	}

	return userHelp, nil
}

// HandleExempt handles /exempt command
// HandleOverride manages user overrides (whitelist/blacklist/clear)
func (h *CommandHandler) HandleOverride(ctx context.Context, cmdCtx CommandContext, args []string) (string, error) {
	targetGroupID := cmdCtx.TargetGroup

	// Note: args already has group identifier removed by handler.go
	// In private chat: /override @groupname @user whitelist → args = ["@user", "whitelist"]
	// In group chat: /override @user whitelist → args = ["@user", "whitelist"]

	// Validate minimum arguments
	if len(args) < 2 {
		return "", &ValidationError{localize(cmdCtx, "cmd_override_usage_private", nil)}
	}

	// Check admin permission on TARGET group
	isAdmin, err := h.client.IsAdmin(ctx, targetGroupID, cmdCtx.UserID)
	if err != nil {
		return "", fmt.Errorf("failed to check permissions: %w", err)
	}

	if !isAdmin {
		return "", &ValidationError{localize(cmdCtx, "cmd_error_not_admin", map[string]interface{}{
			"action": "manage user overrides",
		})}
	}

	// Parse target user ID or @username
	targetUserID, err := h.ParseUserIdentifier(ctx, targetGroupID, args[0])
	if err != nil {
		return fmt.Sprintf("❌ %v", err), nil
	}

	// Parse mode
	mode := strings.ToLower(args[1])

	// Handle clear mode (remove override)
	if mode == "clear" || mode == "reset" || mode == "off" {
		err = h.store.RemoveUserOverride(ctx, targetGroupID, targetUserID)
		if err != nil {
			return "", fmt.Errorf("failed to remove user override: %w", err)
		}
		userDisplay := h.formatUserDisplay(ctx, targetUserID)
		return localize(cmdCtx, "cmd_override_cleared", map[string]interface{}{
			"user": userDisplay,
		}), nil
	}

	// Validate mode
	if mode != "whitelist" && mode != "blacklist" {
		return "", &ValidationError{localize(cmdCtx, "cmd_override_invalid_mode", map[string]interface{}{
			"mode": mode,
		})}
	}

	// Parse optional duration
	var expiresAt *time.Time
	var durationStr string
	if len(args) >= 3 {
		durationValue, durationUnit, err := ParseDurationString(args[2])
		if err != nil {
			return "", &ValidationError{localize(cmdCtx, "cmd_override_invalid_duration", map[string]interface{}{
				"error": err.Error(),
			})}
		}

		durationSeconds, err := ratelimit.CalculateWindowDuration(durationValue, durationUnit)
		if err != nil {
			return "", &ValidationError{localize(cmdCtx, "cmd_override_invalid_duration", map[string]interface{}{
				"error": err.Error(),
			})}
		}

		expiryTime := time.Now().Add(time.Duration(durationSeconds) * time.Second)
		expiresAt = &expiryTime
		durationStr = FormatWindowDuration(durationValue, durationUnit)
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

	err = h.store.SetUserOverride(ctx, cmdCtx.ChatID, targetUserID, overrideState, reason, cmdCtx.UserID, expiresAt)
	if err != nil {
		return "", fmt.Errorf("failed to set user override: %w", err)
	}

	// Build response message
	userDisplay := h.formatUserDisplay(ctx, targetUserID)
	var durationSuffix string
	if expiresAt != nil {
		durationSuffix = " for " + durationStr
	} else {
		durationSuffix = " permanently"
	}

	if mode == "whitelist" {
		return localize(cmdCtx, "cmd_override_whitelist_success", map[string]interface{}{
			"user":     userDisplay,
			"duration": durationSuffix,
		}), nil
	}
	return localize(cmdCtx, "cmd_override_blacklist_success", map[string]interface{}{
		"user":     userDisplay,
		"duration": durationSuffix,
	}), nil
}

// HandleMyStatus handles /mystatus command
func (h *CommandHandler) HandleMyStatus(ctx context.Context, cmdCtx CommandContext) (string, error) {
	// Use target group ID (resolved in handler.go)
	// In private chat: first arg was @groupname (already stripped by handler.go resolution)
	// In group chat: cmdCtx.TargetGroup == cmdCtx.ChatID
	targetGroupID := cmdCtx.TargetGroup

	// Get user's window stats from TARGET group
	stats, err := h.store.GetUserWindowStats(ctx, cmdCtx.UserID, targetGroupID)
	if err != nil {
		return "", fmt.Errorf("failed to get window stats: %w", err)
	}

	if len(stats) == 0 {
		return "", &ValidationError{localize(cmdCtx, "cmd_mystatus_no_config", nil)}
	}

	// Get group name for display
	bot := h.client.GetBot()
	chat, err := bot.GetChat(tgbotapi.ChatInfoConfig{
		ChatConfig: tgbotapi.ChatConfig{ChatID: targetGroupID},
	})
	groupName := fmt.Sprintf("Group %d", targetGroupID)
	if err == nil && chat.Title != "" {
		groupName = chat.Title
	}

	var response strings.Builder
	response.WriteString(localize(cmdCtx, "cmd_mystatus_title", map[string]interface{}{
		"group": groupName,
	}))

	for _, stat := range stats {
		windowName := strings.ToUpper(stat.SlotID)

		if !stat.Enabled {
			response.WriteString(localize(cmdCtx, "cmd_mystatus_window_disabled", map[string]interface{}{
				"slot": windowName,
			}))
			continue
		}

		usagePercent := 0
		if stat.CharLimit > 0 {
			usagePercent = (stat.CurrentUsage * 100) / stat.CharLimit
		}

		data := map[string]interface{}{
			"slot":       windowName,
			"current":    stat.CurrentUsage,
			"limit":      stat.CharLimit,
			"percentage": usagePercent,
		}

		if stat.CurrentUsage >= stat.CharLimit {
			response.WriteString(localize(cmdCtx, "cmd_mystatus_window_exceeded", data))
		} else if usagePercent >= 80 {
			response.WriteString(localize(cmdCtx, "cmd_mystatus_window_warning", data))
		} else {
			response.WriteString(localize(cmdCtx, "cmd_mystatus_window_ok", data))
		}
	}

	return response.String(), nil
}

// HandleCheckUser handles /checkuser command (admin only)
func (h *CommandHandler) HandleCheckUser(ctx context.Context, cmdCtx CommandContext, args []string) (string, error) {
	targetGroupID := cmdCtx.TargetGroup

	// Note: args already has group identifier removed by handler.go
	// In private chat: /checkuser @groupname @username → args = ["@username"]
	// In group chat: /checkuser @username → args = ["@username"]

	// Check admin permission on TARGET group
	isAdmin, err := h.client.IsAdmin(ctx, targetGroupID, cmdCtx.UserID)
	if err != nil {
		return "", fmt.Errorf("failed to check permissions: %w", err)
	}

	if !isAdmin {
		return "", &ValidationError{localize(cmdCtx, "cmd_error_not_admin", map[string]interface{}{
			"action": "check other users",
		})}
	}

	// Validate arguments
	if len(args) < 1 {
		return "", &ValidationError{localize(cmdCtx, "cmd_checkuser_usage_group", nil)}
	}

	// Parse target user ID or @username
	targetUserID, err := h.ParseUserIdentifier(ctx, targetGroupID, args[0])
	if err != nil {
		return "", &ValidationError{fmt.Sprintf("❌ %v", err)}
	}

	// Get user's window stats from TARGET group
	stats, err := h.store.GetUserWindowStats(ctx, targetUserID, targetGroupID)
	if err != nil {
		return "", fmt.Errorf("failed to get window stats: %w", err)
	}

	if len(stats) == 0 {
		return "", &ValidationError{localize(cmdCtx, "cmd_checkuser_no_config", nil)}
	}

	// Format user display with username if available
	userDisplay := h.formatUserDisplay(ctx, targetUserID)

	// Get group name for display
	bot := h.client.GetBot()
	chat, err := bot.GetChat(tgbotapi.ChatInfoConfig{
		ChatConfig: tgbotapi.ChatConfig{ChatID: targetGroupID},
	})
	groupName := fmt.Sprintf("Group %d", targetGroupID)
	if err == nil && chat.Title != "" {
		groupName = chat.Title
	}

	var response strings.Builder
	response.WriteString(localize(cmdCtx, "cmd_checkuser_title", map[string]interface{}{
		"user":  userDisplay,
		"group": groupName,
	}))

	for _, stat := range stats {
		windowName := strings.ToUpper(stat.SlotID)

		if !stat.Enabled {
			response.WriteString(localize(cmdCtx, "cmd_checkuser_window_disabled", map[string]interface{}{
				"slot": windowName,
			}))
			continue
		}

		usagePercent := 0
		if stat.CharLimit > 0 {
			usagePercent = (stat.CurrentUsage * 100) / stat.CharLimit
		}

		data := map[string]interface{}{
			"slot":       windowName,
			"current":    stat.CurrentUsage,
			"limit":      stat.CharLimit,
			"percentage": usagePercent,
		}

		if stat.CurrentUsage >= stat.CharLimit {
			response.WriteString(localize(cmdCtx, "cmd_checkuser_window_exceeded", data))
		} else if usagePercent >= 80 {
			response.WriteString(localize(cmdCtx, "cmd_checkuser_window_warning", data))
		} else {
			response.WriteString(localize(cmdCtx, "cmd_checkuser_window_ok", data))
		}
	}

	return response.String(), nil
}

// HandleListOverrides lists all users with manual overrides in the group
func (h *CommandHandler) HandleListOverrides(ctx context.Context, cmdCtx CommandContext) (string, error) {
	targetGroupID := cmdCtx.TargetGroup

	// Note: No args to strip - this command takes no arguments
	// In private chat: /overrides @groupname
	// In group chat: /overrides

	// Check admin permission on TARGET group
	isAdmin, err := h.client.IsAdmin(ctx, targetGroupID, cmdCtx.UserID)
	if err != nil {
		return "", fmt.Errorf("failed to check permissions: %w", err)
	}

	if !isAdmin {
		return "", &ValidationError{localize(cmdCtx, "cmd_error_not_admin", map[string]interface{}{
			"action": "view overrides",
		})}
	}

	// Get all overrides for TARGET group
	overrides, err := h.store.GetAllOverrides(ctx, targetGroupID)
	if err != nil {
		return "", fmt.Errorf("failed to get overrides: %w", err)
	}

	if len(overrides) == 0 {
		return "", &ValidationError{localize(cmdCtx, "cmd_showoverrides_no_overrides", nil)}
	}

	// Get group name for display
	bot := h.client.GetBot()
	chat, err := bot.GetChat(tgbotapi.ChatInfoConfig{
		ChatConfig: tgbotapi.ChatConfig{ChatID: targetGroupID},
	})
	groupName := fmt.Sprintf("Group %d", targetGroupID)
	if err == nil && chat.Title != "" {
		groupName = chat.Title
	}

	var response strings.Builder
	response.WriteString(localize(cmdCtx, "cmd_showoverrides_title", map[string]interface{}{
		"group": groupName,
	}))

	for _, override := range overrides {
		// Get user display with username
		userDisplay := h.formatUserDisplay(ctx, override.UserID)

		// Format creation time
		createdTime := override.CreatedAt.Format("2006-01-02 15:04")

		// Format expiration
		var expiresText string
		if override.ExpiresAt != nil {
			expiresAt := *override.ExpiresAt
			now := time.Now()
			if expiresAt.After(now) {
				duration := expiresAt.Sub(now)
				hours := int(duration.Hours())
				if hours < 24 {
					expiresText = fmt.Sprintf("\n  • Expires: in %d hours", hours)
				} else {
					days := hours / 24
					expiresText = fmt.Sprintf("\n  • Expires: in %d days", days)
				}
			} else {
				expiresText = "\n  • ⚠️ Expired (will be cleaned up)"
			}
		} else {
			expiresText = "\n  • Permanent (no expiration)"
		}

		// Show whitelist or blacklist entry
		data := map[string]interface{}{
			"user":    userDisplay,
			"created": createdTime,
			"expires": expiresText,
		}

		if override.OverrideState != nil && *override.OverrideState {
			response.WriteString(localize(cmdCtx, "cmd_showoverrides_whitelist_entry", data))
		} else if override.OverrideState != nil && !*override.OverrideState {
			response.WriteString(localize(cmdCtx, "cmd_showoverrides_blacklist_entry", data))
		}
	}

	return response.String(), nil
}

// HandleSetWindow handles /setwindow command
func (h *CommandHandler) HandleSetWindow(ctx context.Context, cmdCtx CommandContext, args []string) (string, error) {
	// Feature 008 US4: cmdCtx.TargetGroup already resolved in handler.go
	// - For group chat: TargetGroup = chatID
	// - For private chat: TargetGroup = resolved from first arg (@username or -ID)
	// - args: group identifier already removed by handler.go for private chat
	targetGroupID := cmdCtx.TargetGroup

	// Check admin permission on TARGET group
	isAdmin, err := h.client.IsAdmin(ctx, targetGroupID, cmdCtx.UserID)
	if err != nil {
		return "", fmt.Errorf("failed to check permissions: %w", err)
	}

	if !isAdmin {
		return "", &ValidationError{localize(cmdCtx, "cmd_error_not_admin", map[string]interface{}{
			"action": "configure rate limit windows",
		})}
	}

	// Validate arguments (Feature 009 US4: Use user language for command responses)
	if len(args) < 3 {
		if cmdCtx.ChatType == "private" {
			return "", &ValidationError{localize(cmdCtx, "cmd_setwindow_usage_private", nil)}
		}
		return "", &ValidationError{localize(cmdCtx, "cmd_setwindow_usage_group", nil)}
	}

	slotID := strings.ToLower(args[0])
	if slotID != "a" && slotID != "b" && slotID != "c" {
		return "", &ValidationError{localize(cmdCtx, "cmd_setwindow_invalid_slot", map[string]interface{}{
			"slot": slotID,
		})}
	}

	charLimit, err := strconv.Atoi(args[1])
	if err != nil || charLimit <= 0 {
		return "", &ValidationError{localize(cmdCtx, "cmd_setwindow_invalid_limit", map[string]interface{}{
			"error": "must be a positive integer",
		})}
	}

	durationValue, durationUnit, err := ParseDurationString(args[2])
	if err != nil {
		return "", &ValidationError{localize(cmdCtx, "cmd_setwindow_invalid_duration", map[string]interface{}{
			"error": err.Error(),
		})}
	}

	// Calculate window duration in seconds
	windowDuration, err := ratelimit.CalculateWindowDuration(durationValue, durationUnit)
	if err != nil {
		return "", &ValidationError{localize(cmdCtx, "cmd_setwindow_invalid_duration", map[string]interface{}{
			"error": err.Error(),
		})}
	}

	// Ensure group and default windows exist (for TARGET group)
	if err := h.store.CreateDefaultWindows(ctx, targetGroupID); err != nil {
		return "", fmt.Errorf("failed to initialize windows: %w", err)
	}

	// Update window slot configuration (for TARGET group)
	err = h.store.UpdateWindowSlot(ctx, targetGroupID, slotID, charLimit, durationValue, durationUnit, windowDuration, true)
	if err != nil {
		return "", fmt.Errorf("failed to update window: %w", err)
	}

	// Reset all users' messages in this chat (affects all windows - shared message log)
	// Note: With single message log design, this clears ALL window history
	if err := h.store.ResetWindowMessagesForAll(ctx, targetGroupID); err != nil {
		return "", fmt.Errorf("failed to reset window messages: %w", err)
	}

	// Format duration string for display
	durationStr := FormatWindowDuration(durationValue, durationUnit)
	status := "enabled"

	return localize(cmdCtx, "cmd_setwindow_success", map[string]interface{}{
		"window":   strings.ToUpper(slotID),
		"limit":    charLimit,
		"duration": durationStr,
		"status":   status,
	}), nil
}

// HandleDisableWindow handles /disablewindow command
func (h *CommandHandler) HandleDisableWindow(ctx context.Context, cmdCtx CommandContext, args []string) (string, error) {
	// Feature 008 US4: cmdCtx.TargetGroup already resolved in handler.go
	targetGroupID := cmdCtx.TargetGroup

	// Check admin permission on TARGET group
	isAdmin, err := h.client.IsAdmin(ctx, targetGroupID, cmdCtx.UserID)
	if err != nil {
		return "", fmt.Errorf("failed to check permissions: %w", err)
	}

	if !isAdmin {
		return "", &ValidationError{localize(cmdCtx, "permission_denied_admin_only", nil)}
	}

	// Validate arguments (group identifier already removed by handler.go)
	if len(args) < 1 {
		return "", &ValidationError{localize(cmdCtx, "disablewindow_usage", nil)}
	}

	slotID := strings.ToLower(args[0])
	if slotID != "a" && slotID != "b" && slotID != "c" {
		return "", &ValidationError{localize(cmdCtx, "invalid_window_slot", nil)}
	}

	// Disable window (for TARGET group)
	err = h.store.SetWindowEnabled(ctx, targetGroupID, slotID, false)
	if err != nil {
		return "", fmt.Errorf("failed to disable window: %w", err)
	}

	return localize(cmdCtx, "window_disabled_success", map[string]interface{}{
		"window": strings.ToUpper(slotID),
	}), nil
}

// HandleEnableWindow handles /enablewindow command
func (h *CommandHandler) HandleEnableWindow(ctx context.Context, cmdCtx CommandContext, args []string) (string, error) {
	// Feature 008 US4: cmdCtx.TargetGroup already resolved in handler.go
	targetGroupID := cmdCtx.TargetGroup

	// Check admin permission on TARGET group
	isAdmin, err := h.client.IsAdmin(ctx, targetGroupID, cmdCtx.UserID)
	if err != nil {
		return "", fmt.Errorf("failed to check permissions: %w", err)
	}

	if !isAdmin {
		return "", &ValidationError{localize(cmdCtx, "permission_denied_admin_only", nil)}
	}

	// Validate arguments (group identifier already removed by handler.go)
	if len(args) < 1 {
		return "", &ValidationError{localize(cmdCtx, "enablewindow_usage", nil)}
	}

	slotID := strings.ToLower(args[0])
	if slotID != "a" && slotID != "b" && slotID != "c" {
		return "", &ValidationError{localize(cmdCtx, "invalid_window_slot", nil)}
	}

	// Enable window (for TARGET group)
	err = h.store.SetWindowEnabled(ctx, targetGroupID, slotID, true)
	if err != nil {
		return "", fmt.Errorf("failed to enable window: %w", err)
	}

	return localize(cmdCtx, "window_enabled_success", map[string]interface{}{
		"window": strings.ToUpper(slotID),
	}), nil
}

// HandleConfig handles /config command
func (h *CommandHandler) HandleConfig(ctx context.Context, cmdCtx CommandContext, args []string) (string, error) {
	targetGroupID := cmdCtx.TargetGroup

	// Note: No args to strip - this command takes no arguments
	// In private chat: /config @groupname
	// In group chat: /config

	// Check admin permission on TARGET group
	isAdmin, err := h.client.IsAdmin(ctx, targetGroupID, cmdCtx.UserID)
	if err != nil {
		return "", fmt.Errorf("failed to check permissions: %w", err)
	}

	if !isAdmin {
		return "", &ValidationError{localize(cmdCtx, "permission_denied_admin_only", nil)}
	}

	// Get all windows for TARGET group
	windows, err := h.store.GetAllWindows(ctx, targetGroupID)
	if err != nil {
		return "", fmt.Errorf("failed to get windows: %w", err)
	}

	if len(windows) == 0 {
		return "", &ValidationError{localize(cmdCtx, "window_show_none", nil)}
	}

	// Get group's language for displaying group configuration metadata
	groupLang := "en"
	if lang, err := h.store.GetGroupLanguage(ctx, targetGroupID); err == nil {
		groupLang = lang
	}

	// Build response with localized header (use USER language for command response)
	// But include group's language setting as informational metadata
	userLocalizer := i18nPkg.GetLocalizer(cmdCtx.UserLanguage)
	languageName := i18nPkg.GetLanguageName(userLocalizer, groupLang)
	header := i18nPkg.Localize(userLocalizer, "window_show_header", map[string]interface{}{
		"language": languageName,
	})

	var response strings.Builder
	response.WriteString(header)

	for _, window := range windows {
		windowName := strings.ToUpper(window.SlotID)

		// Localized status (use USER language for command response)
		var statusKey string
		if window.Enabled {
			statusKey = "window_status_enabled"
		} else {
			statusKey = "window_status_disabled"
		}
		status := i18nPkg.LocalizeSimple(userLocalizer, statusKey)

		// Format duration with localization (use USER language)
		durationStr := i18nPkg.FormatDuration(userLocalizer, window.DurationValue, window.DurationUnit)

		// Build localized entry (use USER language)
		entry := i18nPkg.Localize(userLocalizer, "window_show_entry", map[string]interface{}{
			"window":   windowName,
			"limit":    window.CharLimit,
			"duration": durationStr,
			"status":   status,
		})
		response.WriteString(entry)
		response.WriteString("\n")
	}

	return response.String(), nil
}

// ============================================================================
// Language Configuration Command (Feature 007)
// ============================================================================

// HandleSetLanguage handles /setlanguage command - sets the language preference
// Feature 009 DUAL MODE:
// - Without group identifier: /setlanguage en → sets USER language (private chat preference)
// - With group identifier: /setlanguage @home en → sets GROUP language (group notifications)
// cmdCtx.TargetGroup == 0 means no group identifier was provided
func (h *CommandHandler) HandleSetLanguage(ctx context.Context, cmdCtx CommandContext, args []string) (string, error) {
	// Feature 009: cmdCtx.TargetGroup already resolved in handler.go
	// - 0 = no group identifier (set user language)
	// - non-zero = group identifier provided (set group language)
	targetGroupID := cmdCtx.TargetGroup

	// Validate arguments (group identifier already removed by handler.go)
	if len(args) < 1 {
		return "", &ValidationError{localize(cmdCtx, "cmd_setlanguage_usage", nil)}
	}

	languageInput := strings.Join(args, " ")
	languageInput = strings.TrimSpace(languageInput)

	// Validate and normalize language
	normalizedLang, valid := i18nPkg.ValidateLanguage(languageInput)
	if !valid {
		return "", &ValidationError{localize(cmdCtx, "cmd_error_invalid_language", nil)}
	}

	// DUAL MODE IMPLEMENTATION
	if targetGroupID == 0 {
		// Mode 1: No group identifier → set USER language
		if err := h.store.SetUserLanguage(ctx, cmdCtx.UserID, normalizedLang); err != nil {
			return "", &ValidationError{localize(cmdCtx, "cmd_setlanguage_error_db", nil)}
		}

		// Return confirmation in NEW language
		newLocalizer := i18nPkg.GetLocalizer(normalizedLang)
		return i18nPkg.LocalizeSimple(newLocalizer, "language_changed"), nil
	}

	// Mode 2: Group identifier provided → set GROUP language (requires admin)
	isAdmin, err := h.client.IsAdmin(ctx, targetGroupID, cmdCtx.UserID)
	if err != nil {
		return "", fmt.Errorf("failed to check permissions: %w", err)
	}

	if !isAdmin {
		return "", &ValidationError{localize(cmdCtx, "permission_denied", nil)}
	}

	// Ensure group record exists
	if err := h.store.EnsureGroup(ctx, targetGroupID); err != nil {
		return "", fmt.Errorf("failed to ensure group record: %w", err)
	}

	// Update language preference for GROUP
	if err := h.store.SetGroupLanguage(ctx, targetGroupID, normalizedLang); err != nil {
		return "", &ValidationError{localize(cmdCtx, "cmd_setlanguage_error_db", nil)}
	}

	// Return confirmation in NEW language
	newLocalizer := i18nPkg.GetLocalizer(normalizedLang)
	return i18nPkg.LocalizeSimple(newLocalizer, "language_changed"), nil
}

// validateLanguageInput validates and normalizes language input
// DEPRECATED: Use i18nPkg.ValidateLanguage instead
func validateLanguageInput(input string) (string, bool) {
	return i18nPkg.ValidateLanguage(input)
}

// getLanguageName returns the display name for a language code
// DEPRECATED: Use i18nPkg.GetLanguageName instead
func getLanguageName(code string) string {
	// Get English localizer for language names
	localizer := i18nPkg.GetLocalizer("en")
	return i18nPkg.GetLanguageName(localizer, code)
}

// ============================================================================
// Group Pause/Resume Commands
// ============================================================================

// HandlePause handles /pause command - pauses ALL rate limiting for the group
func (h *CommandHandler) HandlePause(ctx context.Context, cmdCtx CommandContext, args []string) (string, error) {
	targetGroupID := cmdCtx.TargetGroup

	// Note: args already has group identifier removed by handler.go
	// In private chat: /pause @groupname 30m → args = ["30m"]
	// In group chat: /pause 30m → args = ["30m"]

	// Check admin permission on TARGET group
	isAdmin, err := h.client.IsAdmin(ctx, targetGroupID, cmdCtx.UserID)
	if err != nil {
		return "", fmt.Errorf("failed to check permissions: %w", err)
	}

	if !isAdmin {
		return "", &ValidationError{localize(cmdCtx, "cmd_pause_error_not_admin", nil)}
	}

	// Parse optional duration argument
	var resumeAt *time.Time
	if len(args) > 0 {
		durationValue, durationUnit, err := ParseDurationString(args[0])
		if err != nil {
			return "", &ValidationError{localize(cmdCtx, "cmd_pause_invalid_duration", map[string]interface{}{
				"error": err.Error(),
			})}
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

	// Pause the group (use targetGroupID, not cmdCtx.ChatID which is private chat in Feature 009)
	if err := h.store.SetGroupPaused(ctx, targetGroupID, true, resumeAt); err != nil {
		return "", fmt.Errorf("failed to pause group: %w", err)
	}

	if resumeAt != nil {
		// Calculate duration string for display
		durationStr := args[0] // Use the original duration string (e.g., "30m", "1h", "7d")
		return localize(cmdCtx, "cmd_pause_success_temporary", map[string]interface{}{
			"duration": durationStr,
			"until":    resumeAt.Format("2006-01-02 15:04:05"),
		}), nil
	}
	return localize(cmdCtx, "cmd_pause_success_indefinite", nil), nil
}

// HandleResume handles /resume command - resumes rate limiting for the group
func (h *CommandHandler) HandleResume(ctx context.Context, cmdCtx CommandContext) (string, error) {
	targetGroupID := cmdCtx.TargetGroup

	// Note: No args to strip - this command takes no arguments
	// In private chat: /resume @groupname
	// In group chat: /resume

	// Check admin permission on TARGET group
	isAdmin, err := h.client.IsAdmin(ctx, targetGroupID, cmdCtx.UserID)
	if err != nil {
		return "", fmt.Errorf("failed to check permissions: %w", err)
	}

	if !isAdmin {
		return "", &ValidationError{localize(cmdCtx, "cmd_resume_error_not_admin", nil)}
	}

	// Check if group is paused
	isPaused, err := h.store.IsGroupPaused(ctx, targetGroupID)
	if err != nil {
		return "", fmt.Errorf("failed to check pause status: %w", err)
	}

	if !isPaused {
		return "", &ValidationError{localize(cmdCtx, "cmd_resume_not_paused", nil)}
	}

	// Resume the group
	if err := h.store.SetGroupPaused(ctx, targetGroupID, false, nil); err != nil {
		return "", fmt.Errorf("failed to resume group: %w", err)
	}

	return localize(cmdCtx, "cmd_resume_success", nil), nil
}

// ============================================================================
// MyGroups Command (Feature 013)
// ============================================================================

// HandleMyGroups displays a paginated list of groups where user and bot are both members
// Available to all users (no admin check required - FR-001)
// Syntax: /mygroups [page_number]
func (h *CommandHandler) HandleMyGroups(ctx context.Context, cmdCtx CommandContext, args []string) (string, error) {
	// T048: Metrics instrumentation
	start := time.Now()
	var status string
	defer func() {
		metrics.RecordMyGroupsCommand(status, time.Since(start))
	}()

	const pageSize = 20 // FR-011: 20 groups per page

	// Parse page number from arguments (default: 1)
	pageNumber, err := parseMyGroupsArgs(args)
	if err != nil {
		status = "error"
		return "", &ValidationError{err.Error()}
	}

	// Get total count for pagination calculation
	totalGroups, err := h.store.CountUserGroups(ctx, cmdCtx.UserID)
	if err != nil {
		status = "error"
		return "", fmt.Errorf("failed to count groups: %w", err)
	}

	// Calculate pagination
	totalPages, currentPage, offset := calculatePagination(int(totalGroups), pageNumber, pageSize)

	// Validate page number is within bounds
	if totalGroups > 0 && (currentPage < 1 || currentPage > totalPages) {
		status = "error"
		return "", &ValidationError{
			localize(cmdCtx, "cmd_mygroups_invalid_page", map[string]interface{}{
				"totalPages": totalPages,
			}),
		}
	}

	// Fetch groups for current page (FR-002, FR-003, FR-006, FR-009)
	groups, err := h.store.GetUserGroupsWithRole(ctx, cmdCtx.UserID, int32(pageSize), int32(offset))
	if err != nil {
		status = "error"
		return "", fmt.Errorf("failed to get user groups: %w", err)
	}

	// Format and return response (FR-004, FR-008, FR-010, FR-011)
	status = "success"
	return formatGroupList(cmdCtx, groups, currentPage, totalPages), nil
}

// ParseMyGroupsArgs parses command arguments to extract page number (exported for testing)
// Returns page number (1-indexed) or error if invalid
func ParseMyGroupsArgs(args []string) (int, error) {
	return parseMyGroupsArgs(args)
}

// parseMyGroupsArgs parses command arguments to extract page number
// Returns page number (1-indexed) or error if invalid
func parseMyGroupsArgs(args []string) (int, error) {
	// No arguments: default to page 1
	if len(args) == 0 {
		return 1, nil
	}

	// Parse first argument as page number
	page, err := strconv.Atoi(args[0])
	if err != nil {
		return 0, fmt.Errorf("invalid page number: must be a positive integer")
	}

	// Validate page number is positive
	if page < 1 {
		return 0, fmt.Errorf("invalid page number: must be >= 1")
	}

	return page, nil
}

// CalculatePagination computes pagination values (exported for testing)
// Returns: totalPages, currentPage (clamped), offset
func CalculatePagination(totalGroups, requestedPage, pageSize int) (int, int, int) {
	return calculatePagination(totalGroups, requestedPage, pageSize)
}

// calculatePagination computes pagination values
// Returns: totalPages, currentPage (clamped), offset
func calculatePagination(totalGroups, requestedPage, pageSize int) (int, int, int) {
	// Calculate total pages (ceiling division)
	totalPages := (totalGroups + pageSize - 1) / pageSize
	if totalPages < 1 {
		totalPages = 1 // Always at least 1 page
	}

	// Clamp current page to valid range
	currentPage := requestedPage
	if currentPage < 1 {
		currentPage = 1
	}
	if currentPage > totalPages {
		currentPage = totalPages
	}

	// Calculate offset for database query
	offset := (currentPage - 1) * pageSize
	if offset < 0 {
		offset = 0
	}

	return totalPages, currentPage, offset
}

// FormatGroupList formats the group list with pagination info (exported for testing)
// Implements FR-004 (display name), FR-008 (localization), FR-010 (NULL handling), FR-011 (pagination)
func FormatGroupList(cmdCtx CommandContext, groups []ratelimit.UserGroupMembership, currentPage, totalPages int) string {
	return formatGroupList(cmdCtx, groups, currentPage, totalPages)
}

// formatGroupList formats the group list with pagination info
// Implements FR-004 (display name), FR-008 (localization), FR-010 (NULL handling), FR-011 (pagination)
func formatGroupList(cmdCtx CommandContext, groups []ratelimit.UserGroupMembership, currentPage, totalPages int) string {
	// FR-005: Empty state handling
	if len(groups) == 0 {
		return localize(cmdCtx, "cmd_mygroups_empty", nil)
	}

	// Build response with header
	var response strings.Builder

	// Header with optional pagination info
	if totalPages > 1 {
		response.WriteString(localize(cmdCtx, "cmd_mygroups_header_paginated", map[string]interface{}{
			"current": currentPage,
			"total":   totalPages,
		}))
	} else {
		response.WriteString(localize(cmdCtx, "cmd_mygroups_header", nil))
	}
	response.WriteString("\n\n")

	// List groups with numbering
	for i, group := range groups {
		// Number (1-indexed per page, not global)
		response.WriteString(fmt.Sprintf("%d. ", i+1))

		// Group name (FR-010: handle NULL titles) + Feature 014: show @username when available
		groupDisplayName := ""
		if group.Title != nil && *group.Title != "" {
			groupDisplayName = *group.Title
			// Add @username if available (public groups/channels)
			if group.Username != nil && *group.Username != "" {
				groupDisplayName = fmt.Sprintf("%s (@%s)", groupDisplayName, *group.Username)
			}
		} else if group.Username != nil && *group.Username != "" {
			// Fallback to @username if no title
			groupDisplayName = fmt.Sprintf("@%s", *group.Username)
		} else {
			// Last resort: unnamed group with ID
			groupDisplayName = localize(cmdCtx, "cmd_mygroups_unnamed_group", map[string]interface{}{
				"chatID": group.ChatID,
			})
		}
		response.WriteString(groupDisplayName)

		// Role indicator (FR-003: admin or user)
		response.WriteString(" - ")
		if group.IsAdmin {
			response.WriteString(localize(cmdCtx, "cmd_mygroups_role_admin", nil))
		} else {
			response.WriteString(localize(cmdCtx, "cmd_mygroups_role_user", nil))
		}

		response.WriteString("\n")
	}

	// Pagination navigation (FR-011)
	if totalPages > 1 && currentPage < totalPages {
		response.WriteString(localize(cmdCtx, "cmd_mygroups_next_page", map[string]interface{}{
			"page": currentPage + 1,
		}))
	}

	return response.String()
}
