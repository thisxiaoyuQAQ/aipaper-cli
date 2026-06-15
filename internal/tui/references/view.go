package referencestui

import (
	"fmt"
	"strings"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/i18n"
)

func (m Model) View() string {
	var b strings.Builder
	visible := m.visible()
	fmt.Fprintf(&b, m.i18n.Text(i18n.ReferencesHeader)+"\n", len(visible), len(m.selected), len(m.rejected))
	if m.searching {
		fmt.Fprintf(&b, "%s: %s\n", m.i18n.Text(i18n.ReferencesSearch), m.filter)
	}
	b.WriteString("\n")
	if len(visible) == 0 {
		b.WriteString(m.i18n.Text(i18n.ReferencesNoMatches))
		b.WriteString("\n")
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
			fmt.Fprintf(&b, "    %s: %s\n", m.i18n.Text(i18n.ReferencesDOI), candidate.DOI)
		} else if candidate.URL != "" {
			fmt.Fprintf(&b, "    %s: %s\n", m.i18n.Text(i18n.ReferencesURL), candidate.URL)
		}
		if candidate.RelevanceScore != 0 {
			fmt.Fprintf(&b, "    %s: %.2f\n", m.i18n.Text(i18n.ReferencesRelevance), candidate.RelevanceScore)
		}
		if candidate.RelevanceReason != "" {
			fmt.Fprintf(&b, "    %s: %s\n", m.i18n.Text(i18n.ReferencesReason), candidate.RelevanceReason)
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
