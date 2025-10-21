// Package bot provides input validation for bot commands.
package bot

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	// MaxCharLimit is the maximum number of characters allowed per window
	MaxCharLimit = 100000 // Maximum characters per window
	// MinCharLimit is the minimum number of characters allowed per window
	MinCharLimit = 100 // Minimum characters per window

	// Window type abbreviation
	minAbbrev = "min"
)

// ValidateCharLimit validates character limit with bounds checking
func ValidateCharLimit(charLimit int) error {
	if charLimit < MinCharLimit {
		return fmt.Errorf("character limit must be at least %d", MinCharLimit)
	}
	if charLimit > MaxCharLimit {
		return fmt.Errorf("character limit cannot exceed %d", MaxCharLimit)
	}
	return nil
}

// ValidateWindowType validates and normalizes window type
func ValidateWindowType(windowType string) (string, error) {
	windowType = strings.ToLower(strings.TrimSpace(windowType))

	switch windowType {
	case WindowTypeMinute, minAbbrev:
		return WindowTypeMinute, nil
	case "hour", "h":
		return WindowTypeHour, nil
	case "day", "d":
		return WindowTypeDay, nil
	default:
		return "", fmt.Errorf("invalid window type '%s': must be 'minute', 'hour', or 'day'", windowType)
	}
}

// ValidateCommandArgs validates that minimum required arguments are provided
func ValidateCommandArgs(args []string, minArgs int, commandName string) error {
	if len(args) < minArgs {
		return fmt.Errorf(
			"insufficient arguments for /%s command (expected at least %d, got %d)",
			commandName, minArgs, len(args),
		)
	}
	return nil
}

// ValidateSetLimitInput performs comprehensive validation for /setlimit command
func ValidateSetLimitInput(charLimitStr string, windowType string) (int, string, error) {
	// Validate character limit string
	charLimitStr = strings.TrimSpace(charLimitStr)
	if charLimitStr == "" {
		return 0, "", fmt.Errorf("character limit cannot be empty")
	}

	charLimit, err := strconv.Atoi(charLimitStr)
	if err != nil {
		return 0, "", fmt.Errorf("invalid character limit: must be a valid number")
	}

	if err := ValidateCharLimit(charLimit); err != nil {
		return 0, "", err
	}

	// Validate and normalize window type
	normalizedWindowType, err := ValidateWindowType(windowType)
	if err != nil {
		return 0, "", err
	}

	return charLimit, normalizedWindowType, nil
}

// GroupIdentifier represents a parsed group reference (username, numeric ID, or title)
type GroupIdentifier struct {
	GroupID  int64  // Numeric group ID (negative for supergroups)
	Username string // Group @username (without @ prefix)
	Title    string // Group title for title-based lookup (Feature 014)
	IsName   bool   // True if parsed from @username, false if numeric ID or title
	IsTitle  bool   // True if parsed as title (neither @ nor numeric) - Feature 014
}

// ParseGroupIdentifier parses a group reference from command arguments.
// Supports three formats (Feature 014):
//   - @username (e.g., @home, @workgroup)
//   - Numeric ID (e.g., -4211780967, -1001234567890)
//   - "Title" (e.g., "My Cool Group" - quoted or unquoted)
//
// Returns GroupIdentifier with parsed values, or error if invalid format.
func ParseGroupIdentifier(arg string) (*GroupIdentifier, error) {
	arg = strings.TrimSpace(arg)

	if arg == "" {
		return nil, fmt.Errorf("group identifier cannot be empty")
	}

	// Check for @username format
	if strings.HasPrefix(arg, "@") {
		username := strings.TrimPrefix(arg, "@")
		username = strings.TrimSpace(username)

		if username == "" {
			return nil, fmt.Errorf("username cannot be empty after @")
		}

		// Validate username format (alphanumeric + underscores, 4-32 chars per Telegram rules)
		if len(username) < 4 || len(username) > 32 {
			return nil, fmt.Errorf("invalid username length: must be 4-32 characters")
		}

		for _, char := range username {
			if !isValidUsernameChar(char) {
				return nil, fmt.Errorf("invalid username format: only letters, numbers, and underscores allowed")
			}
		}

		return &GroupIdentifier{
			Username: username,
			IsName:   true,
			IsTitle:  false,
		}, nil
	}

	// Try parsing as numeric Group ID
	groupID, err := strconv.ParseInt(arg, 10, 64)
	if err == nil {
		// Telegram group IDs are negative
		if groupID >= 0 {
			return nil, fmt.Errorf("invalid group ID: must be negative (e.g., -4211780967)")
		}

		return &GroupIdentifier{
			GroupID: groupID,
			IsName:  false,
			IsTitle: false,
		}, nil
	}

	// Feature 014: Title-based resolution (fallback when not @ or numeric)
	// Note: Quotes are already stripped by ParseCommand, but we trim again for safety
	title := strings.Trim(arg, `"'`)
	title = strings.TrimSpace(title)

	if title == "" {
		return nil, fmt.Errorf("group title cannot be empty")
	}

	return &GroupIdentifier{
		Title:   title,
		IsName:  false,
		IsTitle: true,
	}, nil
}

// isValidUsernameChar checks if a character is valid in a Telegram username
func isValidUsernameChar(char rune) bool {
	return (char >= 'a' && char <= 'z') ||
		(char >= 'A' && char <= 'Z') ||
		(char >= '0' && char <= '9') ||
		char == '_'
}

// ResolveTargetGroup determines the target group for a command based on context.
// Rules (Simplified Private-Chat-Only approach):
//   - If chatID > 0 (private chat): REQUIRE group identifier in args[0]
//   - If chatID < 0 (group chat): USE chatID, args unchanged
//
// Returns:
//   - groupIdentifier: parsed group reference (may need username resolution)
//   - remainingArgs: args after removing group identifier (if any)
//   - error: validation or parsing errors
func ResolveTargetGroup(chatID int64, args []string) (*GroupIdentifier, []string, error) {
	// Check if this is a private chat (positive chatID)
	isPrivateChat := chatID > 0

	if isPrivateChat {
		// Private chat: REQUIRE group identifier as first argument
		if len(args) == 0 {
			return nil, nil, fmt.Errorf("when using commands from private chat, please specify the target group:\n" +
				"  /command @groupname [args...]\n" +
				"  or\n" +
				"  /command -1234567890 [args...]\n" +
				"  or\n" +
				"  /command title [args...]\n" +
				"  or\n" +
				"  /command \"multi word title\" [args...]")
		}

		// Feature 014: Parse group identifier (quotes already stripped by command parser)
		// Rule: Take ONLY first argument as group identifier
		// - Starts with '-' → numeric chat ID
		// - Starts with '@' → username
		// - Otherwise → group title (single or multi-word if quoted in original command)
		groupIdentifierStr := args[0]
		remainingArgs := args[1:]

		// Parse the group identifier
		groupID, parseErr := ParseGroupIdentifier(groupIdentifierStr)
		if parseErr != nil {
			return nil, nil, fmt.Errorf("invalid group identifier: %w", parseErr)
		}

		// Return the identifier - caller must resolve username if needed
		return groupID, remainingArgs, nil
	}

	// Group chat: use current chat ID, no group parameter needed
	return &GroupIdentifier{
		GroupID: chatID,
		IsName:  false,
	}, args, nil
}

// GroupResolver provides methods to resolve group identifiers to chat IDs (Feature 014)
type GroupResolver interface {
	ResolveUsername(username string) (int64, error)
	ResolveTitle(title string) ([]int64, error) // May return multiple matches
	GetGroupInfo(chatID int64) (username, title *string, err error)
}

// ResolveGroupID resolves a GroupIdentifier to a numeric group ID.
// Feature 014: Supports chat_id, @username, and "title" with conflict detection.
// Returns error if title matches multiple groups (ambiguous).
func ResolveGroupID(identifier *GroupIdentifier, resolver GroupResolver) (int64, error) {
	// Case 1: Already a numeric ID
	if !identifier.IsName && !identifier.IsTitle {
		return identifier.GroupID, nil
	}

	// Case 2: Username-based resolution
	if identifier.IsName {
		groupID, err := resolver.ResolveUsername(identifier.Username)
		if err != nil {
			return 0, fmt.Errorf("failed to resolve @%s: %w", identifier.Username, err)
		}
		return groupID, nil
	}

	// Case 3: Title-based resolution (Feature 014)
	if identifier.IsTitle {
		chatIDs, err := resolver.ResolveTitle(identifier.Title)
		if err != nil {
			return 0, fmt.Errorf("failed to resolve group by title '%s': %w", identifier.Title, err)
		}

		if len(chatIDs) == 0 {
			return 0, fmt.Errorf("no group found with title '%s'", identifier.Title)
		}

		// Conflict detection: multiple groups with same title
		if len(chatIDs) > 1 {
			// Build error message listing all matches
			var conflictMsg strings.Builder
			conflictMsg.WriteString(fmt.Sprintf("❌ Multiple groups found with title '%s':\n\n", identifier.Title))

			for i, chatID := range chatIDs {
				username, title, err := resolver.GetGroupInfo(chatID)
				if err != nil {
					conflictMsg.WriteString(fmt.Sprintf("%d. ID: %d (error: %v)\n", i+1, chatID, err))
					continue
				}

				groupDisplay := ""
				if title != nil && *title != "" {
					groupDisplay = *title
					if username != nil && *username != "" {
						groupDisplay = fmt.Sprintf("%s (@%s)", groupDisplay, *username)
					}
				} else if username != nil && *username != "" {
					groupDisplay = fmt.Sprintf("@%s", *username)
				} else {
					groupDisplay = fmt.Sprintf("ID: %d", chatID)
				}

				conflictMsg.WriteString(fmt.Sprintf("%d. %s\n", i+1, groupDisplay))
			}

			conflictMsg.WriteString("\n💡 Please use @username or chat ID instead to specify which group you mean.")

			return 0, fmt.Errorf(conflictMsg.String())
		}

		return chatIDs[0], nil
	}

	return 0, fmt.Errorf("invalid group identifier state")
}
