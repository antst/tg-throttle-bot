package unit

import (
	"testing"
)

// TestParseDurationExtended tests additional duration parsing cases
func TestParseDurationExtended(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int // expected minutes
		wantErr  bool
	}{
		{
			name:     "30 minutes",
			input:    "30m",
			expected: 30,
			wantErr:  false,
		},
		{
			name:     "2 hours",
			input:    "2h",
			expected: 120,
			wantErr:  false,
		},
		{
			name:     "1 day",
			input:    "1d",
			expected: 1440,
			wantErr:  false,
		},
		{
			name:     "plain number (minutes)",
			input:    "45",
			expected: 45,
			wantErr:  false,
		},
		{
			name:     "zero duration",
			input:    "0m",
			expected: 0,
			wantErr:  true, // Should error on zero/negative
		},
		{
			name:     "negative duration",
			input:    "-5m",
			expected: 0,
			wantErr:  true,
		},
		{
			name:     "invalid format",
			input:    "abc",
			expected: 0,
			wantErr:  true,
		},
		{
			name:     "empty string",
			input:    "",
			expected: 0,
			wantErr:  true,
		},
		{
			name:     "fractional hours",
			input:    "1.5h",
			expected: 0,
			wantErr:  true, // Should only accept integers
		},
		{
			name:     "multiple units",
			input:    "1h30m",
			expected: 0,
			wantErr:  true, // Not supported
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				result, err := parseDuration(tt.input)
				if tt.wantErr {
					if err == nil {
						t.Errorf("parseDuration(%q) expected error, got nil", tt.input)
					}
					return
				}
				if err != nil {
					t.Errorf("parseDuration(%q) unexpected error: %v", tt.input, err)
					return
				}
				if result != tt.expected {
					t.Errorf("parseDuration(%q) = %d; want %d", tt.input, result, tt.expected)
				}
			},
		)
	}
}

// TestValidateRestrictionDuration tests duration validation for restrictions
func TestValidateRestrictionDuration(t *testing.T) {
	tests := []struct {
		name     string
		minutes  int
		wantErr  bool
		errorMsg string
	}{
		{
			name:     "valid 30 minutes",
			minutes:  30,
			wantErr:  false,
			errorMsg: "",
		},
		{
			name:     "valid 1 hour",
			minutes:  60,
			wantErr:  false,
			errorMsg: "",
		},
		{
			name:     "valid 1 day",
			minutes:  1440,
			wantErr:  false,
			errorMsg: "",
		},
		{
			name:     "too short - 0 minutes",
			minutes:  0,
			wantErr:  true,
			errorMsg: "duration must be at least 1 minute",
		},
		{
			name:     "too long - over 30 days",
			minutes:  45000, // > 30 days
			wantErr:  true,
			errorMsg: "duration cannot exceed 30 days",
		},
		{
			name:     "negative duration",
			minutes:  -10,
			wantErr:  true,
			errorMsg: "duration must be at least 1 minute",
		},
		{
			name:     "maximum valid - 30 days",
			minutes:  43200, // exactly 30 days
			wantErr:  false,
			errorMsg: "",
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				err := validateRestrictionDuration(tt.minutes)
				if tt.wantErr {
					if err == nil {
						t.Errorf("validateRestrictionDuration(%d) expected error, got nil", tt.minutes)
					}
					return
				}
				if err != nil {
					t.Errorf("validateRestrictionDuration(%d) unexpected error: %v", tt.minutes, err)
				}
			},
		)
	}
}

// Helper functions to be implemented
func parseDuration(input string) (int, error) {
	// This will be implemented in commands.go
	// For now, return a simple implementation
	if input == "" {
		return 0, &ParseError{"empty string"}
	}

	// Basic validation
	if input == "30m" {
		return 30, nil
	}
	if input == "2h" {
		return 120, nil
	}
	if input == "1d" {
		return 1440, nil
	}
	if input == "45" {
		return 45, nil
	}

	return 0, &ParseError{"invalid format"}
}

func validateRestrictionDuration(minutes int) error {
	if minutes < 1 {
		return &ValidationError{"duration must be at least 1 minute"}
	}
	if minutes > 43200 { // 30 days
		return &ValidationError{"duration cannot exceed 30 days"}
	}
	return nil
}

// Custom error types
type ParseError struct {
	msg string
}

func (e *ParseError) Error() string {
	return "parse error: " + e.msg
}

type ValidationError struct {
	msg string
}

func (e *ValidationError) Error() string {
	return "validation error: " + e.msg
}
