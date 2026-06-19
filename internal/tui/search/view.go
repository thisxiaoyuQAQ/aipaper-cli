package search

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/i18n"
	domainsearch "github.com/thisxiaoyuQAQ/aipaper-cli/internal/search"
)

var (
	searchTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("86"))

	searchPanelStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("105")).
				Padding(1, 2)

	searchLabelStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("117"))

	searchMetricStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("250"))

	searchSuccessStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("42"))

	searchInfoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("86"))

	searchWarnStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214"))

	searchErrorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("196"))

	searchHintStyle = lipgloss.NewStyle().
			Faint(true)
)

func (m Model) View() string {
	var b strings.Builder
	b.WriteString(searchTitleStyle.Render(m.i18n.Text(i18n.SearchTitle)))
	b.WriteString("\n\n")

	var panel strings.Builder
	switch m.status {
	case StatusSearching:
		panel.WriteString(searchInfoStyle.Render(m.i18n.Text(i18n.SearchStatusSearching)))
		panel.WriteString("\n\n")
		if m.materialCount > 0 {
			fmt.Fprintf(&panel, searchMetricStyle.Render(m.i18n.Text(i18n.SearchMaterialCandidates))+"\n", m.materialCount)
		}
		m.writeMaterialCandidateHint(&panel)
	case StatusDisabled:
		panel.WriteString(searchWarnStyle.Render(m.i18n.Text(i18n.SearchStatusDisabled)))
		panel.WriteString("\n\n")
		fmt.Fprintf(&panel, searchMetricStyle.Render(m.i18n.Text(i18n.SearchMaterialCandidates))+"\n", m.materialCount)
		m.writeMaterialCandidateHint(&panel)
		fmt.Fprintf(&panel, searchMetricStyle.Render(m.i18n.Text(i18n.SearchFinalCandidates))+"\n\n", m.FinalCount())
		panel.WriteString(searchHintStyle.Render(m.i18n.Text(i18n.SearchContinueHint)))
		panel.WriteString("\n")
		panel.WriteString(searchHintStyle.Render(m.i18n.Text(i18n.SearchBackQuitHint)))
		panel.WriteString("\n")
	case StatusComplete:
		panel.WriteString(searchSuccessStyle.Render(m.i18n.Text(i18n.SearchStatusComplete)))
		panel.WriteString("\n\n")
		fmt.Fprintf(&panel, searchMetricStyle.Render(m.i18n.Text(i18n.SearchMaterialCandidates))+"\n", m.materialCount)
		m.writeMaterialCandidateHint(&panel)
		fmt.Fprintf(&panel, searchMetricStyle.Render(m.i18n.Text(i18n.SearchSearchCandidates))+"\n", m.searchCount)
		fmt.Fprintf(&panel, searchMetricStyle.Render(m.i18n.Text(i18n.SearchFinalAfterDedup))+"\n\n", m.FinalCount())
		if len(m.searchErrors) > 0 {
			panel.WriteString(searchWarnStyle.Render(m.i18n.Text(i18n.SearchProviderWarnings)))
			panel.WriteString("\n")
			for _, err := range m.searchErrors {
				fmt.Fprintf(&panel, "  - %s\n", searchWarnStyle.Render(m.formatProviderError(err)))
			}
			panel.WriteString("\n")
		}
		panel.WriteString(searchHintStyle.Render(m.i18n.Text(i18n.SearchContinueHint)))
		panel.WriteString("\n")
		panel.WriteString(searchHintStyle.Render(m.i18n.Text(i18n.SearchBackQuitHint)))
		panel.WriteString("\n")
	case StatusAllFailed:
		panel.WriteString(searchErrorStyle.Render(m.i18n.Text(i18n.SearchStatusAllFailed)))
		panel.WriteString("\n\n")
		fmt.Fprintf(&panel, searchMetricStyle.Render(m.i18n.Text(i18n.SearchMaterialCandidates))+"\n", m.materialCount)
		m.writeMaterialCandidateHint(&panel)
		fmt.Fprintf(&panel, searchMetricStyle.Render(m.i18n.Text(i18n.SearchFinalCandidates))+"\n\n", m.FinalCount())
		panel.WriteString(searchErrorStyle.Render(m.i18n.Text(i18n.SearchErrors)))
		panel.WriteString("\n")
		for _, err := range m.searchErrors {
			fmt.Fprintf(&panel, "  - %s\n", searchErrorStyle.Render(m.formatProviderError(err)))
		}
		panel.WriteString("\n")
		panel.WriteString(searchHintStyle.Render(m.i18n.Text(i18n.SearchRetrySkipHint)))
		panel.WriteString("\n")
	case StatusError:
		panel.WriteString(searchErrorStyle.Render(m.i18n.Text(i18n.SearchStatusError)))
		panel.WriteString("\n\n")
		if m.err != nil {
			fmt.Fprintf(&panel, "%s\n\n", searchErrorStyle.Render(fmt.Sprintf("%s: %s", m.i18n.Text(i18n.CommonErrorPrefix), m.err.Error())))
		}
		panel.WriteString(searchHintStyle.Render(m.i18n.Text(i18n.SearchRetryHint)))
		panel.WriteString("\n")
	}

	b.WriteString(searchPanelStyle.Render(panel.String()))
	b.WriteString("\n")
	return b.String()
}

func (m Model) formatProviderError(err domainsearch.ProviderError) string {
	message := err.Message
	switch err.Code {
	case domainsearch.CodeSearchRateLimited:
		message = m.i18n.Text(i18n.SearchRateLimitNotice)
	case domainsearch.CodeSearchTimeout:
		message = m.i18n.Text(i18n.SearchTimeoutNotice)
	}
	msg := err.Source + ": " + message
	if err.Retryable {
		msg += " " + m.i18n.Text(i18n.SearchRetryable)
	}
	return msg
}

func (m Model) shouldShowMaterialCandidateHint() bool {
	return m.materialCount == 0 && m.parsedMaterialCount > 0
}

func (m Model) writeMaterialCandidateHint(b *strings.Builder) {
	if !m.shouldShowMaterialCandidateHint() {
		return
	}
	b.WriteString(searchHintStyle.Render(m.i18n.Text(i18n.SearchMaterialCandidateHint)))
	b.WriteString("\n")
}
