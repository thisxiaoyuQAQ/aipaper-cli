package materials

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
)

func (m Model) View() string {
	var b strings.Builder
	b.WriteString("Materials scan\n\n")
	fmt.Fprintf(&b, "Directory: %s\n", m.displayDir)

	switch m.status {
	case StatusScanning:
		b.WriteString("Status: scanning materials...\n")
	case StatusEmpty:
		if m.createdDir {
			b.WriteString("Status: material directory was created. Add files, then press Enter to scan.\n")
		} else {
			b.WriteString("Status: no material files found. Add files, then press Enter to scan.\n")
		}
	case StatusComplete:
		b.WriteString("Status: scan complete.\n")
	case StatusAllFailed:
		b.WriteString("Status: no material could be parsed.\n")
	case StatusError:
		b.WriteString("Status: scan failed.\n")
	}

	if m.stats.Total > 0 || m.stats.Candidates > 0 {
		fmt.Fprintf(
			&b,
			"\nParsed: %d  Degraded: %d  Failed: %d  Skipped: %d  Candidates: %d\n",
			m.stats.Parsed,
			m.stats.Degraded,
			m.stats.Failed,
			m.stats.Skipped,
			m.stats.Candidates,
		)
	}
	if m.details && len(m.manifest.Items) > 0 {
		b.WriteString("\nDetails\n")
		for _, item := range m.manifest.Items {
			line := fmt.Sprintf("- %s %s [%s/%s]", item.ID, safeDisplayPath(item.Path), item.Kind, item.Status)
			if item.Degraded {
				line += " degraded"
			}
			if item.Error != "" {
				line += ": " + safeDisplayError(item.Error)
			}
			b.WriteString(line)
			b.WriteString("\n")
		}
	}
	if m.err != nil {
		fmt.Fprintf(&b, "\nError: %s\n", safeDisplayError(m.err.Error()))
	}

	b.WriteString("\nEnter: continue/rescan  r: rescan  d: details  b: requirements  s: skip  q: quit\n")
	return b.String()
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
