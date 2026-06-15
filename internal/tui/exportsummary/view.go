package exportsummary

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/export"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/i18n"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("86"))

	pathStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("250"))

	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("42"))

	warnStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196"))

	hintStyle = lipgloss.NewStyle().
			Faint(true)
)

// View renders the ExportSummary screen.
func (m Model) View() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render(m.i18n.Text(i18n.ExportTitle)))
	b.WriteString("\n\n")

	outputDir := m.store.Path("final")
	b.WriteString(m.i18n.Text(i18n.ExportOutputDir))
	b.WriteString(": ")
	b.WriteString(pathStyle.Render(outputDir))
	b.WriteString("\n\n")

	if !m.exported {
		b.WriteString(m.i18n.Text(i18n.ExportInProgress))
		b.WriteString("\n")
		return b.String()
	}

	if m.err != nil {
		b.WriteString(m.renderError(m.err))
		b.WriteString("\n")
		b.WriteString(hintStyle.Render(m.i18n.Text(i18n.ExportRetryBackHint)))
		b.WriteString("\n")
		return b.String()
	}

	b.WriteString(successStyle.Render(m.i18n.Text(i18n.ExportComplete)))
	b.WriteString("\n\n")

	b.WriteString(m.renderOutputs(m.result))
	b.WriteString(m.renderQualitySummary(m.result))
	b.WriteString(m.renderDocx(m.result))
	b.WriteString(m.renderIssues(m.result))

	b.WriteString("\n")
	b.WriteString(hintStyle.Render(m.i18n.Text(i18n.ExportDoneRetryHint)))
	b.WriteString("\n")
	return b.String()
}

func (m Model) renderError(err error) string {
	var b strings.Builder
	b.WriteString(errorStyle.Render(m.i18n.Text(i18n.ExportFailed)))
	b.WriteString("\n")

	if exportErr, ok := err.(export.Error); ok {
		b.WriteString(errorStyle.Render(fmt.Sprintf("  %s: %s", exportErr.Code, exportErr.Message)))
		b.WriteString("\n")
		if hint := m.errorHint(exportErr.Code); hint != "" {
			b.WriteString(hintStyle.Render("  " + hint))
			b.WriteString("\n")
		}
	} else {
		b.WriteString(errorStyle.Render("  " + err.Error()))
		b.WriteString("\n")
	}
	return b.String()
}

func (m Model) errorHint(code string) string {
	switch code {
	case export.CodeNoAcceptedChapters:
		return m.i18n.Text(i18n.ExportHintNoChapters)
	case export.CodeUnconfirmedReference:
		return m.i18n.Text(i18n.ExportHintUnconfirmedRef)
	default:
		return ""
	}
}

func (m Model) renderOutputs(result export.Result) string {
	var b strings.Builder
	b.WriteString(m.i18n.Text(i18n.ExportOutputs))
	b.WriteString(":\n")
	if len(result.Outputs) == 0 {
		b.WriteString(hintStyle.Render(m.i18n.Text(i18n.ExportNoOutputs)))
		b.WriteString("\n")
		return b.String()
	}
	for _, out := range result.Outputs {
		b.WriteString("  - ")
		b.WriteString(pathStyle.Render(out.Path))
		b.WriteString("\n")
	}
	return b.String()
}

func (m Model) renderQualitySummary(result export.Result) string {
	// Check if quality-report.md exists in outputs
	hasQualityReport := false
	for _, out := range result.Outputs {
		if strings.Contains(out.Path, "quality-report.md") {
			hasQualityReport = true
			break
		}
	}
	if !hasQualityReport {
		return ""
	}

	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(successStyle.Render(m.i18n.Text(i18n.ExportQualitySummary)))
	b.WriteString("\n")

	// Extract quality conclusion from result metadata if available
	if result.Metadata != nil {
		if qualityConclusion, ok := result.Metadata["quality_conclusion"].(string); ok && qualityConclusion != "" {
			b.WriteString("  ")
			b.WriteString(hintStyle.Render(qualityConclusion))
			b.WriteString("\n")
		}
	}

	// Show quality-report.md entry hint
	b.WriteString("  ")
	b.WriteString(hintStyle.Render(m.i18n.Text(i18n.ExportQualityReport)))
	b.WriteString("\n")

	return b.String()
}

func (m Model) renderDocx(result export.Result) string {
	if result.DocxWritten {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(warnStyle.Render(m.i18n.Text(i18n.ExportDocxDegraded)))
	b.WriteString("\n")
	b.WriteString(hintStyle.Render(m.i18n.Text(i18n.ExportDocxDegradedHint)))
	b.WriteString("\n")
	return b.String()
}

func (m Model) renderIssues(result export.Result) string {
	if len(result.Issues) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(warnStyle.Render(m.i18n.Text(i18n.ExportIssues)))
	b.WriteString("\n")
	for _, issue := range result.Issues {
		line := fmt.Sprintf("  - %s: %s", issue.Code, issue.Message)
		if issue.ChapterID != "" {
			line += fmt.Sprintf(m.i18n.Text(i18n.ExportIssueChapter), issue.ChapterID)
		}
		if issue.ReferenceKey != "" {
			line += fmt.Sprintf(m.i18n.Text(i18n.ExportIssueReference), issue.ReferenceKey)
		}
		b.WriteString(warnStyle.Render(line))
		b.WriteString("\n")
	}
	return b.String()
}
