// Package bot provides Telegram bot handlers and command processing.
// It handles incoming messages, rate limiting, and administrative commands.
package bot

import (
	"context"
	"fmt"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/nicksnyder/go-i18n/v2/i18n"

	i18nPkg "github.com/antst/tg-throttle-bot/internal/i18n"
	"github.com/antst/tg-throttle-bot/internal/logging"
	"github.com/antst/tg-throttle-bot/internal/metrics"
	"github.com/antst/tg-throttle-bot/internal/ratelimit"
	"github.com/antst/tg-throttle-bot/internal/storage"
	"github.com/antst/tg-throttle-bot/internal/telegram"
)

// Handler processes incoming messages
type Handler struct {
	client          telegram.Client
	store           ratelimit.MultiWindowStorage
	commandHandler  *CommandHandler
	logger          *logging.Logger
	router          *MessageRouter  // Three-category message routing
	syncCoordinator SyncCoordinator // Feature 011: Proactive member sync
}

// SyncCoordinator defines the interface for proactive member synchronization
type SyncCoordinator interface {
	InitialGroupSync(ctx context.Context, chatID int64) error
	ProcessMemberJoin(ctx context.Context, chatID int64, member tgbotapi.ChatMember) error
	ProcessMemberLeave(ctx context.Context, chatID int64, userID int64) error
	ProcessMemberKicked(ctx context.Context, chatID int64, userID int64, reason string) error
	ProcessMemberRoleChange(ctx context.Context, chatID int64, member tgbotapi.ChatMember) error
}

// NewHandler creates a new message handler with the given storage and Telegram client.
func NewHandler(store ratelimit.MultiWindowStorage, client telegram.Client) *Handler {
	// Create a default logger if none provided
	logger, _ := logging.NewDevelopmentLogger()
	botAPI := client.GetBotAPI()
	return &Handler{
		client:         client,
		store:          store,
		commandHandler: NewCommandHandler(store, client),
		logger:         logger,
		router:         NewMessageRouter(botAPI, logger.Desugar()),
	}
}

// NewHandlerWithLogger creates a new message handler with a custom logger.
func NewHandlerWithLogger(store ratelimit.MultiWindowStorage, client telegram.Client, logger *logging.Logger) *Handler {
	botAPI := client.GetBotAPI()
	return &Handler{
		client:          client,
		store:           store,
		commandHandler:  NewCommandHandler(store, client), // logger parameter removed in clean commands.go
		logger:          logger,
		router:          NewMessageRouter(botAPI, logger.Desugar()),
		syncCoordinator: nil, // Will be set via SetSyncCoordinator() after creation
	}
}

// SetSyncCoordinator sets the sync coordinator for proactive member synchronization (Feature 011)
func (h *Handler) SetSyncCoordinator(coordinator SyncCoordinator) {
	h.syncCoordinator = coordinator
}

// GetRouter returns the handler's MessageRouter for use in callbacks
func (h *Handler) GetRouter() *MessageRouter {
	return h.router
}

// getLocalizer retrieves the appropriate localizer for a chat based on its language preference
func (h *Handler) getLocalizer(ctx context.Context, chatID int64) *i18n.Localizer {
	lang := "en" // default
	if groupLang, err := h.store.GetGroupLanguage(ctx, chatID); err == nil {
		lang = groupLang
	}
	return i18nPkg.GetLocalizer(lang)
}

// isGroupCommand returns true if the command requires a group context
// (either @groupname or -ID argument in private chat, or operates on current group in group chat)
func isGroupCommand(cmdName string) bool {
	groupCommands := map[string]bool{
		// State-changing commands (send group notifications)
		"setwindow":     true,
		"disablewindow": true,
		"enablewindow":  true,
		"setlanguage":   true, // Can be user or group command
		"pause":         true,
		"resume":        true,
		"override":      true,
		// Read-only commands (no group notifications)
		"config":    true,
		"mystatus":  true,
		"checkuser": true,
		"overrides": true,
	}
	return groupCommands[cmdName]
}

// sendsGroupNotification returns true if the command sends a notification to the group
// Only state-changing commands send notifications
func sendsGroupNotification(cmdName string) bool {
	notificationCommands := map[string]bool{
		"setwindow":     true,
		"disablewindow": true,
		"enablewindow":  true,
		"setlanguage":   true, // Only when targeting a group (checked at call site)
		"pause":         true,
		"resume":        true,
		"override":      true,
	}
	return notificationCommands[cmdName]
}

// formatPolicyGroupNotification creates a concise group notification for policy changes.
// Per spec: "less than 10 words" for group messages.
func formatPolicyGroupNotification(cmd Command, cleanedArgs []string, username string, groupID int64, localizer *i18n.Localizer) string {
	// Use @username if available, otherwise "admin"
	actor := username
	if actor == "" {
		actor = "admin"
	} else {
		actor = "@" + actor
	}

	// Format notification based on command type
	switch cmd.Name {
	case "setwindow":
		// Extract slot from cleanedArgs (group identifier already removed)
		slot := ""
		if len(cleanedArgs) > 0 {
			slot = strings.ToUpper(cleanedArgs[0])
		}
		return localizer.MustLocalize(&i18n.LocalizeConfig{
			DefaultMessage: &i18n.Message{
				ID:    "group.notification.window_updated",
				Other: "Window {{.Slot}} updated by {{.Actor}}",
			},
			TemplateData: map[string]string{
				"Slot":  slot,
				"Actor": actor,
			},
		})

	case "enablewindow":
		slot := ""
		if len(cleanedArgs) > 0 {
			slot = strings.ToUpper(cleanedArgs[0])
		}
		return localizer.MustLocalize(&i18n.LocalizeConfig{
			DefaultMessage: &i18n.Message{
				ID:    "group.notification.window_enabled",
				Other: "Window {{.Slot}} enabled by {{.Actor}}",
			},
			TemplateData: map[string]string{
				"Slot":  slot,
				"Actor": actor,
			},
		})

	case "disablewindow":
		slot := ""
		if len(cleanedArgs) > 0 {
			slot = strings.ToUpper(cleanedArgs[0])
		}
		return localizer.MustLocalize(&i18n.LocalizeConfig{
			DefaultMessage: &i18n.Message{
				ID:    "group.notification.window_disabled",
				Other: "Window {{.Slot}} disabled by {{.Actor}}",
			},
			TemplateData: map[string]string{
				"Slot":  slot,
				"Actor": actor,
			},
		})

	case "setlanguage":
		lang := ""
		if len(cleanedArgs) > 0 {
			lang = cleanedArgs[0]
		}
		return localizer.MustLocalize(&i18n.LocalizeConfig{
			DefaultMessage: &i18n.Message{
				ID:    "group.notification.language_changed",
				Other: "Language changed to {{.Language}} by {{.Actor}}",
			},
			TemplateData: map[string]string{
				"Language": lang,
				"Actor":    actor,
			},
		})

	case "pause":
		return localizer.MustLocalize(&i18n.LocalizeConfig{
			DefaultMessage: &i18n.Message{
				ID:    "group.notification.group_paused",
				Other: "Rate limiting paused by {{.Actor}}",
			},
			TemplateData: map[string]string{
				"Actor": actor,
			},
		})

	case "resume":
		return localizer.MustLocalize(&i18n.LocalizeConfig{
			DefaultMessage: &i18n.Message{
				ID:    "group.notification.group_resumed",
				Other: "Rate limiting resumed by {{.Actor}}",
			},
			TemplateData: map[string]string{
				"Actor": actor,
			},
		})

	default:
		// Fallback for unknown policy commands
		return localizer.MustLocalize(&i18n.LocalizeConfig{
			DefaultMessage: &i18n.Message{
				ID:    "group.notification.config_changed",
				Other: "Configuration changed by {{.Actor}}",
			},
			TemplateData: map[string]string{
				"Actor": actor,
			},
		})
	}
}

// HandleUpdate processes a single update from Telegram.
// It routes commands to the command handler and regular messages to rate limit checking.
// Routes member sync updates (my_chat_member, chat_member) to sync handlers (Feature 011).
func (h *Handler) HandleUpdate(ctx context.Context, update tgbotapi.Update) error {
	// Feature 011: Handle bot status changes (my_chat_member)
	if update.MyChatMember != nil {
		return h.handleMyChatMember(ctx, update.MyChatMember)
	}

	// Feature 011: Handle member status changes (chat_member)
	if update.ChatMember != nil {
		return h.handleChatMember(ctx, update.ChatMember)
	}

	if update.Message == nil {
		return nil
	}

	msg := update.Message

	// Feature 009: Commands are ONLY processed from private chats
	// Commands in groups are treated as regular messages for rate limiting
	if msg.IsCommand() && msg.Chat.Type == "private" {
		return h.handleCommand(ctx, msg)
	}

	// Check rate limit for regular messages (including commands sent in groups)
	return h.handleMessage(ctx, msg)
}

// handleMessage checks rate limits and deletes messages if needed
func (h *Handler) handleMessage(ctx context.Context, msg *tgbotapi.Message) error {
	chatID := msg.Chat.ID
	userID := msg.From.ID

	// Harvest username for @username syntax support (Feature 006 - User Story 6)
	// Extract username from Telegram message and update database on EVERY message
	username := msg.From.UserName // May be empty string if user has no username
	h.logger.Debugw(
		"Username harvesting",
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

	// Feature 011 Phase 6: Reactive fallback for member synchronization
	// Ensures group and membership records exist even if real-time updates were missed
	if msg.Chat.IsGroup() || msg.Chat.IsSuperGroup() {
		// Type assert to access storage methods beyond ratelimit.MultiWindowStorage interface
		fullStore, ok := h.store.(*storage.RateLimitStorage)
		if !ok {
			h.logger.Warn("Store is not *storage.RateLimitStorage, skipping reactive fallback")
		} else {
			// Ensure group record exists (with username if available)
			groupUsername := msg.Chat.UserName // May be empty for private groups
			if err := fullStore.EnsureGroupWithUsername(ctx, chatID, groupUsername); err != nil {
				h.logger.Warnw(
					"Failed to ensure group record (reactive fallback)",
					"chat_id", chatID,
					"username", groupUsername,
					"error", err,
				)
				// Non-fatal: Continue processing
			}

			// Ensure membership record exists (reactive fallback)
			if h.syncCoordinator != nil {
				// Try to determine if user is admin via fresh API call
				// Note: This adds latency, but provides accurate role information
				isAdmin := false
				member, err := h.client.GetChatMember(ctx, chatID, userID)
				if err == nil {
					isAdmin = member.IsAdministrator() || member.IsCreator()
				}

				// Upsert membership with current state
				_, err = fullStore.UpsertGroupMembership(ctx, storage.UpsertGroupMembershipParams{
					ChatID:          chatID,
					UserID:          userID,
					Status:          storage.MembershipActive,
					JoinedAt:        time.Now(), // Approximate: actual join time unknown
					LeftAt:          nil,
					IsAdmin:         isAdmin,
					CanSendMessages: true, // Assume true if message was sent successfully
				})
				if err != nil {
					h.logger.Warnw(
						"Failed to upsert membership (reactive fallback)",
						"chat_id", chatID,
						"user_id", userID,
						"error", err,
					)
					// Non-fatal: Continue processing
				} else {
					h.logger.Debugw(
						"Reactive fallback: ensured membership",
						"chat_id", chatID,
						"user_id", userID,
						"is_admin", isAdmin,
					)
				}
			}
		}
	}

	// Track message processing
	chatType := msg.Chat.Type
	metrics.RecordMessage(chatType)

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
		h.logger.Debugw("Group is paused, skipping rate limit", "chat_id", chatID)
		return nil
	}

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
	if overrideState != nil {
		if *overrideState {
			// Whitelist: User is manually exempted, allow message
			h.logger.Debugw("User whitelisted, skipping rate limit", "user_id", userID)
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

		// Send warnings for newly crossed thresholds (≥95%)
		if len(result.NewWarnings) > 0 {
			groupName := msg.Chat.Title
			if groupName == "" {
				groupName = fmt.Sprintf("Chat %d", chatID)
			}
			h.sendMultiWindowWarnings(ctx, chatID, userID, groupName, result.NewWarnings)
		}

		metrics.RecordRateLimitCheck("allowed")
		return nil
	}

	// User violated at least one window - enforce restriction
	groupName := msg.Chat.Title
	if groupName == "" {
		groupName = fmt.Sprintf("Chat %d", chatID)
	}
	return h.enforceMultiWindowRateLimit(ctx, userID, chatID, messageID, result.ViolatedWindows, result.Windows, groupName)
}

// REMOVED: isUserExempt function (exemptions table removed in migration 000007)
// Exemptions functionality replaced by user_overrides table with /override command
// See: docs/SCHEMA_CLEANUP_ANALYSIS.md for rationale

// handleCommand processes bot commands (private chat only since Feature 009)
func (h *Handler) handleCommand(ctx context.Context, msg *tgbotapi.Message) error {
	chatID := msg.Chat.ID
	userID := msg.From.ID
	username := msg.From.UserName

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

	// Feature 009: All commands come from private chat
	// Group commands parse @groupname or -groupID from first arg
	targetGroupID := int64(0) // Default: no group context
	remainingArgs := cmd.Args

	if isGroupCommand(cmd.Name) {
		// Group command → resolve target group from first arg (@groupname or -ID)
		// Exception: /setlanguage can work without group arg to set USER language
		if cmd.Name == "setlanguage" && (len(cmd.Args) == 0 || (len(cmd.Args) > 0 && !strings.HasPrefix(cmd.Args[0], "@") && !strings.HasPrefix(cmd.Args[0], "-"))) {
			// No group identifier - this sets user language
			targetGroupID = 0
			remainingArgs = cmd.Args
		} else {
			// Has group identifier or other policy command
			identifier, resolvedArgs, err := ResolveTargetGroup(chatID, cmd.Args)
			if err != nil {
				// Invalid or missing group identifier
				h.logger.Warnw(
					"Failed to resolve target group",
					"chat_id", chatID,
					"user_id", userID,
					"command", cmd.Name,
					"error", err,
				)
				metrics.RecordError("resolve_target_group_failed", cmd.Name)
				// Send error to user
				if err := h.router.SendCommandResponse(ctx, userID, SanitizeError(err)); err != nil {
					h.logger.Errorw("Failed to send error response", "error", err)
				}
				return nil // Error handled
			}

			// Resolve username to group ID if needed
			resolvedGroupID, err := ResolveGroupID(identifier, func(username string) (int64, error) {
				return h.store.GetGroupByUsername(ctx, username)
			})
			if err != nil {
				h.logger.Warnw(
					"Failed to resolve group username",
					"username", identifier.Username,
					"chat_id", chatID,
					"user_id", userID,
					"command", cmd.Name,
					"error", err,
				)
				metrics.RecordError("resolve_group_username_failed", cmd.Name)
				// Send error to user
				if sendErr := h.router.SendCommandResponse(ctx, userID, SanitizeError(err)); sendErr != nil {
					h.logger.Errorw("Failed to send error response", "error", sendErr)
				}
				return nil // Error handled
			}

			targetGroupID = resolvedGroupID
			remainingArgs = resolvedArgs

			// DEBUG: Log what we're passing to commands
			h.logger.Infow(
				"Resolved group and args",
				"command", cmd.Name,
				"original_args", cmd.Args,
				"resolved_args", remainingArgs,
				"resolved_args_count", len(remainingArgs),
				"target_group_id", targetGroupID,
			)
		}
	}

	// Fetch user's language preference once (Feature 009: all command responses use user's language)
	userLang, err := h.store.GetUserLanguage(ctx, userID)
	if err != nil || userLang == nil || *userLang == "" {
		defaultLang := "en"
		userLang = &defaultLang
	}

	// Create command context (Feature 009: private chat only, simplified)
	cmdCtx := CommandContext{
		ChatID:       chatID,
		UserID:       userID,
		ChatType:     "private",     // Feature 009: All commands from private chat
		TargetGroup:  targetGroupID, // Resolved from @username or -ID
		UserLanguage: *userLang,     // User's preferred language
	}

	var response string
	var cmdErr error

	// Route commands
	switch cmd.Name {
	case "start", "help":
		response, cmdErr = h.commandHandler.HandleHelp(ctx, cmdCtx, remainingArgs)

	case "override":
		response, cmdErr = h.commandHandler.HandleOverride(ctx, cmdCtx, remainingArgs)

	case "overrides":
		response, cmdErr = h.commandHandler.HandleListOverrides(ctx, cmdCtx)

	case "checkuser":
		response, cmdErr = h.commandHandler.HandleCheckUser(ctx, cmdCtx, remainingArgs)

	case "pause":
		response, cmdErr = h.commandHandler.HandlePause(ctx, cmdCtx, remainingArgs)

	case "resume":
		response, cmdErr = h.commandHandler.HandleResume(ctx, cmdCtx)

	case "setwindow":
		response, cmdErr = h.commandHandler.HandleSetWindow(ctx, cmdCtx, remainingArgs)

	case "setlanguage":
		response, cmdErr = h.commandHandler.HandleSetLanguage(ctx, cmdCtx, remainingArgs)

	case "disablewindow":
		response, cmdErr = h.commandHandler.HandleDisableWindow(ctx, cmdCtx, remainingArgs)

	case "enablewindow":
		response, cmdErr = h.commandHandler.HandleEnableWindow(ctx, cmdCtx, remainingArgs)

	case "config":
		response, cmdErr = h.commandHandler.HandleConfig(ctx, cmdCtx, remainingArgs)

	case "mystatus":
		response, cmdErr = h.commandHandler.HandleMyStatus(ctx, cmdCtx)

	default:
		// Feature 009 US4: Localized unknown command error
		localizer := i18nPkg.GetLocalizer(cmdCtx.UserLanguage)
		response = i18nPkg.Localize(localizer, "unknown_command", map[string]interface{}{
			"command": cmd.Name,
		})
	}

	// Record command execution metrics
	duration := time.Since(startTime)
	metrics.RecordCommand(cmd.Name, "private", duration) // Feature 009: always private

	// Handle command errors
	isSuccess := true
	if cmdErr != nil {
		// Check if it's a validation error or system error
		if validationErr, ok := cmdErr.(*ValidationError); ok {
			// Validation error: user-facing message, no group notification
			response = validationErr.Message
			isSuccess = false
			h.logger.Infow(
				"Command validation failed",
				"command", cmd.Name,
				"chat_id", chatID,
				"user_id", userID,
				"duration_ms", duration.Milliseconds(),
			)
		} else {
			// System error: sanitize before showing to user, no group notification
			response = SanitizeError(cmdErr)
			isSuccess = false
			h.logger.Errorw(
				"Command execution failed",
				"command", cmd.Name,
				"chat_id", chatID,
				"user_id", userID,
				"error", cmdErr,
			)
			metrics.RecordError("command_execution_failed", cmd.Name)
		}
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
		// Group commands that succeeded may send dual messages (private + group notification)
		// BUT ONLY if command is state-changing AND succeeded (isSuccess = true)
		if sendsGroupNotification(cmd.Name) && targetGroupID != 0 && isSuccess {
			// Format group notification based on command (use remainingArgs with group identifier removed)
			groupNotification := formatPolicyGroupNotification(*cmd, remainingArgs, username, targetGroupID, h.getLocalizer(ctx, targetGroupID))

			// Send dual messages (private + group, no fallback)
			if err := h.router.SendPolicyChangeMessages(ctx, PolicyChangeRequest{
				UserID:      userID,
				GroupID:     targetGroupID,
				PrivateText: response,
				GroupText:   groupNotification,
			}); err != nil {
				h.logger.Errorw(
					"Failed to send policy change messages",
					"command", cmd.Name,
					"user_id", userID,
					"group_id", targetGroupID,
					"error", err,
				)
			}
		} else {
			// All command responses go to private chat only (silent failure if unavailable)
			if err := h.router.SendCommandResponse(ctx, userID, response); err != nil {
				h.logger.Errorw(
					"Failed to send command response",
					"command", cmd.Name,
					"chat_id", chatID,
					"user_id", userID,
					"error", err,
				)
				metrics.RecordError("send_command_response_failed", "handleCommand")
			}
		}
	}

	return nil
}

// sendMultiWindowWarnings sends warning notifications at 95% threshold
// Sends localized warnings to user's private chat using their language preference
// Includes group name to distinguish warnings from multiple groups
func (h *Handler) sendMultiWindowWarnings(ctx context.Context, chatID, userID int64, groupName string, warnings []*ratelimit.WindowEvaluationResult) {
	// Get user's language preference (fallback to English)
	userLang, err := h.store.GetUserLanguage(ctx, userID)
	if err != nil || userLang == nil || *userLang == "" {
		defaultLang := "en"
		userLang = &defaultLang
	}

	// Get localizer for user's language
	localizer := i18nPkg.GetLocalizer(*userLang)

	for _, warning := range warnings {
		// Render localized warning message with group context
		warningMsg := i18nPkg.Localize(localizer, "rate_limit_warning_95", map[string]interface{}{
			"group":   groupName,
			"window":  strings.ToUpper(warning.SlotID),
			"current": warning.CharCount,
			"limit":   warning.CharLimit,
		})

		// Send to user's private chat (silent failure if unavailable)
		if err := h.router.SendUserNotification(ctx, userID, warningMsg); err != nil {
			// Only log real errors, not silent failures
			h.logger.Warnw(
				"Failed to send rate limit warning to private chat",
				"user_id", userID,
				"chat_id", chatID,
				"group", groupName,
				"window", warning.SlotID,
				"percentage", warning.Percentage,
				"error", err,
			)
		}
	}
}

// enforceMultiWindowRateLimit enforces restriction when any window is violated
// Sends violation notification to user's private chat (NOT group)
func (h *Handler) enforceMultiWindowRateLimit(
	ctx context.Context,
	userID, chatID, messageID int64,
	violatedWindows []string,
	allWindows []*ratelimit.WindowEvaluationResult,
	groupName string,
) error {
	// Delete the violating message
	if err := h.client.DeleteMessage(chatID, int(messageID)); err != nil {
		h.logger.Warnw(
			"Failed to delete message after multi-window violation",
			"chat_id", chatID,
			"message_id", messageID,
			"error", err,
		)
	}

	// Build violation notification with group context
	notificationMsg := h.buildMultiWindowViolationMessage(ctx, userID, groupName, violatedWindows, allWindows)

	// Send to user's private chat (silent failure if unavailable)
	if err := h.router.SendUserNotification(ctx, userID, notificationMsg); err != nil {
		h.logger.Warnw(
			"Failed to send violation notification to private chat",
			"user_id", userID,
			"chat_id", chatID,
			"group", groupName,
			"violated_windows", strings.Join(violatedWindows, ","),
			"error", err,
		)
	}

	metrics.RecordRateLimitEnforcement("multi_window_violated", strings.Join(violatedWindows, ","))

	return nil
}

// buildMultiWindowViolationMessage builds notification message for violated windows
// Includes group name for context when user is in multiple groups
func (h *Handler) buildMultiWindowViolationMessage(
	ctx context.Context,
	userID int64,
	groupName string,
	violatedWindows []string,
	allWindows []*ratelimit.WindowEvaluationResult,
) string {
	// Get user's language preference
	userLang, err := h.store.GetUserLanguage(ctx, userID)
	if err != nil || userLang == nil || *userLang == "" {
		defaultLang := "en"
		userLang = &defaultLang
	}
	localizer := i18nPkg.GetLocalizer(*userLang)

	// Build map for easy lookup
	windowMap := make(map[string]*ratelimit.WindowEvaluationResult)
	for _, w := range allWindows {
		windowMap[w.SlotID] = w
	}

	// Build window details list
	var windowDetails []string
	for _, slotID := range violatedWindows {
		if window, ok := windowMap[slotID]; ok {
			windowDetails = append(windowDetails, i18nPkg.Localize(localizer, "rate_limit_window_detail", map[string]interface{}{
				"window":     strings.ToUpper(slotID),
				"charCount":  window.CharCount,
				"charLimit":  window.CharLimit,
				"percentage": window.Percentage,
			}))
		}
	}

	// Choose message based on violation count
	if len(violatedWindows) == 1 {
		return i18nPkg.Localize(localizer, "rate_limit_exceeded_single", map[string]interface{}{
			"group":   groupName,
			"details": strings.Join(windowDetails, "\n"),
		})
	}

	return i18nPkg.Localize(localizer, "rate_limit_exceeded_multiple", map[string]interface{}{
		"group":   groupName,
		"count":   len(violatedWindows),
		"details": strings.Join(windowDetails, "\n"),
	})
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

// handleMyChatMember processes my_chat_member updates (bot status changes).
// Detects when the bot is added to a group and triggers initial member sync.
func (h *Handler) handleMyChatMember(ctx context.Context, update *tgbotapi.ChatMemberUpdated) error {
	chatID := update.Chat.ID
	oldStatus := update.OldChatMember.Status
	newStatus := update.NewChatMember.Status

	h.logger.Infow(
		"Bot status changed",
		"chat_id", chatID,
		"chat_title", update.Chat.Title,
		"old_status", oldStatus,
		"new_status", newStatus,
	)

	// Detect transition: bot was added to a group
	if (oldStatus == "left" || oldStatus == "kicked") &&
		(newStatus == "member" || newStatus == "administrator") {
		h.logger.Infow(
			"Bot added to group, triggering initial sync",
			"chat_id", chatID,
			"chat_title", update.Chat.Title,
		)

		// Ensure group record exists (Feature 011: proactive group record creation)
		if err := h.store.EnsureGroupWithUsername(ctx, chatID, update.Chat.UserName); err != nil {
			h.logger.Errorw(
				"Failed to ensure group exists",
				"chat_id", chatID,
				"error", err,
			)
			return err
		}

		// Trigger initial sync via coordinator
		if h.syncCoordinator != nil {
			if err := h.syncCoordinator.InitialGroupSync(ctx, chatID); err != nil {
				h.logger.Errorw(
					"Failed to perform initial group sync",
					"chat_id", chatID,
					"error", err,
				)
				// Don't return error - log and continue (non-critical failure)
			}
		} else {
			h.logger.Warnw(
				"Sync coordinator not available, skipping initial sync",
				"chat_id", chatID,
			)
		}
	}

	// Detect transition: bot was removed from a group
	if (oldStatus == "member" || oldStatus == "administrator") &&
		(newStatus == "left" || newStatus == "kicked" || newStatus == "banned") {
		h.logger.Infow(
			"Bot removed from group",
			"chat_id", chatID,
			"chat_title", update.Chat.Title,
			"reason", newStatus,
		)
		// Future: Mark group as inactive in sync_metadata
	}

	return nil
}

// handleChatMember processes chat_member updates (member status changes).
// Implements real-time member tracking for Feature 011 (Phase 4 - User Story 2).
func (h *Handler) handleChatMember(ctx context.Context, update *tgbotapi.ChatMemberUpdated) error {
	chatID := update.Chat.ID
	userID := int64(update.NewChatMember.User.ID)
	oldStatus := update.OldChatMember.Status
	newStatus := update.NewChatMember.Status

	h.logger.Debugw(
		"Received chat_member update",
		"chat_id", chatID,
		"user_id", userID,
		"username", update.NewChatMember.User.UserName,
		"old_status", oldStatus,
		"new_status", newStatus,
	)

	// Ensure user record exists (username harvesting)
	if err := h.store.EnsureUser(ctx, userID, update.NewChatMember.User.UserName); err != nil {
		h.logger.Warnw(
			"Failed to ensure user record",
			"user_id", userID,
			"error", err,
		)
	}

	// Detect member joined (left/kicked/banned → member/administrator)
	if (oldStatus == "left" || oldStatus == "kicked" || oldStatus == "banned") &&
		(newStatus == "member" || newStatus == "administrator") {

		h.logger.Infow(
			"Member joined group",
			"chat_id", chatID,
			"user_id", userID,
			"username", update.NewChatMember.User.UserName,
			"is_admin", newStatus == "administrator",
		)

		if h.syncCoordinator != nil {
			if err := h.syncCoordinator.ProcessMemberJoin(ctx, chatID, update.NewChatMember); err != nil {
				h.logger.Warnw(
					"Failed to process member join",
					"chat_id", chatID,
					"user_id", userID,
					"error", err,
				)
			}
		}
	}

	// Detect member left (member/administrator → left)
	if (oldStatus == "member" || oldStatus == "administrator") &&
		newStatus == "left" {

		h.logger.Infow(
			"Member left group",
			"chat_id", chatID,
			"user_id", userID,
			"username", update.NewChatMember.User.UserName,
		)

		if h.syncCoordinator != nil {
			if err := h.syncCoordinator.ProcessMemberLeave(ctx, chatID, userID); err != nil {
				h.logger.Warnw(
					"Failed to process member leave",
					"chat_id", chatID,
					"user_id", userID,
					"error", err,
				)
			}
		}
	}

	// Detect member kicked/banned (member/administrator → kicked/banned)
	if (oldStatus == "member" || oldStatus == "administrator") &&
		(newStatus == "kicked" || newStatus == "banned") {

		h.logger.Infow(
			"Member kicked/banned from group",
			"chat_id", chatID,
			"user_id", userID,
			"username", update.NewChatMember.User.UserName,
			"reason", newStatus,
		)

		if h.syncCoordinator != nil {
			if err := h.syncCoordinator.ProcessMemberKicked(ctx, chatID, userID, newStatus); err != nil {
				h.logger.Warnw(
					"Failed to process member kick/ban",
					"chat_id", chatID,
					"user_id", userID,
					"error", err,
				)
			}
		}
	}

	// Detect promotion/demotion (member ↔ administrator)
	if (oldStatus == "member" && newStatus == "administrator") ||
		(oldStatus == "administrator" && newStatus == "member") {

		h.logger.Infow(
			"Member role changed",
			"chat_id", chatID,
			"user_id", userID,
			"username", update.NewChatMember.User.UserName,
			"old_role", oldStatus,
			"new_role", newStatus,
		)

		if h.syncCoordinator != nil {
			if err := h.syncCoordinator.ProcessMemberRoleChange(ctx, chatID, update.NewChatMember); err != nil {
				h.logger.Warnw(
					"Failed to process member role change",
					"chat_id", chatID,
					"user_id", userID,
					"error", err,
				)
			}
		}
	}

	return nil
}
