package references

import (
	"fmt"
	"strings"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/store"
)

// WriteCandidates writes candidates to both JSON and Markdown formats.
// Returns the list of output paths written.
func WriteCandidates(s store.Store, candidates []contracts.ReferenceCandidate) ([]string, error) {
	candidatesData := contracts.ReferenceCandidates{Items: candidates}
	jsonPath := s.Path("references", "candidates.json")
	mdPath := s.Path("references", "candidates.md")

	var outputs []string
	if res, err := store.WriteJSON(jsonPath, candidatesData, store.Overwrite); err != nil {
		return nil, err
	} else {
		outputs = append(outputs, res.Path)
	}

	mdContent := FormatCandidatesMarkdown(candidates)
	if res, err := store.WriteFile(mdPath, []byte(mdContent), store.Overwrite); err != nil {
		return nil, err
	} else {
		outputs = append(outputs, res.Path)
	}

	return outputs, nil
}

// FormatCandidatesMarkdown formats a list of candidates into Markdown.
func FormatCandidatesMarkdown(candidates []contracts.ReferenceCandidate) string {
	var b strings.Builder
	b.WriteString("# Reference Candidates\n\n")
	if len(candidates) == 0 {
		b.WriteString("No reference candidates.\n")
		return b.String()
	}
	for _, candidate := range candidates {
		fmt.Fprintf(&b, "## %s\n\n", candidate.ID)
		fmt.Fprintf(&b, "- Title: %s\n", candidate.Title)
		fmt.Fprintf(&b, "- Authors: %s\n", strings.Join(candidate.Authors, ", "))
		if candidate.Year != 0 {
			fmt.Fprintf(&b, "- Year: %d\n", candidate.Year)
		}
		fmt.Fprintf(&b, "- Source: %s\n", SourceLabel(candidate.Source))
		if candidate.DOI != "" {
			fmt.Fprintf(&b, "- DOI: %s\n", candidate.DOI)
		}
		if candidate.URL != "" {
			fmt.Fprintf(&b, "- URL: %s\n", candidate.URL)
		}
		if label := AvailabilityLabel(candidate.Availability); label != "" {
			fmt.Fprintf(&b, "- 可获取性：%s\n", label)
		}
		if label := ReliabilityLabel(candidate.Reliability); label != "" {
			fmt.Fprintf(&b, "- 可靠性：%s\n", label)
		}
		if candidate.AccessURL != "" {
			fmt.Fprintf(&b, "- 优先访问链接：%s\n", candidate.AccessURL)
		}
		if summary := CandidateSummary(candidate.Abstract); summary != "" {
			fmt.Fprintf(&b, "- 概要：%s\n", summary)
		}
		if candidate.Venue != "" {
			fmt.Fprintf(&b, "- Venue: %s\n", candidate.Venue)
		}
		if candidate.CitationCount != 0 {
			fmt.Fprintf(&b, "- Citation count: %d\n", candidate.CitationCount)
		}
		if candidate.DedupeGroup != "" {
			fmt.Fprintf(&b, "- Dedupe group: %s\n", candidate.DedupeGroup)
		}
		b.WriteString("\n")
	}
	return b.String()
}
