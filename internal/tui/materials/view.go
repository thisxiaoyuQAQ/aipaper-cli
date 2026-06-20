package materials

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/i18n"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/tui/ui"
)

func (m Model) View() string {
	var b strings.Builder
	b.WriteString(m.i18n.Text(i18n.MaterialsTitle))
	b.WriteString("\n\n")
	fmt.Fprintf(&b, "%s: %s\n", m.i18n.Text(i18n.MaterialsDirectory), m.displayDir)

	switch m.status {
	case StatusScanning:
		b.WriteString(m.i18n.Text(i18n.MaterialsStatusScanning))
		b.WriteString("\n")
	case StatusEmpty:
		if m.createdDir {
			b.WriteString(m.i18n.Text(i18n.MaterialsStatusCreated))
		} else {
			b.WriteString(m.i18n.Text(i18n.MaterialsStatusEmpty))
		}
		b.WriteString("\n")
	case StatusComplete:
		b.WriteString(m.i18n.Text(i18n.MaterialsStatusComplete))
		b.WriteString("\n")
	case StatusAllFailed:
		b.WriteString(m.i18n.Text(i18n.MaterialsStatusAllFailed))
		b.WriteString("\n")
	case StatusError:
		b.WriteString(m.i18n.Text(i18n.MaterialsStatusError))
		b.WriteString("\n")
	}

	if m.stats.Total > 0 || m.stats.Candidates > 0 {
		fmt.Fprintf(
			&b,
			"\n"+m.i18n.Text(i18n.MaterialsStatsLine)+"\n",
			m.stats.Parsed,
			m.stats.Degraded,
			m.stats.Failed,
			m.stats.Skipped,
			m.stats.Candidates,
		)
	}
	if m.details && len(m.manifest.Items) > 0 {
		b.WriteString("\n")
		b.WriteString(m.i18n.Text(i18n.MaterialsDetails))
		b.WriteString("\n")
		for _, item := range m.manifest.Items {
			line := fmt.Sprintf("- %s %s [%s/%s]", item.ID, safeDisplayPath(item.Path), item.Kind, item.Status)
			if item.Degraded {
				line += " " + m.i18n.Text(i18n.MaterialsDegraded)
			}
			if item.Error != "" {
				line += ": " + safeDisplayError(item.Error)
			}
			b.WriteString(m.wrapLine(line))
		}
	}
	if m.err != nil {
		fmt.Fprintf(&b, "\n%s: %s\n", m.i18n.Text(i18n.CommonErrorPrefix), safeDisplayError(m.err.Error()))
	}

	b.WriteString("\n")
	b.WriteString(m.i18n.Text(i18n.MaterialsFooter))
	b.WriteString("\n")
	return b.String()
}

// wrapLine wraps a long manifest/error line to the terminal width and
// rejoins with newlines, indenting continuation lines under the bullet. When
// the width is unknown the line is returned unchanged.
func (m Model) wrapLine(line string) string {
	if m.width <= 0 {
		return line + "\n"
	}
	width := m.width - 2
	if width < 10 {
		width = 10
	}
	wrapped := ui.WrapCells(line, width)
	if len(wrapped) <= 1 {
		return line + "\n"
	}
	var sb strings.Builder
	for i, wl := range wrapped {
		if i == 0 {
			sb.WriteString(wl)
		} else {
			sb.WriteString("\n  ")
			sb.WriteString(wl)
		}
	}
	sb.WriteString("\n")
	return sb.String()
}

func safeDisplayPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if parsed, err := url.Parse(path); err == nil && parsed.Scheme != "" && parsed.Host != "" {
		parsed.RawQuery = ""
		parsed.Fragment = ""
		return parsed.String()
	}
	if filepath.IsAbs(path) {
		base := filepath.Base(path)
		if base != "." && base != string(filepath.Separator) {
			return base
		}
	}
	return path
}

func safeDisplayError(message string) string {
	message = strings.TrimSpace(message)
	if message == "" {
		return ""
	}
	fields := strings.Fields(message)
	for i, field := range fields {
		fields[i] = safeDisplayPath(strings.Trim(field, `"'`))
	}
	return strings.Join(fields, " ")
}
