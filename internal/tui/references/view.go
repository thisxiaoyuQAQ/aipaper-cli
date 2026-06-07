package referencestui

import (
	"fmt"
	"strings"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
)

func (m Model) View() string {
	var b strings.Builder
	visible := m.visible()
	fmt.Fprintf(&b, "Reference candidates (%d visible, %d selected, %d rejected)\n", len(visible), len(m.selected), len(m.rejected))
	if m.searching {
		fmt.Fprintf(&b, "Search: %s\n", m.filter)
	}
	b.WriteString("\n")
	if len(visible) == 0 {
		b.WriteString("No matching candidates.\n")
		return b.String()
	}
	for i, candidate := range visible {
		cursor := " "
		if i == m.cursor {
			cursor = ">"
		}
		state := " "
		switch {
		case m.selected[candidate.ID]:
			state = "x"
		case m.rejected[candidate.ID]:
			state = "-"
		}
		fmt.Fprintf(&b, "%s [%s] %s %s\n", cursor, state, candidate.ID, candidate.Title)
		fmt.Fprintf(&b, "    %s\n", candidateSubtitle(candidate))
		if candidate.DOI != "" {
			fmt.Fprintf(&b, "    DOI: %s\n", candidate.DOI)
		} else if candidate.URL != "" {
			fmt.Fprintf(&b, "    URL: %s\n", candidate.URL)
		}
		if candidate.RelevanceScore != 0 {
			fmt.Fprintf(&b, "    Relevance: %.2f\n", candidate.RelevanceScore)
		}
		if candidate.RelevanceReason != "" {
			fmt.Fprintf(&b, "    Reason: %s\n", candidate.RelevanceReason)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func candidateSubtitle(candidate contracts.ReferenceCandidate) string {
	parts := []string{}
	if len(candidate.Authors) > 0 {
		parts = append(parts, strings.Join(candidate.Authors, ", "))
	}
	if candidate.Year != 0 {
		parts = append(parts, fmt.Sprintf("%d", candidate.Year))
	}
	if candidate.Source != "" {
		parts = append(parts, candidate.Source)
	}
	if candidate.Venue != "" {
		parts = append(parts, candidate.Venue)
	}
	return strings.Join(parts, " | ")
}
