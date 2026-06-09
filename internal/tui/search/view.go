package search

import (
	"fmt"
	"strings"

	domainsearch "github.com/thisxiaoyuQAQ/aipaper-cli/internal/search"
)

func (m Model) View() string {
	var b strings.Builder
	b.WriteString("aipaper-cli — Search Progress\n\n")

	switch m.status {
	case StatusSearching:
		b.WriteString("⏳ Searching for reference candidates...\n\n")
		if m.materialCount > 0 {
			fmt.Fprintf(&b, "Material candidates: %d\n", m.materialCount)
		}
	case StatusDisabled:
		b.WriteString("✓ Online search disabled\n\n")
		fmt.Fprintf(&b, "Material candidates: %d\n", m.materialCount)
		fmt.Fprintf(&b, "Final candidates: %d\n\n", m.FinalCount())
		b.WriteString("Press Enter to continue to references confirmation\n")
		b.WriteString("Press B to go back, Q to quit\n")
	case StatusComplete:
		b.WriteString("✓ Search complete\n\n")
		fmt.Fprintf(&b, "Material candidates: %d\n", m.materialCount)
		fmt.Fprintf(&b, "Search candidates: %d\n", m.searchCount)
		fmt.Fprintf(&b, "Final candidates (after deduplication): %d\n\n", m.FinalCount())
		if len(m.searchErrors) > 0 {
			b.WriteString("⚠ Provider warnings:\n")
			for _, err := range m.searchErrors {
				fmt.Fprintf(&b, "  - %s: %s\n", err.Source, err.Message)
			}
			b.WriteString("\n")
		}
		b.WriteString("Press Enter to continue to references confirmation\n")
		b.WriteString("Press B to go back, Q to quit\n")
	case StatusAllFailed:
		b.WriteString("✗ All search providers failed\n\n")
		fmt.Fprintf(&b, "Material candidates: %d (will be used)\n", m.materialCount)
		fmt.Fprintf(&b, "Final candidates: %d\n\n", m.FinalCount())
		b.WriteString("Errors:\n")
		for _, err := range m.searchErrors {
			fmt.Fprintf(&b, "  - %s: %s", err.Source, err.Message)
			if err.Retryable {
				b.WriteString(" (retryable)")
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
		b.WriteString("Press R to retry, S to skip search, B to go back, Q to quit\n")
	case StatusError:
		b.WriteString("✗ Search error\n\n")
		if m.err != nil {
			fmt.Fprintf(&b, "Error: %s\n\n", m.err.Error())
		}
		b.WriteString("Press R to retry, B to go back, Q to quit\n")
	}

	return b.String()
}

func formatProviderError(err domainsearch.ProviderError) string {
	msg := err.Source + ": " + err.Message
	if err.Retryable {
		msg += " (retryable)"
	}
	return msg
}
