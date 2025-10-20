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

// GroupIdentifier represents a parsed group reference (username or numeric ID)
type GroupIdentifier struct {
	GroupID  int64  // Numeric group ID (negative for supergroups)
	Username string // Group @username (without @ prefix)
	IsName   bool   // True if parsed from @username, false if numeric ID
}

// ParseGroupIdentifier parses a group reference from command arguments.
// Supports two formats:
//   - @username (e.g., @home, @workgroup)
//   - Numeric ID (e.g., -4211780967, -1001234567890)
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
		}, nil
	}

	// Try parsing as numeric Group ID
	groupID, err := strconv.ParseInt(arg, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid group identifier: must be @username or numeric ID")
	}

	// Telegram group IDs are negative
	if groupID >= 0 {
		return nil, fmt.Errorf("invalid group ID: must be negative (e.g., -4211780967)")
	}

	return &GroupIdentifier{
		GroupID: groupID,
		IsName:  false,
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
				"  /command -1234567890 [args...]")
		}

		// Parse the first argument as group identifier
		groupID, parseErr := ParseGroupIdentifier(args[0])
		if parseErr != nil {
			return nil, nil, fmt.Errorf("invalid group identifier: %w", parseErr)
		}

		// Return the identifier - caller must resolve username if needed
		return groupID, args[1:], nil
	}

	// Group chat: use current chat ID, no group parameter needed
	return &GroupIdentifier{
		GroupID: chatID,
		IsName:  false,
	}, args, nil
}

// ResolveGroupID resolves a GroupIdentifier to a numeric group ID.
// If identifier is already numeric, returns it directly.
// If identifier is a username, looks it up via the provided resolver function.
func ResolveGroupID(identifier *GroupIdentifier, resolveUsername func(string) (int64, error)) (int64, error) {
	if !identifier.IsName {
		// Already a numeric ID
		return identifier.GroupID, nil
	}

	// Need to resolve username to ID
	groupID, err := resolveUsername(identifier.Username)
	if err != nil {
		return 0, fmt.Errorf("failed to resolve @%s: %w", identifier.Username, err)
	}

	return groupID, nil
}
