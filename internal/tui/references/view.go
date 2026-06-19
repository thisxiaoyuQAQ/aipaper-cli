package referencestui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/i18n"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/references"
)

var (
	referencesTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("86"))

	referencesPanelStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("105")).
				Padding(1, 2)

	referencesActiveStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("220"))

	referencesLabelStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("117"))

	referencesMetaStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("250"))

	referencesSelectedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("42"))

	referencesRejectedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("196"))

	referencesPendingStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("250"))

	referencesErrorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("196"))

	referencesHintStyle = lipgloss.NewStyle().
				Faint(true)
)

func (m Model) View() string {
	var b strings.Builder
	visible := m.visible()
	header := fmt.Sprintf(m.i18n.Text(i18n.ReferencesHeader), len(visible), len(m.selected), len(m.rejected))
	b.WriteString(referencesTitleStyle.Render(header))
	b.WriteString("\n")

	info := []string{referencesHintStyle.Render(m.sortLabel() + ": " + m.sortModeLabel())}
	if m.searching {
		info = append(info, referencesActiveStyle.Render(fmt.Sprintf("%s: %s", m.i18n.Text(i18n.ReferencesSearch), m.filter)))
	}
	b.WriteString(strings.Join(info, "  "))
	b.WriteString("\n\n")

	var panel strings.Builder
	if m.err != nil {
		fmt.Fprintf(&panel, "%s\n\n", referencesErrorStyle.Render(fmt.Sprintf("%s: %v", m.i18n.Text(i18n.CommonErrorPrefix), m.err)))
	}
	if len(visible) == 0 {
		panel.WriteString(referencesHintStyle.Render(m.i18n.Text(i18n.ReferencesNoMatches)))
		panel.WriteString("\n")
		panel.WriteString("\n")
		panel.WriteString(referencesHintStyle.Render(m.footerHint()))
		panel.WriteString("\n")
		b.WriteString(referencesPanelStyle.Render(panel.String()))
		b.WriteString("\n")
		return b.String()
	}
	for i, candidate := range visible {
		cursor := " "
		title := candidate.Title
		id := candidate.ID
		if i == m.cursor {
			cursor = referencesActiveStyle.Render("▶")
			title = referencesActiveStyle.Render(title)
			id = referencesActiveStyle.Render(id)
		}
		fmt.Fprintf(&panel, "%s %s %s %s\n", cursor, m.renderState(candidate), id, title)
		if subtitle := candidateSubtitle(candidate); subtitle != "" {
			fmt.Fprintf(&panel, "    %s\n", referencesMetaStyle.Render(subtitle))
		}
		if candidate.DOI != "" {
			fmt.Fprintf(&panel, "    %s: %s\n", m.i18n.Text(i18n.ReferencesDOI), referencesMetaStyle.Render(candidate.DOI))
		} else if candidate.URL != "" {
			fmt.Fprintf(&panel, "    %s: %s\n", m.i18n.Text(i18n.ReferencesURL), referencesMetaStyle.Render(candidate.URL))
		}
		if summary := references.CandidateSummary(candidate.Abstract); summary != "" {
			fmt.Fprintf(&panel, "    %s: %s\n", m.i18n.Text(i18n.ReferencesSummary), summary)
		}
		if candidate.RelevanceScore != 0 {
			fmt.Fprintf(&panel, "    %s: %s\n", m.i18n.Text(i18n.ReferencesRelevance), referencesMetaStyle.Render(fmt.Sprintf("%.2f", candidate.RelevanceScore)))
		}
		if candidate.RelevanceReason != "" {
			fmt.Fprintf(&panel, "    %s: %s\n", m.i18n.Text(i18n.ReferencesReason), candidate.RelevanceReason)
		}
		panel.WriteString("\n")
	}
	panel.WriteString(referencesHintStyle.Render(m.footerHint()))
	panel.WriteString("\n")

	b.WriteString(referencesPanelStyle.Render(panel.String()))
	b.WriteString("\n")
	return b.String()
}

func (m Model) renderState(candidate contracts.ReferenceCandidate) string {
	switch {
	case m.selected[candidate.ID]:
		return referencesSelectedStyle.Render("[x]")
	case m.rejected[candidate.ID]:
		return referencesRejectedStyle.Render("[-]")
	default:
		return referencesPendingStyle.Render("[ ]")
	}
}

func (m Model) sortLabel() string {
	if m.i18n.Lang() == i18n.En {
		return "Sort"
	}
	return "排序"
}

func (m Model) sortModeLabel() string {
	switch m.sortMode {
	case SortRelevance:
		return "relevance"
	case SortYear:
		return "year"
	case SortTitle:
		return "title"
	default:
		return "original"
	}
}

func (m Model) footerHint() string {
	if m.i18n.Lang() == i18n.En {
		if m.searching {
			return "Enter/Esc: finish search  Backspace: delete"
		}
		return "↑/↓: move  Space: select  r: reject  a: select high relevance  /: search  s: sort  Enter: confirm  q: quit"
	}
	if m.searching {
		return "Enter/Esc：结束搜索  Backspace：删除"
	}
	return "↑/↓：移动  Space：选择  r：拒绝  a：选择高相关  /：搜索  s：排序  Enter：确认  q：退出"
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
		parts = append(parts, references.SourceLabel(candidate.Source))
	}
	if label := references.AvailabilityLabel(candidate.Availability); label != "" {
		parts = append(parts, "可获取性："+label)
	}
	if label := references.ReliabilityLabel(candidate.Reliability); label != "" {
		parts = append(parts, "可靠性："+label)
	}
	if candidate.Venue != "" {
		parts = append(parts, candidate.Venue)
	}
	return strings.Join(parts, " | ")
}
