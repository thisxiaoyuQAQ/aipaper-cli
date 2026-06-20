package referencestui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/i18n"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/references"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/tui/ui"
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
	// No window size known yet (tests / zero-value): render the original
	// unrestricted layout so existing string assertions keep passing.
	if m.width <= 0 || m.height <= 0 {
		return m.viewUnbounded()
	}
	return m.viewBounded()
}

// viewUnbounded is the legacy "render every candidate" layout, used when the
// terminal dimensions are unknown.
func (m Model) viewUnbounded() string {
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
		m.writeCandidate(&panel, i, candidate)
		panel.WriteString("\n")
	}
	panel.WriteString(referencesHintStyle.Render(m.footerHint()))
	panel.WriteString("\n")

	b.WriteString(referencesPanelStyle.Render(panel.String()))
	b.WriteString("\n")
	return b.String()
}

// viewBounded renders the list fitted to the terminal: long lines wrap to the
// panel's inner width and only the candidates that fit the window height are
// shown, scrolling to keep the cursor item visible.
func (m Model) viewBounded() string {
	visible := m.visible()

	// Outer layout reserves room for the header (1) + blank (1) + info (1) +
	// blank (1) above the panel, and a trailing newline (1). The panel itself
	// carries a 1-line border top/bottom and (1,2) padding.
	outerOverhead := 6
	availableHeight := m.height - outerOverhead
	if availableHeight < 8 {
		availableHeight = 8
	}
	panelWidth := m.width
	if panelWidth > 120 {
		panelWidth = 120
	}
	// Inner content width: subtract border (2) + horizontal padding (2*2=4).
	innerWidth := panelWidth - 2 - 4
	if innerWidth < 10 {
		innerWidth = 10
	}

	var b strings.Builder
	header := fmt.Sprintf(m.i18n.Text(i18n.ReferencesHeader), len(visible), len(m.selected), len(m.rejected))
	b.WriteString(referencesTitleStyle.Render(ui.TruncateCells(header, m.width)))
	b.WriteString("\n")

	info := []string{referencesHintStyle.Render(m.sortLabel() + ": " + m.sortModeLabel())}
	if m.searching {
		info = append(info, referencesActiveStyle.Render(fmt.Sprintf("%s: %s", m.i18n.Text(i18n.ReferencesSearch), m.filter)))
	}
	b.WriteString(ui.TruncateCells(strings.Join(info, "  "), m.width))
	b.WriteString("\n\n")

	var panel strings.Builder
	if m.err != nil {
		fmt.Fprintf(&panel, "%s\n\n", referencesErrorStyle.Render(fmt.Sprintf("%s: %v", m.i18n.Text(i18n.CommonErrorPrefix), m.err)))
	}
	if len(visible) == 0 {
		panel.WriteString(referencesHintStyle.Render(m.i18n.Text(i18n.ReferencesNoMatches)))
		panel.WriteString("\n\n")
		panel.WriteString(referencesHintStyle.Render(m.footerHint()))
		panel.WriteString("\n")
		b.WriteString(m.renderPanel(panel.String(), panelWidth))
		b.WriteString("\n")
		return b.String()
	}

	// Render every candidate to wrapped line blocks and remember each block's
	// height. Footer is always pinned at the bottom of the viewport.
	blocks := make([][]string, len(visible))
	heights := make([]int, len(visible))
	for i, candidate := range visible {
		block := m.renderCandidateLines(i, candidate, innerWidth)
		blocks[i] = block
		heights[i] = len(block)
	}
	footer := referencesHintStyle.Render(m.footerHint())
	footerLines := ui.WrapCells(footer, innerWidth)
	footerHeight := len(footerLines)

	// Reserve room for a blank separator before the footer.
	listHeight := availableHeight - 2 - footerHeight
	if listHeight < 1 {
		listHeight = 1
	}

	start, end := viewportRange(heights, m.cursor, listHeight)
	for i := start; i < end; i++ {
		for _, line := range blocks[i] {
			panel.WriteString(line)
			panel.WriteString("\n")
		}
		panel.WriteString("\n")
	}
	// Fill remaining list rows so the footer stays anchored to the panel bottom.
	shown := 0
	for i := start; i < end; i++ {
		shown += heights[i] + 1 // block + blank separator
	}
	for shown < listHeight {
		panel.WriteString("\n")
		shown++
	}
	panel.WriteString("\n")
	for _, line := range footerLines {
		panel.WriteString(line)
		panel.WriteString("\n")
	}

	b.WriteString(m.renderPanel(panel.String(), panelWidth))
	b.WriteString("\n")
	return b.String()
}

// renderPanel renders panel content inside the bordered, padded panel. Content
// lines are pre-padded to innerWidth, so the panel auto-sizes to innerWidth +
// padding + border = panelWidth without relying on lipgloss Width semantics
// (which would double-count the border and overflow the window).
func (m Model) renderPanel(content string, panelWidth int) string {
	return referencesPanelStyle.Render(content)
}

// viewportRange returns [start, end) — the candidate indices to display so the
// cursor item is visible and the window fits listHeight lines. The cursor item
// is always included even when taller than the window.
func viewportRange(heights []int, cursor, listHeight int) (int, int) {
	n := len(heights)
	if n == 0 {
		return 0, 0
	}
	if cursor < 0 {
		cursor = 0
	}
	if cursor >= n {
		cursor = n - 1
	}
	start, end := cursor, cursor+1
	total := heights[cursor]
	// Expand downward (later candidates) first.
	for end < n && total+heights[end]+1 <= listHeight {
		total += heights[end] + 1
		end++
	}
	// Then expand upward (earlier candidates) to fill remaining space.
	for start-1 >= 0 && total+heights[start-1]+1 <= listHeight {
		start--
		total += heights[start] + 1
	}
	return start, end
}

// renderCandidateLines renders one candidate as wrapped display lines (without
// a trailing separator). indented continuation lines align under the title.
func (m Model) renderCandidateLines(i int, candidate contracts.ReferenceCandidate, innerWidth int) []string {
	active := i == m.cursor
	cursor := " "
	state := m.renderState(candidate)
	id := candidate.ID
	title := candidate.Title
	if active {
		cursor = referencesActiveStyle.Render("▶")
		id = referencesActiveStyle.Render(id)
		title = referencesActiveStyle.Render(title)
	}
	prefix := fmt.Sprintf("%s %s %s ", cursor, state, id)
	prefixWidth := ui.StringWidth(cursor) + ui.StringWidth(state) + ui.StringWidth(id) + 3
	titleWidth := innerWidth - prefixWidth
	if titleWidth < 4 {
		titleWidth = 4
	}

	lines := make([]string, 0, 8)
	titleLines := ui.WrapCells(title, titleWidth)
	for j, tl := range titleLines {
		if j == 0 {
			lines = append(lines, ui.PadCells(prefix+tl, innerWidth))
		} else {
			lines = append(lines, ui.PadCells(strings.Repeat(" ", prefixWidth)+tl, innerWidth))
		}
	}

	indent := strings.Repeat(" ", 4)
	if subtitle := candidateSubtitle(candidate); subtitle != "" {
		for _, wl := range ui.WrapCells(subtitle, innerWidth-4) {
			lines = append(lines, ui.PadCells(indent+referencesMetaStyle.Render(wl), innerWidth))
		}
	}
	if candidate.DOI != "" {
		line := fmt.Sprintf("%s: %s", m.i18n.Text(i18n.ReferencesDOI), referencesMetaStyle.Render(candidate.DOI))
		for _, wl := range ui.WrapCells(line, innerWidth-4) {
			lines = append(lines, ui.PadCells(indent+wl, innerWidth))
		}
	} else if candidate.URL != "" {
		line := fmt.Sprintf("%s: %s", m.i18n.Text(i18n.ReferencesURL), referencesMetaStyle.Render(candidate.URL))
		for _, wl := range ui.WrapCells(line, innerWidth-4) {
			lines = append(lines, ui.PadCells(indent+wl, innerWidth))
		}
	}
	if summary := references.CandidateSummary(candidate.Abstract); summary != "" {
		line := fmt.Sprintf("%s: %s", m.i18n.Text(i18n.ReferencesSummary), summary)
		for _, wl := range ui.WrapCells(line, innerWidth-4) {
			lines = append(lines, ui.PadCells(indent+wl, innerWidth))
		}
	}
	if candidate.RelevanceScore != 0 {
		line := fmt.Sprintf("%s: %s", m.i18n.Text(i18n.ReferencesRelevance), referencesMetaStyle.Render(fmt.Sprintf("%.2f", candidate.RelevanceScore)))
		lines = append(lines, ui.PadCells(indent+line, innerWidth))
	}
	if candidate.RelevanceReason != "" {
		line := fmt.Sprintf("%s: %s", m.i18n.Text(i18n.ReferencesReason), candidate.RelevanceReason)
		for _, wl := range ui.WrapCells(line, innerWidth-4) {
			lines = append(lines, ui.PadCells(indent+wl, innerWidth))
		}
	}
	return lines
}

// writeCandidate appends one candidate (legacy unbounded layout) to b.
func (m Model) writeCandidate(b *strings.Builder, i int, candidate contracts.ReferenceCandidate) {
	cursor := " "
	title := candidate.Title
	id := candidate.ID
	if i == m.cursor {
		cursor = referencesActiveStyle.Render("▶")
		title = referencesActiveStyle.Render(title)
		id = referencesActiveStyle.Render(id)
	}
	fmt.Fprintf(b, "%s %s %s %s\n", cursor, m.renderState(candidate), id, title)
	if subtitle := candidateSubtitle(candidate); subtitle != "" {
		fmt.Fprintf(b, "    %s\n", referencesMetaStyle.Render(subtitle))
	}
	if candidate.DOI != "" {
		fmt.Fprintf(b, "    %s: %s\n", m.i18n.Text(i18n.ReferencesDOI), referencesMetaStyle.Render(candidate.DOI))
	} else if candidate.URL != "" {
		fmt.Fprintf(b, "    %s: %s\n", m.i18n.Text(i18n.ReferencesURL), referencesMetaStyle.Render(candidate.URL))
	}
	if summary := references.CandidateSummary(candidate.Abstract); summary != "" {
		fmt.Fprintf(b, "    %s: %s\n", m.i18n.Text(i18n.ReferencesSummary), summary)
	}
	if candidate.RelevanceScore != 0 {
		fmt.Fprintf(b, "    %s: %s\n", m.i18n.Text(i18n.ReferencesRelevance), referencesMetaStyle.Render(fmt.Sprintf("%.2f", candidate.RelevanceScore)))
	}
	if candidate.RelevanceReason != "" {
		fmt.Fprintf(b, "    %s: %s\n", m.i18n.Text(i18n.ReferencesReason), candidate.RelevanceReason)
	}
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
