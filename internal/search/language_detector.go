package search

import (
	"unicode"
)

// DetectLanguage detects the primary language of the given topic text.
// Returns "zh-CN" for Chinese, "en" for English.
// Detection is based on the ratio of Chinese characters in the text.
func DetectLanguage(topic string) string {
	if topic == "" {
		return "en"
	}

	runes := []rune(topic)
	if len(runes) == 0 {
		return "en"
	}

	chineseCount := 0
	for _, r := range runes {
		if isChineseChar(r) {
			chineseCount++
		}
	}

	// If more than 30% of characters are Chinese, treat as Chinese text
	ratio := float64(chineseCount) / float64(len(runes))
	if ratio > 0.3 {
		return "zh-CN"
	}

	return "en"
}

// isChineseChar checks if a rune is a Chinese character.
// Covers CJK Unified Ideographs ranges.
func isChineseChar(r rune) bool {
	return unicode.Is(unicode.Han, r)
}
