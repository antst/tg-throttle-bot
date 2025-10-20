package i18n

import "embed"

// TranslationsFS embeds all translation files at compile time
//
//go:embed translations/*.json
var TranslationsFS embed.FS
