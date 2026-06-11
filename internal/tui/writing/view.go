package writing

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// Styles for the four-region layout
var (
	// Left metrics panel
	metricsStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(0, 1)

	metricLabelStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("86"))

	metricValueStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("250"))

	// Middle-top log panel
	logStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240")).
			Padding(0, 1)

	logRoleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("105"))

	logErrorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196"))

	// Middle-bottom content panel
	contentStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("105")).
			Padding(0, 1)

	contentTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("117"))

	// Right progress panel
	progressStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("141")).
			Padding(0, 1)

	chapterDoneStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("82"))

	chapterActiveStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("227"))

	chapterPendingStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("240"))

	chapterErrorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("196"))

	// Footer
	footerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Italic(true)
)

// View renders the WritingProgress screen.
func (m Model) View() string {
	if m.width < 60 {
		// Narrow terminal: vertical stack layout
		return m.renderNarrowLayout()
	}

	// Wide terminal: four-region layout
	return m.renderWideLayout()
}

func (m Model) renderWideLayout() string {
	// Calculate dimensions
	leftWidth := 30
	rightWidth := 35
	middleWidth := m.width - leftWidth - rightWidth - 6 // borders and padding

	topHeight := m.height / 2
	bottomHeight := m.height - topHeight - 4 // footer and spacing

	if middleWidth < 30 {
		middleWidth = 30
	}
	if topHeight < 10 {
		topHeight = 10
	}
	if bottomHeight < 10 {
		bottomHeight = 10
	}

	// Render regions
	leftPanel := m.renderMetrics(leftWidth, m.height-2)
	topMiddlePanel := m.renderLogs(middleWidth, topHeight)
	bottomMiddlePanel := m.renderContent(middleWidth, bottomHeight)
	rightPanel := m.renderProgress(rightWidth, m.height-2)

	// Combine middle panels vertically
	middlePanel := lipgloss.JoinVertical(lipgloss.Left,
		topMiddlePanel,
		bottomMiddlePanel,
	)

	// Combine all regions horizontally
	mainView := lipgloss.JoinHorizontal(lipgloss.Top,
		leftPanel,
		middlePanel,
		rightPanel,
	)

	// Add footer
	footer := m.renderFooter()

	return lipgloss.JoinVertical(lipgloss.Left, mainView, footer)
}

func (m Model) renderNarrowLayout() string {
	// Vertical stack: metrics -> logs -> content -> progress
	width := m.width - 4

	sections := []string{
		m.renderMetrics(width, 12),
		m.renderLogs(width, 10),
		m.renderContent(width, 10),
		m.renderProgress(width, 10),
		m.renderFooter(),
	}

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func (m Model) renderMetrics(width, height int) string {
	lines := []string{
		m.renderMetricLine("Status", m.formatStatus()),
		m.renderMetricLine("Phase", m.phase),
		m.renderMetricLine("Progress", fmt.Sprintf("%.1f%%", m.totalProgress)),
		"",
		m.renderMetricLine("Words", fmt.Sprintf("%d", m.wordCount)),
		m.renderMetricLine("Model", m.model),
		"",
		m.renderMetricLine("Context", m.formatContext()),
		m.renderMetricLine("Tokens", m.formatTokens()),
		m.renderMetricLine("Cost", m.formatCost()),
		m.renderMetricLine("Elapsed", m.formatElapsed()),
	}

	content := strings.Join(lines, "\n")
	content = truncateToHeight(content, height-2)

	return metricsStyle.Width(width).Height(height).Render(
		lipgloss.NewStyle().Bold(true).Render("📊 Metrics") + "\n\n" + content,
	)
}

func (m Model) renderLogs(width, height int) string {
	visibleLines := height - 4 // header + borders
	if visibleLines < 1 {
		visibleLines = 1
	}

	var lines []string
	startIdx := 0
	if len(m.logs) > visibleLines {
		if m.autoScroll {
			startIdx = len(m.logs) - visibleLines
		} else {
			startIdx = m.logScrollPos
			if startIdx > len(m.logs)-visibleLines {
				startIdx = len(m.logs) - visibleLines
			}
		}
	}

	for i := startIdx; i < len(m.logs) && len(lines) < visibleLines; i++ {
		entry := m.logs[i]
		timestamp := entry.at.Format("15:04:05")
		role := entry.role
		if role == "" {
			role = "system"
		}

		line := fmt.Sprintf("[%s] %s: %s",
			timestamp,
			logRoleStyle.Render(role),
			entry.message,
		)

		if entry.isError {
			line = logErrorStyle.Render(line)
		}

		lines = append(lines, line)
	}

	scrollIndicator := ""
	if !m.autoScroll {
		scrollIndicator = " [manual scroll]"
	}

	header := lipgloss.NewStyle().Bold(true).Render("📝 Logs" + scrollIndicator)
	content := strings.Join(lines, "\n")

	return logStyle.Width(width).Height(height).Render(
		header + "\n\n" + content,
	)
}

func (m Model) renderContent(width, height int) string {
	visibleLines := height - 4

	var lines []string
	startIdx := 0
	if len(m.contentLines) > visibleLines {
		startIdx = len(m.contentLines) - visibleLines
	}

	for i := startIdx; i < len(m.contentLines) && len(lines) < visibleLines; i++ {
		line := m.contentLines[i]
		if len(line) > width-4 {
			line = line[:width-7] + "..."
		}
		lines = append(lines, line)
	}

	chapterLabel := m.currentChapterID
	if chapterLabel == "" {
		chapterLabel = "waiting..."
	}

	header := contentTitleStyle.Render(fmt.Sprintf("📄 Content: %s", chapterLabel))
	content := strings.Join(lines, "\n")

	return contentStyle.Width(width).Height(height).Render(
		header + "\n\n" + content,
	)
}

func (m Model) renderProgress(width, height int) string {
	lines := []string{
		lipgloss.NewStyle().Bold(true).Render("📈 Chapter Progress"),
		"",
	}

	// Overall progress bar
	lines = append(lines, m.renderProgressBar(m.totalProgress, width-4))
	lines = append(lines, "")

	// Chapter list
	visibleChapters := height - 8 // header + progress bar + spacing
	if visibleChapters < 1 {
		visibleChapters = 1
	}

	for i, chapterID := range m.chapterOrder {
		if i >= visibleChapters {
			break
		}

		state := m.chapters[chapterID]
		statusIcon := m.chapterStatusIcon(state.Status)
		statusStyle := m.chapterStatusStyle(state.Status)

		line := fmt.Sprintf("%s %s",
			statusIcon,
			statusStyle.Render(chapterID),
		)

		if state.Score != nil {
			line += fmt.Sprintf(" (%d)", *state.Score)
		}

		if state.DraftVersion > 0 {
			line += fmt.Sprintf(" v%d", state.DraftVersion)
		}

		if state.RevisionRounds > 0 {
			line += fmt.Sprintf(" [R%d]", state.RevisionRounds)
		}

		lines = append(lines, line)
	}

	// Citation consistency
	if m.citationScore > 0 {
		lines = append(lines, "")
		lines = append(lines, m.renderMetricLine("Citations", fmt.Sprintf("%d%%", m.citationScore)))
	}

	content := strings.Join(lines, "\n")
	content = truncateToHeight(content, height-2)

	return progressStyle.Width(width).Height(height).Render(content)
}

func (m Model) renderFooter() string {
	if m.done {
		if m.err != nil {
			return footerStyle.Render("❌ Failed. Press 'q' to exit.")
		}
		return footerStyle.Render("✅ Completed. Press 'q' to continue.")
	}

	if m.canceled {
		return footerStyle.Render("⏸️  Stopping at safe point...")
	}

	hints := []string{
		"Ctrl+C: Stop",
		"Space: Toggle auto-scroll",
		"↑/↓: Scroll logs",
	}

	return footerStyle.Render(strings.Join(hints, " • "))
}

// Helper rendering functions

func (m Model) renderMetricLine(label, value string) string {
	return fmt.Sprintf("%s %s",
		metricLabelStyle.Render(label+":"),
		metricValueStyle.Render(value),
	)
}

func (m Model) formatStatus() string {
	if m.done {
		if m.err != nil {
			return "❌ Failed"
		}
		return "✅ Done"
	}
	if m.canceled {
		return "⏸️  Stopping"
	}
	if m.running {
		return "🔄 Running"
	}
	return "⏳ Starting"
}

func (m Model) formatContext() string {
	if m.contextMax == 0 {
		return "--"
	}
	return fmt.Sprintf("%d/%d", m.contextUsed, m.contextMax)
}

func (m Model) formatTokens() string {
	if m.totalTokens == 0 {
		return "--"
	}
	return fmt.Sprintf("%d", m.totalTokens)
}

func (m Model) formatCost() string {
	if m.totalCost == 0.0 {
		return "--"
	}
	return fmt.Sprintf("$%.4f", m.totalCost)
}

func (m Model) formatElapsed() string {
	if m.elapsed == 0 {
		return "--"
	}
	return formatDuration(m.elapsed)
}

func (m Model) renderProgressBar(percent float64, width int) string {
	if width < 10 {
		width = 10
	}

	barWidth := width - 8 // "100.0% " prefix
	filled := int(percent / 100.0 * float64(barWidth))
	if filled > barWidth {
		filled = barWidth
	}

	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)

	return fmt.Sprintf("%.1f%% %s", percent, bar)
}

func (m Model) chapterStatusIcon(status ChapterStatus) string {
	switch status {
	case ChapterDone:
		return "✅"
	case ChapterWriting:
		return "✍️"
	case ChapterReviewing:
		return "🔍"
	case ChapterVerifying:
		return "🔬"
	case ChapterRewriting:
		return "♻️"
	case ChapterNeedsReview:
		return "⚠️"
	case ChapterNeedsRevision:
		return "📝"
	case ChapterPending:
		return "⏳"
	default:
		return "○"
	}
}

func (m Model) chapterStatusStyle(status ChapterStatus) lipgloss.Style {
	switch status {
	case ChapterDone:
		return chapterDoneStyle
	case ChapterWriting, ChapterReviewing, ChapterVerifying, ChapterRewriting:
		return chapterActiveStyle
	case ChapterNeedsReview, ChapterNeedsRevision:
		return chapterErrorStyle
	case ChapterPending:
		return chapterPendingStyle
	default:
		return lipgloss.NewStyle()
	}
}

func truncateToHeight(content string, maxLines int) string {
	lines := strings.Split(content, "\n")
	if len(lines) <= maxLines {
		return content
	}
	return strings.Join(lines[:maxLines], "\n")
}

func formatDuration(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60

	if h > 0 {
		return fmt.Sprintf("%dh%dm%ds", h, m, s)
	}
	if m > 0 {
		return fmt.Sprintf("%dm%ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}
