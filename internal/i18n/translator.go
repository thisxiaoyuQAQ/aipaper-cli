package i18n

import "fmt"

// Key identifies a translatable UI message.
type Key string

// T is a lightweight translator for the TUI/CLI UI.
type T struct {
	lang Language
}

// New creates a translator. Empty and unsupported languages default to Chinese.
func New(lang string) T {
	return T{lang: NormalizeLanguage(lang)}
}

// IsZero reports whether the translator has not been initialized.
func (t T) IsZero() bool {
	return t.lang == ""
}

// Lang returns the normalized language, defaulting to Chinese for zero values.
func (t T) Lang() Language {
	if t.lang == "" {
		return ZhCN
	}
	return t.lang
}

// Text returns a localized string with Chinese fallback and key fallback.
func (t T) Text(key Key) string {
	lang := t.Lang()
	if value := catalogs[lang][key]; value != "" {
		return value
	}
	if value := catalogs[ZhCN][key]; value != "" {
		return value
	}
	return string(key)
}

// Format formats a localized string with fmt.Sprintf.
func (t T) Format(key Key, args ...any) string {
	return fmt.Sprintf(t.Text(key), args...)
}
