package referencestui

import (
	"strings"
	"testing"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/tui/ui"
)

// TestViewportScrollsCursorIntoView verifies the bounded view (BUG-20260620-01):
// when the candidate list is taller than the window, only a window of
// candidates is rendered and the cursor item is always visible.
func TestViewportScrollsCursorIntoView(t *testing.T) {
	items := make([]contracts.ReferenceCandidate, 0, 40)
	for i := 0; i < 40; i++ {
		items = append(items, contracts.ReferenceCandidate{
			ID:             idFor(i),
			Title:          titleFor(i),
			Authors:        []string{"Author"},
			Year:           2020 + i,
			RelevanceScore: 0.9,
			DOI:            "10.1000/" + idFor(i),
			Abstract:       abstractFor(i),
			Status:         "pending",
		})
	}
	m := NewModel(contracts.ReferenceCandidates{Items: items}, Options{MinConfirmed: 1})
	m = m.SetSize(80, 20) // small window: only a few candidates fit

	// Move cursor deep into the list.
	for i := 0; i < 35; i++ {
		m = m.UpdateKey("down")
	}

	view := m.View()
	lineCount := strings.Count(view, "\n")

	// A 20-row window cannot render all 40 candidates (each >=5 lines).
	if lineCount >= 20*6 {
		t.Fatalf("bounded view rendered too many lines (%d); viewport not applied:\n%s", lineCount, view)
	}
	// Cursor item must be visible.
	if !strings.Contains(view, titleFor(35)) {
		t.Fatalf("cursor item %q not visible in viewport:\n%s", titleFor(35), view)
	}
	// The very first item should be scrolled out of view.
	if strings.Contains(view, titleFor(0)) {
		t.Fatalf("first item should be scrolled out:\n%s", view)
	}

	// Scroll back to the top: first item visible again, last not.
	for i := 0; i < 35; i++ {
		m = m.UpdateKey("up")
	}
	view = m.View()
	if !strings.Contains(view, titleFor(0)) {
		t.Fatalf("first item should be visible at top:\n%s", view)
	}
}

// TestBoundedViewWrapsLongTitles ensures long titles wrap to the panel width
// instead of overflowing the window.
func TestBoundedViewWrapsLongTitles(t *testing.T) {
	longTitle := strings.Repeat("长标题测试", 30)
	m := NewModel(contracts.ReferenceCandidates{Items: []contracts.ReferenceCandidate{{
		ID:    "cand_long",
		Title: longTitle,
		Year:  2024,
		Status: "pending",
	}}}, Options{MinConfirmed: 1})
	m = m.SetSize(60, 30)

	view := m.View()
	for _, line := range strings.Split(view, "\n") {
		// Wrapped lines stay within the 60-cell window (allow ANSI overhead).
		if ui.StringWidth(line) > 70 {
			t.Fatalf("line exceeds window width (%d cells): %q", ui.StringWidth(line), line)
		}
	}
}

// TestUnboundedFallbackPreservesLegacyView ensures zero-size models still use
// the original "render everything" layout (regression guard for existing tests).
func TestUnboundedFallbackPreservesLegacyView(t *testing.T) {
	m := NewModel(sampleCandidates(), Options{MinConfirmed: 1})
	view := m.View()
	for _, want := range []string{"RAG for Review Writing", "Human in the Loop Review", "Older Survey"} {
		if !strings.Contains(view, want) {
			t.Fatalf("unbounded view missing %q:\n%s", want, view)
		}
	}
}

func idFor(i int) string    { return "cand_" + string(rune('a'+i%26)) + string(rune('0'+i/26)) }
func titleFor(i int) string { return "Paper Number " + idFor(i) }
func abstractFor(i int) string {
	return "Abstract content for candidate " + idFor(i) + " describing the study in detail."
}
