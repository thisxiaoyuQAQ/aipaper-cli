package search

import "testing"

func TestDetectLanguage(t *testing.T) {
	tests := []struct {
		name     string
		topic    string
		expected string
	}{
		{
			name:     "pure Chinese",
			topic:    "明朝君主哪个更厉害",
			expected: "zh-CN",
		},
		{
			name:     "pure English",
			topic:    "Retrieval augmented generation",
			expected: "en",
		},
		{
			name:     "Chinese majority mixed",
			topic:    "深度学习在医学影像中的应用",
			expected: "zh-CN",
		},
		{
			name:     "English majority mixed",
			topic:    "RAG with 中文 support",
			expected: "en",
		},
		{
			name:     "empty string",
			topic:    "",
			expected: "en",
		},
		{
			name:     "exactly 30% Chinese boundary",
			topic:    "abc明def清",
			expected: "en", // 2/8 = 25% < 30%
		},
		{
			name:     "just over 30% Chinese",
			topic:    "ab明清唐",
			expected: "zh-CN", // 3/5 = 60% > 30%
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectLanguage(tt.topic)
			if result != tt.expected {
				t.Errorf("DetectLanguage(%q) = %q, want %q", tt.topic, result, tt.expected)
			}
		})
	}
}

func TestIsChineseChar(t *testing.T) {
	tests := []struct {
		name     string
		char     rune
		expected bool
	}{
		{"Chinese character", '明', true},
		{"Chinese character", '朝', true},
		{"English letter", 'a', false},
		{"Digit", '1', false},
		{"Punctuation", '。', false},
		{"Space", ' ', false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isChineseChar(tt.char)
			if result != tt.expected {
				t.Errorf("isChineseChar('%c') = %v, want %v", tt.char, result, tt.expected)
			}
		})
	}
}
