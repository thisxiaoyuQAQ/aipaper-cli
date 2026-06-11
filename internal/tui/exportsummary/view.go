package exportsummary

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/export"
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

	b.WriteString(titleStyle.Render("导出摘要"))
	b.WriteString("\n\n")

	outputDir := m.store.Path("final")
	b.WriteString("输出目录: ")
	b.WriteString(pathStyle.Render(outputDir))
	b.WriteString("\n\n")

	if !m.exported {
		b.WriteString("正在导出最终产物...\n")
		return b.String()
	}

	if m.err != nil {
		b.WriteString(renderError(m.err))
		b.WriteString("\n")
		b.WriteString(hintStyle.Render("[r] 重新导出    [b]/[esc] 返回"))
		b.WriteString("\n")
		return b.String()
	}

	b.WriteString(successStyle.Render("✓ 导出完成"))
	b.WriteString("\n\n")

	b.WriteString(renderOutputs(m.result))
	b.WriteString(renderQualitySummary(m.result))
	b.WriteString(renderDocx(m.result))
	b.WriteString(renderIssues(m.result))

	b.WriteString("\n")
	b.WriteString(hintStyle.Render("[enter] 完成    [r] 重新导出"))
	b.WriteString("\n")
	return b.String()
}

func renderError(err error) string {
	var b strings.Builder
	b.WriteString(errorStyle.Render("✗ 导出失败"))
	b.WriteString("\n")

	if exportErr, ok := err.(export.Error); ok {
		b.WriteString(errorStyle.Render(fmt.Sprintf("  %s: %s", exportErr.Code, exportErr.Message)))
		b.WriteString("\n")
		if hint := errorHint(exportErr.Code); hint != "" {
			b.WriteString(hintStyle.Render("  " + hint))
			b.WriteString("\n")
		}
	} else {
		b.WriteString(errorStyle.Render("  " + err.Error()))
		b.WriteString("\n")
	}
	return b.String()
}

func errorHint(code string) string {
	switch code {
	case export.CodeNoAcceptedChapters:
		return "没有已接受的章节，请先完成至少一个章节再导出（未生成错误文件）。"
	case export.CodeUnconfirmedReference:
		return "存在未确认的引用，请先在文献确认阶段确认相关引用（未生成错误文件）。"
	default:
		return ""
	}
}

func renderOutputs(result export.Result) string {
	var b strings.Builder
	b.WriteString("生成文件:\n")
	if len(result.Outputs) == 0 {
		b.WriteString(hintStyle.Render("  (无)"))
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

func renderQualitySummary(result export.Result) string {
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
	b.WriteString(successStyle.Render("质量摘要:"))
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
	b.WriteString(hintStyle.Render("详细质量报告: final/quality-report.md"))
	b.WriteString("\n")

	return b.String()
}

func renderDocx(result export.Result) string {
	if result.DocxWritten {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(warnStyle.Render("⚠ Docx 降级: paper.docx 未生成"))
	b.WriteString("\n")
	b.WriteString(hintStyle.Render("  Markdown (paper.md) 与报告 (report.md) 已成功生成。"))
	b.WriteString("\n")
	return b.String()
}

func renderIssues(result export.Result) string {
	if len(result.Issues) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(warnStyle.Render("问题提示:"))
	b.WriteString("\n")
	for _, issue := range result.Issues {
		line := fmt.Sprintf("  - %s: %s", issue.Code, issue.Message)
		if issue.ChapterID != "" {
			line += fmt.Sprintf(" (章节 %s)", issue.ChapterID)
		}
		if issue.ReferenceKey != "" {
			line += fmt.Sprintf(" (引用 %s)", issue.ReferenceKey)
		}
		b.WriteString(warnStyle.Render(line))
		b.WriteString("\n")
	}
	return b.String()
}
