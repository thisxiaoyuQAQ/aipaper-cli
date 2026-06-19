package references

import "strings"

const CandidateSummaryMaxRunes = 160

// CandidateSummary formats an abstract as a single-line candidate summary.
func CandidateSummary(abstract string) string {
	text := strings.Join(strings.Fields(strings.TrimSpace(abstract)), " ")
	if text == "" {
		return ""
	}
	runes := []rune(text)
	if len(runes) <= CandidateSummaryMaxRunes {
		return text
	}
	return string(runes[:CandidateSummaryMaxRunes]) + "..."
}
