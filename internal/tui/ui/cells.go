// Package ui provides shared cell-width rendering helpers for TUI screens:
// wrapping, truncation, padding and viewport windowing that respect the
// terminal's actual dimensions. These mirror the helpers that the writing
// screen already uses internally (fitLines/truncateCells/padCells) but are
// shared so list-style screens (references, materials, search, export, done)
// can wrap long lines and fit their borders to the window without overflowing.
package ui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// StringWidth reports the display width of s in terminal cells, ignoring
// ANSI escape sequences. Returns 0 for empty input.
func StringWidth(s string) int {
	return ansi.StringWidth(s)
}

// TruncateCells truncates s to at most width display cells. Text beyond the
// width is dropped (no ellipsis) to keep column alignment predictable inside
// bordered panels. Returns "" for width <= 0.
func TruncateCells(s string, width int) string {
	if width <= 0 {
		return ""
	}
	return ansi.Truncate(s, width, "")
}

// PadCells pads s on the right with spaces so its display width equals width.
// If s is already wider than width it is truncated to width.
func PadCells(s string, width int) string {
	w := StringWidth(s)
	if w >= width {
		return TruncateCells(s, width)
	}
	return s + strings.Repeat(" ", width-w)
}

// WrapCells breaks s into a slice of lines, each no wider than width cells.
// Words longer than width are hard-split. Empty input yields a single empty
// line so callers can treat the result uniformly. width <= 0 yields [""].
func WrapCells(s string, width int) []string {
	if width <= 0 {
		return []string{""}
	}
	wrapped := ansi.Wrap(s, width, "")
	if wrapped == "" {
		return []string{""}
	}
	parts := strings.Split(wrapped, "\n")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		out = append(out, strings.TrimLeft(p, " "))
	}
	return out
}

// ClampOffset clamps an offset into [0, max(0, total-visible)] so a viewport
// never scrolls past its content. total is the number of content lines and
// visible is the viewport height.
func ClampOffset(offset, total, visible int) int {
	if total <= visible {
		return 0
	}
	maxOffset := total - visible
	if offset < 0 {
		return 0
	}
	if offset > maxOffset {
		return maxOffset
	}
	return offset
}

// FollowCursor returns the smallest offset that keeps item at cursor visible
// within a viewport of visibleHeight lines over a list of totalItems entries
// that each occupy heights[i] lines. It only scrolls when the cursor leaves
// the current window, so short lists stay pinned at the top.
func FollowCursor(cursor, totalItems, visibleHeight int, heights []int, currentOffset int) int {
	if totalItems <= 0 || visibleHeight <= 0 {
		return 0
	}
	if cursor < 0 {
		cursor = 0
	}
	if cursor >= totalItems {
		cursor = totalItems - 1
	}
	// Compute start line of the cursor item and its end line under the
	// current offset.
	start := 0
	for i := 0; i < cursor && i < len(heights); i++ {
		start += heights[i]
	}
	h := 1
	if cursor < len(heights) {
		h = heights[cursor]
	}
	end := start + h

	// Cursor above the window: scroll up to put it at the top.
	if start < currentOffset {
		return start
	}
	// Cursor below the window: scroll down until its last line fits.
	if end > currentOffset+visibleHeight {
		return end - visibleHeight
	}
	return currentOffset
}

// VisibleWindow returns the content lines [offset, offset+visible) of lines,
// clamped to bounds. The returned slice always has exactly visible entries;
// out-of-range slots are empty strings so callers can render a fixed-height
// viewport without bounds checks.
func VisibleWindow(lines []string, offset, visible int) []string {
	offset = ClampOffset(offset, len(lines), visible)
	out := make([]string, visible)
	for i := 0; i < visible; i++ {
		idx := offset + i
		if idx >= 0 && idx < len(lines) {
			out[i] = lines[idx]
		}
	}
	return out
}
