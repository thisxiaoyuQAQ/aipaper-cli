package search

import (
	"fmt"
	"strings"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/i18n"
	domainsearch "github.com/thisxiaoyuQAQ/aipaper-cli/internal/search"
)

func (m Model) View() string {
	var b strings.Builder
	b.WriteString(m.i18n.Text(i18n.SearchTitle))
	b.WriteString("\n\n")

	switch m.status {
	case StatusSearching:
		b.WriteString(m.i18n.Text(i18n.SearchStatusSearching))
		b.WriteString("\n\n")
		if m.materialCount > 0 {
			fmt.Fprintf(&b, m.i18n.Text(i18n.SearchMaterialCandidates)+"\n", m.materialCount)
		}
	case StatusDisabled:
		b.WriteString(m.i18n.Text(i18n.SearchStatusDisabled))
		b.WriteString("\n\n")
		fmt.Fprintf(&b, m.i18n.Text(i18n.SearchMaterialCandidates)+"\n", m.materialCount)
		fmt.Fprintf(&b, m.i18n.Text(i18n.SearchFinalCandidates)+"\n\n", m.FinalCount())
		b.WriteString(m.i18n.Text(i18n.SearchContinueHint))
		b.WriteString("\n")
		b.WriteString(m.i18n.Text(i18n.SearchBackQuitHint))
		b.WriteString("\n")
	case StatusComplete:
		b.WriteString(m.i18n.Text(i18n.SearchStatusComplete))
		b.WriteString("\n\n")
		fmt.Fprintf(&b, m.i18n.Text(i18n.SearchMaterialCandidates)+"\n", m.materialCount)
		fmt.Fprintf(&b, m.i18n.Text(i18n.SearchSearchCandidates)+"\n", m.searchCount)
		fmt.Fprintf(&b, m.i18n.Text(i18n.SearchFinalAfterDedup)+"\n\n", m.FinalCount())
		if len(m.searchErrors) > 0 {
			b.WriteString(m.i18n.Text(i18n.SearchProviderWarnings))
			b.WriteString("\n")
			for _, err := range m.searchErrors {
				fmt.Fprintf(&b, "  - %s\n", m.formatProviderError(err))
			}
			b.WriteString("\n")
		}
		b.WriteString(m.i18n.Text(i18n.SearchContinueHint))
		b.WriteString("\n")
		b.WriteString(m.i18n.Text(i18n.SearchBackQuitHint))
		b.WriteString("\n")
	case StatusAllFailed:
		b.WriteString(m.i18n.Text(i18n.SearchStatusAllFailed))
		b.WriteString("\n\n")
		fmt.Fprintf(&b, m.i18n.Text(i18n.SearchMaterialCandidates)+"\n", m.materialCount)
		fmt.Fprintf(&b, m.i18n.Text(i18n.SearchFinalCandidates)+"\n\n", m.FinalCount())
		b.WriteString(m.i18n.Text(i18n.SearchErrors))
		b.WriteString("\n")
		for _, err := range m.searchErrors {
			fmt.Fprintf(&b, "  - %s\n", m.formatProviderError(err))
		}
		b.WriteString("\n")
		b.WriteString(m.i18n.Text(i18n.SearchRetrySkipHint))
		b.WriteString("\n")
	case StatusError:
		b.WriteString(m.i18n.Text(i18n.SearchStatusError))
		b.WriteString("\n\n")
		if m.err != nil {
			fmt.Fprintf(&b, "%s: %s\n\n", m.i18n.Text(i18n.CommonErrorPrefix), m.err.Error())
		}
		b.WriteString(m.i18n.Text(i18n.SearchRetryHint))
		b.WriteString("\n")
	}

	return b.String()
}

func (m Model) formatProviderError(err domainsearch.ProviderError) string {
	msg := err.Source + ": " + err.Message
	if err.Retryable {
		msg += " " + m.i18n.Text(i18n.SearchRetryable)
	}
	return msg
}
