package ratelimit

import (
	"context"
	"fmt"
)

// WindowValidator validates window configurations to ensure logical ordering
type WindowValidator struct {
	storage MultiWindowStorage
}

// NewWindowValidator creates a new window validator
func NewWindowValidator(storage MultiWindowStorage) *WindowValidator {
	return &WindowValidator{
		storage: storage,
	}
}

// ValidateWindowConfiguration validates that window configurations follow logical ordering rules
// FR-024: If window types are mixed (minute, hour, day), enforce A ≤ B ≤ C ordering
// If all window types are the same, any limit configuration is allowed
func (v *WindowValidator) ValidateWindowConfiguration(ctx context.Context, chatID int64) error {
	windows, err := v.storage.GetAllWindows(ctx, chatID)
	if err != nil {
		return fmt.Errorf("get all windows: %w", err)
	}

	// Filter enabled windows
	enabledWindows := []*WindowSlot{}
	for _, w := range windows {
		if w.Enabled {
			enabledWindows = append(enabledWindows, w)
		}
	}

	if len(enabledWindows) < 2 {
		// Nothing to validate with less than 2 windows
		return nil
	}

	// Sort by slot ID to ensure A, B, C order
	sortedWindows := make([]*WindowSlot, 3)
	for _, w := range enabledWindows {
		switch w.SlotID {
		case "a":
			sortedWindows[0] = w
		case "b":
			sortedWindows[1] = w
		case "c":
			sortedWindows[2] = w
		}
	}

	// Check if all window durations are the same
	var firstDuration int
	allSame := true
	for _, w := range sortedWindows {
		if w != nil {
			if firstDuration == 0 {
				firstDuration = w.WindowDuration
			} else if w.WindowDuration != firstDuration {
				allSame = false
				break
			}
		}
	}

	// If all same duration, any limits allowed
	if allSame {
		return nil
	}

	// Mixed durations: enforce duration ordering A < B < C
	for i := 0; i < len(sortedWindows)-1; i++ {
		if sortedWindows[i] == nil {
			continue
		}

		for j := i + 1; j < len(sortedWindows); j++ {
			if sortedWindows[j] == nil {
				continue
			}

			if sortedWindows[i].WindowDuration >= sortedWindows[j].WindowDuration {
				durationStrI := formatDuration(sortedWindows[i].DurationValue, sortedWindows[i].DurationUnit)
				durationStrJ := formatDuration(sortedWindows[j].DurationValue, sortedWindows[j].DurationUnit)
				return fmt.Errorf(
					"invalid window configuration: Window %s duration (%d seconds, %s) must be less than Window %s duration (%d seconds, %s)",
					sortedWindows[i].SlotID, sortedWindows[i].WindowDuration, durationStrI,
					sortedWindows[j].SlotID, sortedWindows[j].WindowDuration, durationStrJ,
				)
			}
		}
	}

	return nil
}

// formatDuration is a helper to format duration for error messages
func formatDuration(value int, unit string) string {
	if value == 1 {
		return unit
	}
	return fmt.Sprintf("%d %ss", value, unit)
}

// ValidateNewWindowSlot validates a new window configuration before applying
func (v *WindowValidator) ValidateNewWindowSlot(ctx context.Context, chatID int64, slotID string, charLimit int, durationUnit string, windowDuration int) error {
	// Validate slot ID
	if slotID != "a" && slotID != "b" && slotID != "c" {
		return fmt.Errorf("invalid slot ID: %s (must be 'a', 'b', or 'c')", slotID)
	}

	// Validate char limit
	if charLimit <= 0 || charLimit > 10000000 {
		return fmt.Errorf("invalid char limit: %d (must be between 1 and 10,000,000)", charLimit)
	}

	// Validate duration unit
	if durationUnit != "minute" && durationUnit != "hour" && durationUnit != "day" {
		return fmt.Errorf("invalid duration unit: %s (must be 'minute', 'hour', or 'day')", durationUnit)
	}

	// Validate window duration is reasonable
	if windowDuration <= 0 || windowDuration > 31536000 { // Max 1 year
		return fmt.Errorf("invalid window duration: %d seconds (must be between 1 and 31536000)", windowDuration)
	}

	return nil
}

// ValidateWindowOrdering validates that enabling/disabling a window maintains logical ordering
func (v *WindowValidator) ValidateWindowOrdering(ctx context.Context, chatID int64, slotID string, enabled bool) error {
	// Get current window configuration
	windows, err := v.storage.GetAllWindows(ctx, chatID)
	if err != nil {
		return fmt.Errorf("get all windows: %w", err)
	}

	// Simulate the change
	for _, w := range windows {
		if w.SlotID == slotID {
			w.Enabled = enabled
			break
		}
	}

	// Validate the new configuration
	return v.ValidateWindowConfiguration(ctx, chatID)
}

// CheckLastWindowDisable checks if disabling this window would disable all windows
// Returns warning message if this is the last enabled window
func (v *WindowValidator) CheckLastWindowDisable(ctx context.Context, chatID int64, slotID string) (bool, error) {
	windows, err := v.storage.GetAllWindows(ctx, chatID)
	if err != nil {
		return false, fmt.Errorf("get all windows: %w", err)
	}

	enabledCount := 0
	for _, w := range windows {
		if w.Enabled && w.SlotID != slotID {
			enabledCount++
		}
	}

	return enabledCount == 0, nil
}
