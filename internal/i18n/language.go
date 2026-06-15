package i18n

import "strings"

// Language is a normalized UI language code.
type Language string

const (
	ZhCN Language = "zh-CN"
	En   Language = "en"
)

// NormalizeLanguage converts common language aliases to the supported UI
// language codes. Unknown and empty values fall back to Chinese.
func NormalizeLanguage(value string) Language {
	switch normalizeAlias(value) {
	case "en", "enus", "eng", "english":
		return En
	case "zhcn", "zh", "cn", "chs", "zhhans", "chinese", "中文", "简体中文":
		return ZhCN
	default:
		return ZhCN
	}
}

// IsSupported reports whether value names a supported language or alias.
func IsSupported(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return true
	}
	switch normalizeAlias(trimmed) {
	case "en", "enus", "eng", "english", "zhcn", "zh", "cn", "chs", "zhhans", "chinese", "中文", "简体中文":
		return true
	default:
		return false
	}
}

func normalizeAlias(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "-", "")
	value = strings.ReplaceAll(value, "_", "")
	value = strings.ReplaceAll(value, " ", "")
	return value
}
