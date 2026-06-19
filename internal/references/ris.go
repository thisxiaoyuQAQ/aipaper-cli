package references

import (
	"fmt"
	"strings"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
)

type RISEntry struct {
	Type   string              `json:"type"`
	Fields map[string][]string `json:"fields"`
}

func ParseRIS(data []byte) ([]RISEntry, error) {
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	var entries []RISEntry
	current := RISEntry{Fields: map[string][]string{}}
	inEntry := false
	var lastTag string
	for _, line := range lines {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		if len(line) >= 6 && line[2:6] == "  - " {
			tag := strings.ToUpper(strings.TrimSpace(line[:2]))
			value := strings.TrimSpace(line[6:])
			if tag == "TY" {
				if inEntry && len(current.Fields) > 0 {
					entries = append(entries, current)
				}
				current = RISEntry{Type: value, Fields: map[string][]string{}}
				inEntry = true
				lastTag = tag
				continue
			}
			if !inEntry {
				current = RISEntry{Fields: map[string][]string{}}
				inEntry = true
			}
			if tag == "ER" {
				entries = append(entries, current)
				current = RISEntry{Fields: map[string][]string{}}
				inEntry = false
				lastTag = ""
				continue
			}
			current.Fields[tag] = append(current.Fields[tag], value)
			lastTag = tag
			continue
		}
		if inEntry && lastTag != "" {
			values := current.Fields[lastTag]
			if len(values) > 0 {
				values[len(values)-1] = strings.TrimSpace(values[len(values)-1] + " " + strings.TrimSpace(line))
				current.Fields[lastTag] = values
			}
		}
	}
	if inEntry && (current.Type != "" || len(current.Fields) > 0) {
		entries = append(entries, current)
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("no RIS entries found")
	}
	return entries, nil
}

func CandidatesFromRIS(entries []RISEntry, startIndex int) []contracts.ReferenceCandidate {
	if startIndex <= 0 {
		startIndex = 1
	}
	candidates := make([]contracts.ReferenceCandidate, 0, len(entries))
	for i, entry := range entries {
		title := risFirst(entry.Fields, "TI", "T1", "CT", "BT")
		if title == "" {
			title = "RIS reference"
		}
		candidate := contracts.ReferenceCandidate{
			ID:           fmt.Sprintf("cand_%03d", startIndex+i),
			Title:        title,
			Authors:      risValues(entry.Fields, "AU", "A1"),
			Year:         parseYear(risFirst(entry.Fields, "PY", "Y1", "DA")),
			Source:       "ris",
			DOI:          risFirst(entry.Fields, "DO"),
			URL:          risFirst(entry.Fields, "UR", "L2", "LK"),
			Abstract:     risFirst(entry.Fields, "AB", "N2"),
			Venue:        risFirst(entry.Fields, "JO", "JF", "T2", "JA"),
			Status:       "pending",
			Reliability:  "user_export",
			Availability: availabilityFromURLAndDOI(risFirst(entry.Fields, "UR", "L2", "LK"), risFirst(entry.Fields, "DO")),
			AccessURL:    risFirst(entry.Fields, "UR", "L2", "LK"),
		}
		candidate.DedupeGroup = CandidateDedupeGroup(candidate)
		candidates = append(candidates, candidate)
	}
	return candidates
}

func risFirst(fields map[string][]string, tags ...string) string {
	for _, tag := range tags {
		for _, value := range fields[strings.ToUpper(tag)] {
			if strings.TrimSpace(value) != "" {
				return strings.TrimSpace(value)
			}
		}
	}
	return ""
}

func risValues(fields map[string][]string, tags ...string) []string {
	var out []string
	for _, tag := range tags {
		for _, value := range fields[strings.ToUpper(tag)] {
			if strings.TrimSpace(value) != "" {
				out = append(out, strings.TrimSpace(value))
			}
		}
	}
	return out
}

func availabilityFromURLAndDOI(rawURL, doi string) string {
	if strings.TrimSpace(rawURL) != "" {
		return "landing_page"
	}
	if strings.TrimSpace(doi) != "" {
		return "doi_landing"
	}
	return "unknown"
}
