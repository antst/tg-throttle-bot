// Package i18n provides internationalization support for the Telegram bot.
// It uses go-i18n v2 for message localization with embedded translation files.
//
// Supported languages: English (en), Russian (ru), Dutch (nl)
//
// Usage:
//
//	// Initialize at startup
//	if err := i18n.Init(); err != nil {
//	    log.Fatal(err)
//	}
//
//	// Get localizer for a specific language
//	loc := i18n.GetLocalizer("ru")
//
//	// Translate a simple message
//	msg := i18n.LocalizeSimple(loc, "permission_denied")
//
//	// Translate with template data
//	msg := i18n.Localize(loc, "window_set_success", map[string]interface{}{
//	    "window": "A",
//	    "limit": 1000,
//	    "duration": "30 minutes",
//	})
package i18n

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

var (
	// Bundle holds all translations
	Bundle *i18n.Bundle

	// Localizers for each supported language (cached)
	localizers = make(map[string]*i18n.Localizer)

	// Supported languages
	SupportedLanguages = []string{"en", "ru", "nl"}
)

// Init initializes the i18n bundle and loads all translation files
func Init() error {
	// Create bundle with default language (English)
	Bundle = i18n.NewBundle(language.English)
	Bundle.RegisterUnmarshalFunc("json", json.Unmarshal)

	// Load translation files from embedded FS
	for _, lang := range SupportedLanguages {
		filename := fmt.Sprintf("translations/%s.json", lang)
		if _, err := Bundle.LoadMessageFileFS(TranslationsFS, filename); err != nil {
			return fmt.Errorf("failed to load translation file %s: %w", filename, err)
		}
	}

	// Pre-create localizers for all supported languages
	for _, lang := range SupportedLanguages {
		localizers[lang] = i18n.NewLocalizer(Bundle, lang)
	}

	return nil
}

// GetLocalizer returns a localizer for the specified language
// Falls back to English if language is not supported
func GetLocalizer(lang string) *i18n.Localizer {
	// Normalize language code
	lang = NormalizeLanguage(lang)

	// Get cached localizer
	if loc, ok := localizers[lang]; ok {
		return loc
	}

	// Fallback to English
	return localizers["en"]
}

// NormalizeLanguage converts language names to ISO 639-1 language codes.
// It handles both code inputs (en, ru, nl) and full name inputs (english, russian, dutch).
// Cyrillic input is also supported (русский -> ru).
//
// If the input is not recognized, it defaults to "en" (English).
//
// Examples:
//
//	NormalizeLanguage("english") // returns "en"
//	NormalizeLanguage("EN")      // returns "en"
//	NormalizeLanguage("русский") // returns "ru"
//	NormalizeLanguage("unknown") // returns "en" (fallback)
func NormalizeLanguage(input string) string {
	lower := strings.ToLower(strings.TrimSpace(input))

	switch lower {
	case "english", "en":
		return "en"
	case "russian", "ru", "русский":
		return "ru"
	case "dutch", "nl", "nederlands":
		return "nl"
	default:
		return "en" // Fallback to English
	}
}

// ValidateLanguage checks if a language code or name is supported.
// It returns the normalized language code and true if the language is supported,
// or an empty string and false if the language is not supported.
//
// Unlike NormalizeLanguage, this function does NOT provide a fallback to English
// for unsupported languages.
//
// Examples:
//
//	ValidateLanguage("english")  // returns ("en", true)
//	ValidateLanguage("ru")       // returns ("ru", true)
//	ValidateLanguage("spanish")  // returns ("", false)
func ValidateLanguage(input string) (string, bool) {
	lower := strings.ToLower(strings.TrimSpace(input))

	// Check explicit valid inputs first (before fallback normalization)
	switch lower {
	case "english", "en":
		return "en", true
	case "russian", "ru", "русский":
		return "ru", true
	case "dutch", "nl", "nederlands":
		return "nl", true
	default:
		return "", false // Reject unknown languages
	}
}

// Localize translates a message key with template data using the provided localizer.
// Template data is interpolated into the message using go-i18n v2 syntax ({{.variable}}).
//
// If the message key is not found or translation fails, it returns the message ID
// as a fallback to help with debugging.
//
// Parameters:
//   - localizer: The i18n.Localizer for the target language
//   - messageID: The message key from translation files (e.g., "window_set_success")
//   - templateData: Map of variable names to values for template interpolation
//
// Examples:
//
//	Localize(loc, "window_set_success", map[string]interface{}{
//	    "window": "A",
//	    "limit": 1000,
//	    "duration": "30 minutes",
//	})
//	// Returns: "✅ Window A configured: 1000 characters per 30 minutes"
func Localize(localizer *i18n.Localizer, messageID string, templateData map[string]interface{}) string {
	msg, err := localizer.Localize(&i18n.LocalizeConfig{
		MessageID:    messageID,
		TemplateData: templateData,
	})

	if err != nil {
		// Fallback: return message ID if translation fails
		return messageID
	}

	return msg
}

// LocalizeSimple translates a message key without template data.
// This is a convenience wrapper around Localize for messages that don't require
// variable interpolation.
//
// If the message key is not found, it returns the message ID as a fallback.
//
// Examples:
//
//	LocalizeSimple(loc, "permission_denied")
//	// Returns: "Only administrators can change language settings"
func LocalizeSimple(localizer *i18n.Localizer, messageID string) string {
	return Localize(localizer, messageID, nil)
}

// GetLanguageName returns the localized name of a language code.
// The returned name is in the target language (not necessarily in the language being named).
//
// For example, if using a Russian localizer:
//   - GetLanguageName(russianLocalizer, "en") returns "Английский" (English in Russian)
//   - GetLanguageName(russianLocalizer, "ru") returns "Русский" (Russian in Russian)
//
// Supported language codes: "en", "ru", "nl"
// Unsupported codes return the code itself.
func GetLanguageName(localizer *i18n.Localizer, langCode string) string {
	switch langCode {
	case "en":
		return LocalizeSimple(localizer, "english_name")
	case "ru":
		return LocalizeSimple(localizer, "russian_name")
	case "nl":
		return LocalizeSimple(localizer, "dutch_name")
	default:
		return langCode
	}
}

// FormatDuration formats a duration value with proper pluralization.
// It automatically selects singular or plural forms based on the value.
//
// Supported units: "minute", "hour", "day"
//
// The function looks up translation keys:
//   - For value == 1: uses the unit directly (e.g., "minute")
//   - For value != 1: appends "s" to unit (e.g., "minutes")
//
// Examples:
//
//	FormatDuration(loc, 1, "minute")  // "1 minute" (English) or "1 минуту" (Russian)
//	FormatDuration(loc, 30, "minute") // "30 minutes" (English) or "30 минут" (Russian)
//	FormatDuration(loc, 2, "hour")    // "2 hours" (English) or "2 часов" (Russian)
func FormatDuration(localizer *i18n.Localizer, value int, unit string) string {
	var unitKey string

	// Select singular or plural form
	if value == 1 {
		unitKey = unit // singular: "minute", "hour", "day"
	} else {
		unitKey = unit + "s" // plural: "minutes", "hours", "days"
	}

	translatedUnit := LocalizeSimple(localizer, unitKey)
	return fmt.Sprintf("%d %s", value, translatedUnit)
}
