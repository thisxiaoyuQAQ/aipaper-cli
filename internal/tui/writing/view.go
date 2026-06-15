package writing

import (
	"fmt"
	"strings"
	"time"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/i18n"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
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

	inputStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("63")).
			Padding(0, 1)

	// Footer
	footerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Italic(true)
)

type viewportSpec struct {
	pane    paneID
	title   string
	style   lipgloss.Style
	width   int
	height  int
	offset  int
	focused bool
}

// View renders the WritingProgress screen.
func (m Model) View() string {
	if m.width < 60 {
		return m.renderNarrowLayout()
	}
	return m.renderWideLayout()
}

func (m Model) renderWideLayout() string {
	inputHeight := 3
	footerHeight := 2
	mainHeight := max(8, m.height-inputHeight-footerHeight)

	leftWidth := clamp(m.width/4, 20, 30)
	rightWidth := clamp(m.width/4, 22, 35)
	middleWidth := m.width - leftWidth - rightWidth
	if middleWidth < 18 {
		middleWidth = max(18, m.width-leftWidth-rightWidth)
	}

	topHeight := mainHeight / 2
	bottomHeight := mainHeight - topHeight
	if topHeight < 4 {
		topHeight = 4
	}
	if bottomHeight < 4 {
		bottomHeight = 4
	}

	leftPanel := m.renderMetrics(leftWidth, mainHeight)
	topMiddlePanel := m.renderLogs(middleWidth, topHeight)
	bottomMiddlePanel := m.renderContent(middleWidth, bottomHeight)
	rightPanel := m.renderProgress(rightWidth, mainHeight)

	middlePanel := lipgloss.JoinVertical(lipgloss.Left, topMiddlePanel, bottomMiddlePanel)
	mainView := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, middlePanel, rightPanel)
	return lipgloss.JoinVertical(lipgloss.Left, mainView, m.renderInput(), m.renderFooter())
}

func (m Model) renderNarrowLayout() string {
	width := max(20, m.width)
	footerHeight := 2
	inputHeight := 3
	sectionHeight := max(5, (m.height-footerHeight-inputHeight)/4)
	sections := []string{
		m.renderMetrics(width, sectionHeight),
		m.renderLogs(width, sectionHeight),
		m.renderContent(width, sectionHeight),
		m.renderProgress(width, sectionHeight),
		m.renderInput(),
		m.renderFooter(),
	}
	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func (m Model) renderMetrics(width, height int) string {
	lines := []string{
		m.renderMetricLine(m.i18n.Text(i18n.WritingStatusLabel), m.formatStatus()),
		m.renderMetricLine(m.i18n.Text(i18n.WritingPhaseLabel), m.phase),
		m.renderMetricLine(m.i18n.Text(i18n.WritingProgressLabel), fmt.Sprintf("%.1f%%", m.totalProgress)),
		m.renderMetricLine(m.i18n.Text(i18n.WritingFocusLabel), m.focusedPane.String()),
		m.renderMetricLine(m.i18n.Text(i18n.WritingPendingLabel), fmt.Sprintf("%d", m.pendingInstructions)),
		"",
		m.renderMetricLine(m.i18n.Text(i18n.WritingWordsLabel), fmt.Sprintf("%d", m.wordCount)),
		m.renderMetricLine(m.i18n.Text(i18n.WritingModelLabel), m.model),
		"",
		m.renderMetricLine(m.i18n.Text(i18n.WritingContextLabel), m.formatContext()),
		m.renderMetricLine(m.i18n.Text(i18n.WritingTokensLabel), m.formatTokens()),
		m.renderMetricLine(m.i18n.Text(i18n.WritingCostLabel), m.formatCost()),
		m.renderMetricLine(m.i18n.Text(i18n.WritingElapsedLabel), m.formatElapsed()),
	}
	return m.renderViewport(viewportSpec{pane: paneMetrics, title: m.i18n.Text(i18n.WritingMetricsTitle), style: metricsStyle, width: width, height: height, offset: m.metricsScrollPos, focused: m.focusedPane == paneMetrics}, lines)
}

func (m Model) renderLogs(width, height int) string {
	lines := make([]string, 0, len(m.logs))
	for _, entry := range m.logs {
		timestamp := entry.at.Format("15:04:05")
		role := entry.role
		if role == "" {
			role = "system"
		}
		line := fmt.Sprintf("[%s] %s: %s", timestamp, role, entry.message)
		if entry.isError {
			line = "ERROR " + line
		}
		lines = append(lines, line)
	}

	offset := m.logScrollPos
	if m.autoScroll {
		visible := max(1, height-4)
		if len(lines) > visible {
			offset = len(lines) - visible
		}
	}
	scrollIndicator := ""
	if !m.autoScroll {
		scrollIndicator = m.i18n.Text(i18n.WritingManualScroll)
	}
	return m.renderViewport(viewportSpec{pane: paneLogs, title: m.i18n.Text(i18n.WritingLogsTitle) + scrollIndicator, style: logStyle, width: width, height: height, offset: offset, focused: m.focusedPane == paneLogs}, lines)
}

func (m Model) renderContent(width, height int) string {
	chapterLabel := m.currentChapterID
	if chapterLabel == "" {
		chapterLabel = m.i18n.Text(i18n.WritingWaiting)
	}
	return m.renderViewport(viewportSpec{pane: paneContent, title: fmt.Sprintf(m.i18n.Text(i18n.WritingContentTitle), chapterLabel), style: contentStyle, width: width, height: height, offset: m.contentScrollPos, focused: m.focusedPane == paneContent}, m.contentLines)
}

func (m Model) renderProgress(width, height int) string {
	lines := []string{
		m.renderProgressBar(m.totalProgress, max(10, width-6)),
		"",
	}
	for _, chapterID := range m.chapterOrder {
		state := m.chapters[chapterID]
		statusIcon := m.chapterStatusIcon(state.Status)
		line := fmt.Sprintf("%s %s", statusIcon, chapterID)
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
	if m.citationScore > 0 {
		lines = append(lines, "", m.renderMetricLine(m.i18n.Text(i18n.WritingCitationsLabel), fmt.Sprintf("%d%%", m.citationScore)))
	}
	return m.renderViewport(viewportSpec{pane: paneProgress, title: m.i18n.Text(i18n.WritingChapterProgress), style: progressStyle, width: width, height: height, offset: m.progressScrollPos, focused: m.focusedPane == paneProgress}, lines)
}

func (m Model) renderViewport(spec viewportSpec, rawLines []string) string {
	width := max(10, spec.width)
	height := max(3, spec.height)
	innerWidth := max(1, width-4)
	contentWidth := max(1, innerWidth-2)
	visibleLines := max(1, height-4)

	lines := fitLines(rawLines, contentWidth)
	offset := clamp(spec.offset, 0, max(0, len(lines)-visibleLines))
	visible := make([]string, 0, visibleLines)
	for i := 0; i < visibleLines; i++ {
		idx := offset + i
		line := ""
		if idx < len(lines) {
			line = truncateCells(lines[idx], contentWidth)
		}
		bar := scrollbarCell(len(lines), visibleLines, offset, i)
		visible = append(visible, padCells(line, contentWidth)+" "+bar)
	}

	title := spec.title
	if spec.focused {
		title = "▶ " + title
	}
	body := lipgloss.NewStyle().Bold(true).Render(truncateCells(title, innerWidth)) + "\n" + strings.Join(visible, "\n")
	style := spec.style.Width(width).Height(height)
	if spec.focused {
		style = style.BorderForeground(lipgloss.Color("220"))
	}
	return style.Render(body)
}

func fitLines(lines []string, width int) []string {
	if len(lines) == 0 {
		return []string{}
	}
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if line == "" {
			out = append(out, "")
			continue
		}
		wrapped := ansi.Wrap(line, width, " ")
		if wrapped == "" {
			out = append(out, "")
			continue
		}
		for _, part := range strings.Split(wrapped, "\n") {
			out = append(out, strings.TrimLeft(part, " "))
		}
	}
	return out
}

func scrollbarCell(total, visible, offset, row int) string {
	if total <= visible || visible <= 0 {
		return " "
	}
	thumbSize := max(1, visible*visible/total)
	maxThumbStart := visible - thumbSize
	maxOffset := max(1, total-visible)
	thumbStart := offset * maxThumbStart / maxOffset
	if row >= thumbStart && row < thumbStart+thumbSize {
		return "█"
	}
	return "│"
}

func truncateCells(s string, width int) string {
	if width <= 0 {
		return ""
	}
	return ansi.Truncate(s, width, "")
}

func padCells(s string, width int) string {
	w := ansi.StringWidth(s)
	if w >= width {
		return truncateCells(s, width)
	}
	return s + strings.Repeat(" ", width-w)
}

func (m Model) renderInput() string {
	width := max(20, m.width)
	label := m.i18n.Text(i18n.WritingInputLabel)
	value := m.input
	if value == "" {
		if m.paused {
			value = m.i18n.Text(i18n.WritingInputPaused)
		} else {
			value = m.i18n.Text(i18n.WritingInputRunning)
		}
	}
	line := truncateCells(label+value, max(1, width-4))
	return inputStyle.Width(width).Height(3).Render(line)
}

func (m Model) renderFooter() string {
	if m.done {
		if m.err != nil {
			return footerStyle.Render(m.i18n.Text(i18n.WritingFooterFailed))
		}
		return footerStyle.Render(m.i18n.Text(i18n.WritingFooterCompleted))
	}
	if m.paused {
		return footerStyle.Render(m.i18n.Text(i18n.WritingFooterPaused))
	}
	if m.pauseRequested {
		return footerStyle.Render(m.i18n.Text(i18n.WritingFooterPausing))
	}
	if m.canceled {
		return footerStyle.Render(m.i18n.Text(i18n.WritingFooterStopped))
	}

	hints := []string{
		m.i18n.Text(i18n.WritingHintPause),
		m.i18n.Text(i18n.WritingHintSubmit),
		m.i18n.Text(i18n.WritingHintExit),
		m.i18n.Text(i18n.WritingHintFocus),
		m.i18n.Text(i18n.WritingHintScroll),
		m.i18n.Text(i18n.WritingHintAutoScroll),
	}
	footer := strings.Join(hints, " • ")
	if hint := m.staleHint(); hint != "" {
		return hint + "\n" + footerStyle.Render(footer)
	}
	return footerStyle.Render(footer)
}

// staleHint returns a one-line warning when the screen has not received any
// runtime event for longer than staleThreshold while still running. It is the
// TUI backstop for cases the heartbeat watchdog does not cover (channel full
// drop, events filtered out). Empty when not stale, done, or canceled.
func (m Model) staleHint() string {
	if !m.running || m.done || m.canceled || m.paused {
		return ""
	}
	if m.lastActivity.IsZero() {
		return ""
	}
	since := time.Since(m.lastActivity)
	if since < staleThreshold {
		return ""
	}
	return logErrorStyle.Render(fmt.Sprintf(
		m.i18n.Text(i18n.WritingStaleHint),
		formatDuration(since.Round(time.Second)),
	))
}

// Helper rendering functions

func (m Model) renderMetricLine(label, value string) string {
	return fmt.Sprintf("%s %s", metricLabelStyle.Render(label+":"), metricValueStyle.Render(value))
}

func (m Model) formatStatus() string {
	if m.done {
		if m.err != nil {
			return m.i18n.Text(i18n.WritingStatusFailed)
		}
		return m.i18n.Text(i18n.WritingStatusDone)
	}
	if m.paused {
		return m.i18n.Text(i18n.WritingStatusPaused)
	}
	if m.pauseRequested {
		return m.i18n.Text(i18n.WritingStatusPausing)
	}
	if m.canceled {
		return m.i18n.Text(i18n.WritingStatusStopped)
	}
	if m.running {
		return m.i18n.Text(i18n.WritingStatusRunning)
	}
	return m.i18n.Text(i18n.WritingStatusStarting)
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
	barWidth := width - 8
	filled := int(percent / 100.0 * float64(barWidth))
	if filled > barWidth {
		filled = barWidth
	}
	if filled < 0 {
		filled = 0
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

func clamp(v, min, maxValue int) int {
	if v < min {
		return min
	}
	if v > maxValue {
		return maxValue
	}
	return v
}
