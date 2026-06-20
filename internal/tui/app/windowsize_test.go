package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/store"
)

// TestRootModelWindowSizeMsgInjectsSizeToReferences (BUG-20260620-01) ensures a
// tea.WindowSizeMsg is forwarded to the active References screen so the view
// can wrap and scroll to fit the window.
func TestRootModelWindowSizeMsgInjectsSizeToReferences(t *testing.T) {
	workDir := t.TempDir()
	s := store.New(workDir)
	if err := store.EnsureLayout(s); err != nil {
		t.Fatalf("EnsureLayout failed: %v", err)
	}
	candidates := contracts.ReferenceCandidates{Items: []contracts.ReferenceCandidate{{
		ID:     "cand_001",
		Title:  "Some Paper",
		Source: "bibtex",
		Status: "pending",
	}}}
	if _, err := store.WriteJSON(s.Path("references", "candidates.json"), candidates, store.Overwrite); err != nil {
		t.Fatalf("write candidates: %v", err)
	}

	m := NewRootModel(RootOptions{WorkDir: workDir, InitialScreen: ScreenReferences})
	if m.CurrentScreen != ScreenReferences {
		t.Fatalf("expected references screen, got %q", m.CurrentScreen)
	}
	if m.References.Width() != 0 || m.References.Height() != 0 {
		t.Fatalf("references should start without size, got %dx%d", m.References.Width(), m.References.Height())
	}

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	m = updated.(RootModel)

	if m.width != 100 || m.height != 40 {
		t.Fatalf("root size = %dx%d, want 100x40", m.width, m.height)
	}
	if m.References.Width() != 100 || m.References.Height() != 40 {
		t.Fatalf("references size = %dx%d, want 100x40", m.References.Width(), m.References.Height())
	}
}
