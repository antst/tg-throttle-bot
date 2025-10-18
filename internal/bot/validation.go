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
