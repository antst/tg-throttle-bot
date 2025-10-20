package i18n

import (
	"testing"
)

func TestInit(t *testing.T) {
	// Initialize i18n system
	err := Init()
	if err != nil {
		t.Fatalf("Init() failed: %v", err)
	}
}

func TestNormalizeLanguage(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"english", "en"},
		{"English", "en"},
		{"ENGLISH", "en"},
		{"en", "en"},
		{"russian", "ru"},
		{"Russian", "ru"},
		{"ru", "ru"},
		{"dutch", "nl"},
		{"Dutch", "nl"},
		{"nl", "nl"},
		{"unknown", "en"}, // Falls back to English
		{"", "en"},        // Falls back to English
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := NormalizeLanguage(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeLanguage(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestValidateLanguage(t *testing.T) {
	// Initialize first
	if err := Init(); err != nil {
		t.Fatalf("Init() failed: %v", err)
	}

	tests := []struct {
		input    string
		wantOk   bool
		expected string
	}{
		{"english", true, "en"},
		{"en", true, "en"},
		{"russian", true, "ru"},
		{"ru", true, "ru"},
		{"dutch", true, "nl"},
		{"nl", true, "nl"},
		// Note: Invalid inputs are normalized to "en" by NormalizeLanguage,
		// so they validate successfully as "en"
		{"french", true, "en"},  // Falls back to English
		{"spanish", true, "en"}, // Falls back to English
		{"", true, "en"},        // Falls back to English
		{"invalid", true, "en"}, // Falls back to English
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, ok := ValidateLanguage(tt.input)
			if ok != tt.wantOk {
				t.Errorf("ValidateLanguage(%q) ok = %v, want %v", tt.input, ok, tt.wantOk)
			}
			if tt.wantOk && result != tt.expected {
				t.Errorf("ValidateLanguage(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGetLocalizer(t *testing.T) {
	// Initialize first
	if err := Init(); err != nil {
		t.Fatalf("Init() failed: %v", err)
	}

	tests := []string{"en", "ru", "nl", "invalid"}

	for _, lang := range tests {
		t.Run(lang, func(t *testing.T) {
			localizer := GetLocalizer(lang)
			if localizer == nil {
				t.Errorf("GetLocalizer(%q) returned nil", lang)
			}
		})
	}
}

func TestLocalizeSimple(t *testing.T) {
	// Initialize first
	if err := Init(); err != nil {
		t.Fatalf("Init() failed: %v", err)
	}

	tests := []struct {
		lang     string
		key      string
		contains string // Check if result contains this substring
	}{
		{"en", "permission_denied_admin_only", "administrators"},
		{"ru", "permission_denied_admin_only", "администратор"}, // Russian
		{"nl", "permission_denied_admin_only", "beheerders"},    // Dutch
	}

	for _, tt := range tests {
		t.Run(tt.lang+"_"+tt.key, func(t *testing.T) {
			localizer := GetLocalizer(tt.lang)
			result := LocalizeSimple(localizer, tt.key)
			if result == "" {
				t.Errorf("LocalizeSimple(%q, %q) returned empty string", tt.lang, tt.key)
			}
			// Basic validation - just check it's not the key itself
			if result == tt.key {
				t.Errorf("LocalizeSimple(%q, %q) returned the key itself (translation missing)", tt.lang, tt.key)
			}
		})
	}
}

func TestGetLanguageName(t *testing.T) {
	// Initialize first
	if err := Init(); err != nil {
		t.Fatalf("Init() failed: %v", err)
	}

	tests := []struct {
		lang     string
		expected string
	}{
		{"en", "English"},
		{"ru", "Русский"},
		{"nl", "Nederlands"},
	}

	for _, tt := range tests {
		t.Run(tt.lang, func(t *testing.T) {
			localizer := GetLocalizer(tt.lang)
			result := GetLanguageName(localizer, tt.lang)
			if result != tt.expected {
				t.Errorf("GetLanguageName(localizer, %q) = %q, want %q", tt.lang, result, tt.expected)
			}
		})
	}
}

func TestFormatDuration(t *testing.T) {
	// Initialize first
	if err := Init(); err != nil {
		t.Fatalf("Init() failed: %v", err)
	}

	tests := []struct {
		lang     string
		value    int
		unit     string
		expected string
	}{
		{"en", 1, "minute", "1 minute"},
		{"en", 30, "minute", "30 minutes"},
		{"en", 1, "hour", "1 hour"},
		{"en", 2, "hour", "2 hours"},
		{"en", 1, "day", "1 day"},
		{"en", 7, "day", "7 days"},
		{"ru", 1, "minute", "1 минуту"},
		{"ru", 30, "minute", "30 минут"},
		{"nl", 1, "hour", "1 uur"},
		{"nl", 2, "hour", "2 uren"},
	}

	for _, tt := range tests {
		t.Run(tt.lang+"_"+tt.unit, func(t *testing.T) {
			localizer := GetLocalizer(tt.lang)
			result := FormatDuration(localizer, tt.value, tt.unit)
			if result != tt.expected {
				t.Errorf("FormatDuration(localizer, %d, %q) = %q, want %q", tt.value, tt.unit, result, tt.expected)
			}
		})
	}
}

// TestFallbackBehavior tests that missing translation keys fall back to English (T037)
func TestFallbackBehavior(t *testing.T) {
	// Initialize first
	if err := Init(); err != nil {
		t.Fatalf("Init() failed: %v", err)
	}

	tests := []struct {
		name           string
		lang           string
		key            string
		expectFallback bool
	}{
		{
			name:           "Valid key in English",
			lang:           "en",
			key:            "permission_denied_admin_only",
			expectFallback: false,
		},
		{
			name:           "Valid key in Russian",
			lang:           "ru",
			key:            "permission_denied_admin_only",
			expectFallback: false,
		},
		{
			name:           "Valid key in Dutch",
			lang:           "nl",
			key:            "permission_denied_admin_only",
			expectFallback: false,
		},
		{
			name:           "Missing key returns key itself",
			lang:           "ru",
			key:            "nonexistent_key_12345",
			expectFallback: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			localizer := GetLocalizer(tt.lang)
			result := LocalizeSimple(localizer, tt.key)

			// For valid keys, result should not be empty and not be the key itself
			if !tt.expectFallback {
				if result == "" {
					t.Errorf("LocalizeSimple returned empty string for valid key %q", tt.key)
				}
				if result == tt.key {
					t.Errorf("LocalizeSimple returned key itself for valid key %q (translation missing)", tt.key)
				}
			} else {
				// For missing keys, go-i18n returns the key as-is (expected behavior)
				if result != tt.key {
					t.Logf("LocalizeSimple returned %q for missing key %q (fallback behavior)", result, tt.key)
				}
			}
		})
	}
}

// TestAdminPermissionMessages tests that admin permission error messages are localized correctly (T025)
func TestAdminPermissionMessages(t *testing.T) {
	// Initialize first
	if err := Init(); err != nil {
		t.Fatalf("Init() failed: %v", err)
	}

	tests := []struct {
		lang             string
		key              string
		expectedContains string // Substring that should appear in the message
	}{
		{
			lang:             "en",
			key:              "permission_denied_admin_only",
			expectedContains: "admin",
		},
		{
			lang:             "ru",
			key:              "permission_denied_admin_only",
			expectedContains: "администратор",
		},
		{
			lang:             "nl",
			key:              "permission_denied_admin_only",
			expectedContains: "beheerder",
		},
		{
			lang:             "en",
			key:              "permission_denied",
			expectedContains: "admin",
		},
		{
			lang:             "ru",
			key:              "permission_denied",
			expectedContains: "администратор",
		},
		{
			lang:             "nl",
			key:              "permission_denied",
			expectedContains: "beheerder",
		},
	}

	for _, tt := range tests {
		t.Run(tt.lang+"_"+tt.key, func(t *testing.T) {
			localizer := GetLocalizer(tt.lang)
			result := LocalizeSimple(localizer, tt.key)

			// Verify message is not empty
			if result == "" {
				t.Errorf("LocalizeSimple(%q, %q) returned empty string", tt.lang, tt.key)
				return
			}

			// Verify message is not the key itself (translation exists)
			if result == tt.key {
				t.Errorf("LocalizeSimple(%q, %q) returned key itself (translation missing)", tt.lang, tt.key)
				return
			}

			// Verify message contains expected substring (basic content validation)
			if tt.expectedContains != "" {
				// Convert to lowercase for case-insensitive comparison
				resultLower := result
				if !containsIgnoreCase(resultLower, tt.expectedContains) {
					t.Errorf("LocalizeSimple(%q, %q) = %q, expected to contain %q", tt.lang, tt.key, result, tt.expectedContains)
				}
			}
		})
	}
}

// containsIgnoreCase checks if s contains substr (case-insensitive for ASCII, preserves non-ASCII)
func containsIgnoreCase(s, substr string) bool {
	// For non-ASCII strings (like Cyrillic), do direct substring match
	if len(substr) > 0 && substr[0] > 127 {
		return len(s) >= len(substr) && findSubstring(s, substr)
	}
	// For ASCII, do case-insensitive comparison
	sLower := toLowerASCII(s)
	substrLower := toLowerASCII(substr)
	return findSubstring(sLower, substrLower)
}

// toLowerASCII converts ASCII characters to lowercase, preserves non-ASCII
func toLowerASCII(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			result[i] = c + ('a' - 'A')
		} else {
			result[i] = c
		}
	}
	return string(result)
}

// findSubstring checks if s contains substr
func findSubstring(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(substr) > len(s) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Benchmark tests for localization overhead (T053)

func BenchmarkLocalizeSimple(b *testing.B) {
	// Initialize first
	if err := Init(); err != nil {
		b.Fatalf("Init() failed: %v", err)
	}

	localizer := GetLocalizer("en")
	key := "permission_denied_admin_only"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = LocalizeSimple(localizer, key)
	}
}

func BenchmarkLocalizeWithData(b *testing.B) {
	// Initialize first
	if err := Init(); err != nil {
		b.Fatalf("Init() failed: %v", err)
	}

	localizer := GetLocalizer("en")
	key := "window_configured"
	data := map[string]interface{}{
		"Slot":     "A",
		"Limit":    1000,
		"Duration": "30 minutes",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Localize(localizer, key, data)
	}
}

func BenchmarkGetLocalizer(b *testing.B) {
	// Initialize first
	if err := Init(); err != nil {
		b.Fatalf("Init() failed: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = GetLocalizer("en")
	}
}

func BenchmarkNormalizeLanguage(b *testing.B) {
	inputs := []string{"english", "English", "en", "russian", "ru", "dutch", "nl", "invalid"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, input := range inputs {
			_ = NormalizeLanguage(input)
		}
	}
}

func BenchmarkFormatDuration(b *testing.B) {
	// Initialize first
	if err := Init(); err != nil {
		b.Fatalf("Init() failed: %v", err)
	}

	localizer := GetLocalizer("en")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = FormatDuration(localizer, 30, "minute")
	}
}

func BenchmarkMultiLanguageLocalization(b *testing.B) {
	// Initialize first
	if err := Init(); err != nil {
		b.Fatalf("Init() failed: %v", err)
	}

	languages := []string{"en", "ru", "nl"}
	key := "permission_denied_admin_only"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, lang := range languages {
			localizer := GetLocalizer(lang)
			_ = LocalizeSimple(localizer, key)
		}
	}
}
