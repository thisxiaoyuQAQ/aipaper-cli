package references

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
)

type BibTeXEntry struct {
	Type   string            `json:"type"`
	Key    string            `json:"key"`
	Fields map[string]string `json:"fields"`
}

func ParseBibTeX(data []byte) ([]BibTeXEntry, error) {
	input := string(data)
	var entries []BibTeXEntry
	for i := 0; i < len(input); {
		at := strings.IndexByte(input[i:], '@')
		if at < 0 {
			break
		}
		i += at + 1
		i = skipSpace(input, i)
		typeStart := i
		for i < len(input) && (unicode.IsLetter(rune(input[i])) || input[i] == '_') {
			i++
		}
		if typeStart == i {
			return nil, fmt.Errorf("bibtex entry at byte %d has no type", i)
		}
		entryType := strings.ToLower(strings.TrimSpace(input[typeStart:i]))
		i = skipSpace(input, i)
		if i >= len(input) || (input[i] != '{' && input[i] != '(') {
			return nil, fmt.Errorf("bibtex entry %q has no opening delimiter", entryType)
		}
		open := input[i]
		close := byte('}')
		if open == '(' {
			close = ')'
		}
		i++
		keyStart := i
		for i < len(input) && input[i] != ',' && input[i] != close {
			i++
		}
		key := strings.TrimSpace(input[keyStart:i])
		if key == "" {
			return nil, fmt.Errorf("bibtex entry %q has no key", entryType)
		}
		if i >= len(input) || input[i] != ',' {
			return nil, fmt.Errorf("bibtex entry %q has no fields", key)
		}
		i++
		fields, next, err := parseFields(input, i, close)
		if err != nil {
			return nil, fmt.Errorf("bibtex entry %q: %w", key, err)
		}
		i = next
		entries = append(entries, BibTeXEntry{Type: entryType, Key: key, Fields: fields})
	}
	return entries, nil
}

func CandidatesFromBibTeX(entries []BibTeXEntry, startIndex int) []contracts.ReferenceCandidate {
	if startIndex <= 0 {
		startIndex = 1
	}
	candidates := make([]contracts.ReferenceCandidate, 0, len(entries))
	for i, entry := range entries {
		fields := normalizedFields(entry.Fields)
		title := fields["title"]
		if title == "" {
			title = entry.Key
		}
		doi := fields["doi"]
		url := fields["url"]
		candidate := contracts.ReferenceCandidate{
			ID:          fmt.Sprintf("cand_%03d", startIndex+i),
			Title:       title,
			Authors:     splitAuthors(fields["author"]),
			Year:        parseYear(fields["year"]),
			Source:      "bibtex",
			DOI:         doi,
			URL:         url,
			Abstract:    fields["abstract"],
			Venue:       firstNonEmpty(fields["journal"], fields["booktitle"], fields["publisher"]),
			DedupeGroup: dedupeGroup(entry.Key, doi, url),
			Status:      "pending",
		}
		candidates = append(candidates, candidate)
	}
	return candidates
}

func parseFields(input string, i int, close byte) (map[string]string, int, error) {
	fields := map[string]string{}
	for i < len(input) {
		i = skipSpaceAndCommas(input, i)
		if i >= len(input) {
			return nil, i, fmt.Errorf("missing closing delimiter")
		}
		if input[i] == close {
			return fields, i + 1, nil
		}
		nameStart := i
		for i < len(input) && (unicode.IsLetter(rune(input[i])) || unicode.IsDigit(rune(input[i])) || input[i] == '_' || input[i] == '-') {
			i++
		}
		name := strings.ToLower(strings.TrimSpace(input[nameStart:i]))
		if name == "" {
			return nil, i, fmt.Errorf("field has no name")
		}
		i = skipSpace(input, i)
		if i >= len(input) || input[i] != '=' {
			return nil, i, fmt.Errorf("field %q has no '='", name)
		}
		i++
		i = skipSpace(input, i)
		value, next, err := parseValue(input, i)
		if err != nil {
			return nil, i, fmt.Errorf("field %q: %w", name, err)
		}
		fields[name] = value
		i = next
	}
	return nil, i, fmt.Errorf("missing closing delimiter")
}

func parseValue(input string, i int) (string, int, error) {
	if i >= len(input) {
		return "", i, fmt.Errorf("missing value")
	}
	switch input[i] {
	case '{':
		return parseBalanced(input, i, '{', '}')
	case '"':
		return parseBalanced(input, i, '"', '"')
	default:
		start := i
		for i < len(input) && input[i] != ',' && input[i] != '}' && input[i] != ')' {
			i++
		}
		return cleanValue(input[start:i]), i, nil
	}
}

func parseBalanced(input string, i int, open, close byte) (string, int, error) {
	i++
	start := i
	depth := 1
	escaped := false
	for i < len(input) {
		ch := input[i]
		if escaped {
			escaped = false
			i++
			continue
		}
		if ch == '\\' {
			escaped = true
			i++
			continue
		}
		if open != '"' && ch == open {
			depth++
		}
		if ch == close {
			depth--
			if depth == 0 {
				return cleanValue(input[start:i]), i + 1, nil
			}
		}
		i++
	}
	return "", i, fmt.Errorf("unterminated value")
}

func skipSpace(input string, i int) int {
	for i < len(input) && unicode.IsSpace(rune(input[i])) {
		i++
	}
	return i
}

func skipSpaceAndCommas(input string, i int) int {
	for i < len(input) && (unicode.IsSpace(rune(input[i])) || input[i] == ',') {
		i++
	}
	return i
}

func cleanValue(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.ReplaceAll(value, "\r", " ")
	return strings.Join(strings.Fields(value), " ")
}

func normalizedFields(fields map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range fields {
		out[strings.ToLower(k)] = strings.TrimSpace(v)
	}
	return out
}

func splitAuthors(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, " and ")
	authors := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			authors = append(authors, part)
		}
	}
	return authors
}

func parseYear(value string) int {
	year, _ := strconv.Atoi(strings.TrimSpace(value))
	return year
}

func dedupeGroup(key, doi, url string) string {
	switch {
	case strings.TrimSpace(doi) != "":
		return "doi:" + strings.ToLower(strings.TrimSpace(doi))
	case strings.TrimSpace(url) != "":
		return "url:" + strings.TrimSpace(url)
	default:
		return "bibtex:" + strings.TrimSpace(key)
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
