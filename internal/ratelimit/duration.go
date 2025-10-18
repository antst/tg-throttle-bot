package ratelimit

import "fmt"

// CalculateWindowDuration converts duration value + unit to seconds
// Used by commands to calculate window_duration field
// Examples: (30, "minute") → 1800, (2, "hour") → 7200, (7, "day") → 604800
func CalculateWindowDuration(value int, unit string) (int, error) {
	switch unit {
	case "minute":
		return value * 60, nil
	case "hour":
		return value * 3600, nil
	case "day":
		return value * 86400, nil
	default:
		return 0, fmt.Errorf("invalid duration unit: %s (must be minute, hour, or day)", unit)
	}
}
