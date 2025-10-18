// Package bot provides Telegram bot handlers and command processing.
// It handles incoming messages, rate limiting, and administrative commands.
package bot

import (
	"context"
	"fmt"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/antst/tg-throttle-bot/internal/logging"
	"github.com/antst/tg-throttle-bot/internal/metrics"
	"github.com/antst/tg-throttle-bot/internal/ratelimit"
	"github.com/antst/tg-throttle-bot/internal/telegram"
)

// Handler processes incoming messages
type Handler struct {
	client         telegram.Client
	store          ratelimit.MultiWindowStorage
	commandHandler *CommandHandler
	logger         *logging.Logger
}

// NewHandler creates a new message handler with the given storage and Telegram client.
func NewHandler(store ratelimit.MultiWindowStorage, client telegram.Client) *Handler {
	// Create a default logger if none provided
	logger, _ := logging.NewDevelopmentLogger()
	return &Handler{
		client:         client,
		store:          store,
		commandHandler: NewCommandHandler(store, client),
		logger:         logger,
	}
}

// NewHandlerWithLogger creates a new message handler with a custom logger.
func NewHandlerWithLogger(store ratelimit.MultiWindowStorage, client telegram.Client, logger *logging.Logger) *Handler {
	return &Handler{
		client:         client,
		store:          store,
		commandHandler: NewCommandHandler(store, client), // logger parameter removed in clean commands.go
		logger:         logger,
	}
}

// HandleUpdate processes a single update from Telegram.
// It routes commands to the command handler and regular messages to rate limit checking.
func (h *Handler) HandleUpdate(ctx context.Context, update tgbotapi.Update) error {
	if update.Message == nil {
		return nil
	}

	msg := update.Message

	// Handle commands
	if msg.IsCommand() {
		return h.handleCommand(ctx, msg)
	}

	// Check rate limit for regular messages
	return h.handleMessage(ctx, msg)
}

// handleMessage checks rate limits and deletes messages if needed
func (h *Handler) handleMessage(ctx context.Context, msg *tgbotapi.Message) error {
	chatID := msg.Chat.ID
	userID := msg.From.ID

	// Harvest username for @username syntax support (Feature 006 - User Story 6)
	// Extract username from Telegram message and update database on EVERY message
	username := msg.From.UserName // May be empty string if user has no username
	h.logger.Infow(
		"DEBUG: Username harvesting",
		"user_id", userID,
		"username", username,
		"username_length", len(username),
	)
	if err := h.store.EnsureUser(ctx, userID, username); err != nil {
		h.logger.Warnw(
			"Failed to ensure user record (username harvesting)",
			"user_id", userID,
			"username", username,
			"error", err,
		)
		// Non-fatal: Continue processing even if username update fails
	} else {
		h.logger.Infow(
			"Successfully ensured user record",
			"user_id", userID,
			"username", username,
		)
	}

	// Track message processing
	chatType := msg.Chat.Type
	metrics.RecordMessage(chatType)

	h.logger.Infow("DEBUG: Starting rate limit checks", "chat_id", chatID, "user_id", userID)

	// Check if rate limiting is paused for this group
	isPaused, err := h.store.IsGroupPaused(ctx, chatID)
	if err != nil {
		h.logger.Errorw(
			"Failed to check if group is paused",
			"chat_id", chatID,
			"error", err,
		)
		metrics.RecordError("check_paused_failed", "handleMessage")
	}
	if isPaused {
		// Group is paused, allow all messages
		h.logger.Infow("DEBUG: Group is paused, skipping rate limit", "chat_id", chatID)
		return nil
	}
	h.logger.Infow("DEBUG: Group not paused", "chat_id", chatID)

	// Check manual override (3-state: nil/true/false)
	overrideState, err := h.store.GetUserOverride(ctx, chatID, userID)
	if err != nil {
		h.logger.Warnw(
			"Failed to check user override",
			"chat_id", chatID,
			"user_id", userID,
			"error", err,
		)
		metrics.RecordError("check_override_failed", "handleMessage")
	}
	h.logger.Infow("DEBUG: Checked override", "override_state", overrideState)
	if overrideState != nil {
		if *overrideState {
			// Whitelist: User is manually exempted, allow message
			h.logger.Infow("DEBUG: User whitelisted, skipping rate limit", "user_id", userID)
			metrics.RecordRateLimitCheck("whitelisted")
			return nil
		} else {
			// Blacklist: User is manually restricted, delete message
			h.logger.Infow(
				"Deleting message from blacklisted user",
				"chat_id", chatID,
				"user_id", userID,
				"message_id", msg.MessageID,
			)
			if err := h.client.DeleteMessage(chatID, msg.MessageID); err != nil {
				h.logger.Errorw(
					"Failed to delete blacklisted message",
					"chat_id", chatID,
					"message_id", msg.MessageID,
					"error", err,
				)
				metrics.RecordError("delete_blacklisted_failed", "handleMessage")
			}
			metrics.RecordRateLimitCheck("blacklisted")
			return nil
		}
	}

	// REMOVED: Exemption checking (exemptions table removed in migration 000007)
	// User exemptions now handled via user_overrides table (checked above)
	// Role-based exemptions were never implemented (exemptions table had 0 rows)
	// If role-based exemptions needed in future, implement via GetUserRoles + user_overrides loop

	// Use multi-window evaluation (Feature 006)
	h.logger.Infow("DEBUG: Calling handleMessageMultiWindow", "chat_id", chatID, "user_id", userID)
	return h.handleMessageMultiWindow(ctx, msg, h.store)
}

// handleMessageMultiWindow handles rate limiting with multi-window support (Feature 006)
// FR-003: Evaluates all enabled windows and restricts if ANY limit is exceeded
// FR-022: Must complete within 500ms per message (SC-003)
func (h *Handler) handleMessageMultiWindow(ctx context.Context, msg *tgbotapi.Message, multiStore ratelimit.MultiWindowStorage) error {
	chatID := msg.Chat.ID
	userID := msg.From.ID
	messageID := int64(msg.MessageID)
	text := msg.Text
	charCount := len([]rune(text))

	// Create a limiter with multi-window support
	limiter := ratelimit.NewLimiter(multiStore)

	// Evaluate all enabled windows
	result, err := limiter.CheckMessageMultiWindow(ctx, chatID, userID, text)
	if err != nil {
		h.logger.Errorw(
			"Failed to evaluate multi-window rate limits",
			"chat_id", chatID,
			"user_id", userID,
			"error", err,
		)
		metrics.RecordError("multi_window_eval_failed", "handleMessage")
		return nil // Don't block on errors
	}

	// If no windows enabled, allow message
	if len(result.Windows) == 0 {
		metrics.RecordRateLimitCheck("no_windows")
		return nil
	}

	// ============================================================================
	// Simple 3-State Processing: Check result and take action
	// ============================================================================
	// REMOVED: Automatic restriction checking (GetActiveMultiWindowRestriction)
	// REMOVED: Recovery status checking (CheckRecoveryStatus)
	// REMOVED: Automatic unrestriction (UnrestrictUser)
	//
	// New design: Messages are simply DELETED when over limit
	// No restriction records created for automatic rate limiting
	// Recovery is automatic - old messages age out of sliding window
	// ============================================================================

	// If message is allowed, record it for all windows
	if result.AllowMessage {
		if err := limiter.RecordMessageForAllWindows(ctx, chatID, userID, charCount); err != nil {
			h.logger.Errorw(
				"Failed to record message for multi-window",
				"chat_id", chatID,
				"user_id", userID,
				"error", err,
			)
			metrics.RecordError("multi_window_record_failed", "handleMessage")
		}

		// Send warnings for newly crossed thresholds
		if len(result.NewWarnings) > 0 {
			h.sendMultiWindowWarnings(ctx, chatID, userID, result.NewWarnings)
		}

		metrics.RecordRateLimitCheck("allowed")
		return nil
	}

	// User violated at least one window - enforce restriction
	return h.enforceMultiWindowRateLimit(ctx, userID, chatID, messageID, result.ViolatedWindows, result.Windows)
}

// REMOVED: isUserExempt function (exemptions table removed in migration 000007)
// Exemptions functionality replaced by user_overrides table with /override command
// See: docs/SCHEMA_CLEANUP_ANALYSIS.md for rationale

// handleCommand processes bot commands
func (h *Handler) handleCommand(ctx context.Context, msg *tgbotapi.Message) error {
	chatID := msg.Chat.ID
	userID := msg.From.ID

	// Parse command
	cmd, err := ParseCommand(msg.Text)
	if err != nil {
		h.logger.Warnw(
			"Failed to parse command",
			"chat_id", chatID,
			"user_id", userID,
			"text", msg.Text,
			"error", err,
		)
		metrics.RecordError("parse_command_failed", "handleCommand")
		return nil // Don't fail on parse errors
	}

	// Track command execution time
	startTime := time.Now()
	chatType := msg.Chat.Type

	var response string
	var cmdErr error

	// Route commands
	switch cmd.Name {
	case "start", "help":
		response, cmdErr = h.commandHandler.HandleHelp(ctx, chatID, userID)

	case "override":
		response, cmdErr = h.commandHandler.HandleOverride(ctx, chatID, userID, cmd.Args)

	case "overrides":
		response, cmdErr = h.commandHandler.HandleListOverrides(ctx, chatID, userID)

	case "checkuser":
		response, cmdErr = h.commandHandler.HandleCheckUser(ctx, chatID, userID, cmd.Args)

	case "pause":
		response, cmdErr = h.commandHandler.HandlePause(ctx, chatID, userID, cmd.Args)

	case "resume":
		response, cmdErr = h.commandHandler.HandleResume(ctx, chatID, userID)

	case "setwindow":
		response, cmdErr = h.commandHandler.HandleSetWindow(ctx, chatID, userID, cmd.Args)

	case "disablewindow":
		response, cmdErr = h.commandHandler.HandleDisableWindow(ctx, chatID, userID, cmd.Args)

	case "enablewindow":
		response, cmdErr = h.commandHandler.HandleEnableWindow(ctx, chatID, userID, cmd.Args)

	case "windows":
		response, cmdErr = h.commandHandler.HandleShowWindows(ctx, chatID, userID, cmd.Args)

	case "mystatus":
		response, cmdErr = h.commandHandler.HandleMyStatus(ctx, chatID, userID)

	default:
		response = fmt.Sprintf("❌ Unknown command: %s\nUse /help to see available commands", cmd.Name)
	}

	// Record command execution metrics
	duration := time.Since(startTime)
	metrics.RecordCommand(cmd.Name, chatType, duration)

	// Handle command errors - sanitize before sending to user
	if cmdErr != nil {
		h.logger.Errorw(
			"Command execution failed",
			"command", cmd.Name,
			"chat_id", chatID,
			"user_id", userID,
			"error", cmdErr,
		)
		metrics.RecordError("command_execution_failed", cmd.Name)
		// Use sanitized error message for user
		response = SanitizeError(cmdErr)
	} else {
		h.logger.Infow(
			"Command executed successfully",
			"command", cmd.Name,
			"chat_id", chatID,
			"user_id", userID,
			"duration_ms", duration.Milliseconds(),
		)
	}

	// Send response if we have one
	if response != "" {
		if err := h.client.SendMessage(chatID, response); err != nil {
			h.logger.Errorw(
				"Failed to send command response",
				"command", cmd.Name,
				"chat_id", chatID,
				"error", err,
			)
			metrics.RecordError("send_command_response_failed", "handleCommand")
			return err
		}
	}

	return nil
}

// sendMultiWindowWarnings sends warning notifications for newly crossed thresholds
// FR-016a: Delivers warnings as ephemeral messages
func (h *Handler) sendMultiWindowWarnings(ctx context.Context, chatID, userID int64, warnings []*ratelimit.WindowEvaluationResult) {
	for _, warning := range warnings {
		warningMsg := fmt.Sprintf(
			"⚠️ Window %s Warning: %d%% limit reached\n"+
				"• Current usage: %d/%d chars\n"+
				"• Window type: %s",
			strings.ToUpper(warning.SlotID),
			warning.Percentage,
			warning.CharCount,
			warning.CharLimit,
			getWindowTypeFromResult(warning),
		)

		// Send as ephemeral message (7s auto-delete)
		config := &telegram.EphemeralMessageConfig{
			DeleteAfter: 7 * time.Second,
		}
		if _, err := h.client.SendEphemeralMessage(chatID, warningMsg, config); err != nil {
			h.logger.Warnw(
				"Failed to send window warning",
				"chat_id", chatID,
				"user_id", userID,
				"window", warning.SlotID,
				"error", err,
			)
		}
	}
}

// enforceMultiWindowRateLimit enforces restriction when any window is violated
// FR-013, FR-014: Identifies violated windows and sends clear notifications
func (h *Handler) enforceMultiWindowRateLimit(
	ctx context.Context,
	userID, chatID, messageID int64,
	violatedWindows []string,
	allWindows []*ratelimit.WindowEvaluationResult,
) error {
	// ============================================================================
	// Simple Enforcement: DELETE message, DON'T create restriction records
	// ============================================================================
	// REMOVED: CreateMultiWindowRestriction (no DB record for automatic enforcement)
	// REMOVED: RestrictUser in Telegram (no permission changes)
	//
	// New design:
	// 1. Delete the violating message
	// 2. Send notification explaining which window(s) were violated
	// 3. DON'T record the message (it never happened)
	// 4. Recovery is automatic - old messages age out naturally
	// ============================================================================

	// Delete the violating message
	if err := h.client.DeleteMessage(chatID, int(messageID)); err != nil {
		h.logger.Warnw(
			"Failed to delete message after multi-window violation",
			"chat_id", chatID,
			"message_id", messageID,
			"error", err,
		)
	}

	// Build violation notification listing ALL violated windows
	notificationMsg := h.buildMultiWindowViolationMessage(violatedWindows, allWindows)

	// Send as ephemeral notification
	config := &telegram.EphemeralMessageConfig{
		DeleteAfter: 7 * time.Second,
	}
	if _, err := h.client.SendEphemeralMessage(chatID, notificationMsg, config); err != nil {
		h.logger.Warnw(
			"Failed to send multi-window violation notification",
			"chat_id", chatID,
			"error", err,
		)
	}

	metrics.RecordRateLimitEnforcement("multi_window_violated", strings.Join(violatedWindows, ","))

	return nil
}

// buildMultiWindowViolationMessage builds notification message for violated windows
// FR-014: Lists all violated windows in priority order (A → B → C)
func (h *Handler) buildMultiWindowViolationMessage(
	violatedWindows []string,
	allWindows []*ratelimit.WindowEvaluationResult,
) string {
	msg := "🚫 Rate Limit Exceeded\n\n"

	if len(violatedWindows) == 1 {
		msg += "You exceeded the limit for:\n"
	} else {
		msg += fmt.Sprintf("You exceeded limits for %d windows:\n", len(violatedWindows))
	}

	// Build map for easy lookup
	windowMap := make(map[string]*ratelimit.WindowEvaluationResult)
	for _, w := range allWindows {
		windowMap[w.SlotID] = w
	}

	// List violated windows in order
	for _, slotID := range violatedWindows {
		if window, ok := windowMap[slotID]; ok {
			msg += fmt.Sprintf("\n• Window %s: %d/%d chars (%d%%)\n",
				strings.ToUpper(slotID),
				window.CharCount,
				window.CharLimit,
				window.Percentage,
			)
		}
	}

	msg += "\n⏳ Your message was deleted.\n" +
		"Wait for your usage to drop below the limit before sending more messages."

	return msg
}

// getWindowTypeFromResult is a helper to extract window type from evaluation result
func getWindowTypeFromResult(result *ratelimit.WindowEvaluationResult) string {
	// This is a simplified version - in production, we'd lookup from database
	if result.CharLimit <= 100 {
		return "minute"
	} else if result.CharLimit <= 1000 {
		return "hour"
	}
	return "day"
}
