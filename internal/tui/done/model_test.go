package done

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/checkpoint"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/export"
)

// TestNewModel verifies model initialization with default and custom options.
func TestNewModel(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		m := NewModel(Options{})
		if m.quit {
			t.Error("expected quit=false initially")
		}
	})

	t.Run("custom work dir", func(t *testing.T) {
		m := NewModel(Options{WorkDir: "/tmp/test"})
		finalPath := m.store.Path("final")
		if !contains(finalPath, "aipaper") {
			t.Errorf("store.Path('final') = %q, want to contain 'aipaper'", finalPath)
		}
	})

	t.Run("with export result", func(t *testing.T) {
		result := export.Result{
			Version: "2024-01-01T00:00:00Z",
			Outputs: []checkpoint.OutputArtifact{
				{Kind: "markdown", Path: "final/paper.md", SHA256: "abc123"},
				{Kind: "markdown", Path: "final/report.md", SHA256: "def456"},
			},
			DocxWritten: true,
		}

		m := NewModel(Options{Result: result})
		if m.result.Version != "2024-01-01T00:00:00Z" {
			t.Errorf("result.Version = %q, want '2024-01-01T00:00:00Z'", m.result.Version)
		}
		if len(m.result.Outputs) != 2 {
			t.Errorf("result.Outputs length = %d, want 2", len(m.result.Outputs))
		}
	})
}

// TestInit verifies Init returns nil (no initial command).
func TestInit(t *testing.T) {
	m := NewModel(Options{})
	cmd := m.Init()
	if cmd != nil {
		t.Errorf("Init() = %v, want nil", cmd)
	}
}

// TestUpdateKey_QuitKeys verifies quit key handling.
func TestUpdateKey_QuitKeys(t *testing.T) {
	tests := []struct {
		name string
		key  string
	}{
		{"q key", "q"},
		{"ctrl+c", "ctrl+c"},
		{"enter key", "enter"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewModel(Options{})
			updated := m.UpdateKey(tt.key)

			if !updated.Quit() {
				t.Errorf("Quit() = false after %q, want true", tt.key)
			}
		})
	}
}

// TestUpdateKey_UnknownKey_NoChange verifies unknown keys don't affect state.
func TestUpdateKey_UnknownKey_NoChange(t *testing.T) {
	m := NewModel(Options{})
	updated := m.UpdateKey("x")

	if updated.Quit() {
		t.Error("Quit() = true after unknown key, want false")
	}
}

// TestUpdate_KeyMsg_DelegatesToUpdateKey verifies tea.KeyMsg handling.
func TestUpdate_KeyMsg_DelegatesToUpdateKey(t *testing.T) {
	m := NewModel(Options{})

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if cmd == nil {
		t.Fatal("Update() cmd = nil, want tea.Quit")
	}

	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("cmd() returned %T, want tea.QuitMsg", msg)
	}

	updatedModel := updated.(Model)
	if !updatedModel.Quit() {
		t.Error("Quit() = false after 'q' KeyMsg, want true")
	}
}

// TestUpdate_UnknownMsg_ReturnsUnchanged verifies unknown messages are ignored.
func TestUpdate_UnknownMsg_ReturnsUnchanged(t *testing.T) {
	m := NewModel(Options{})

	type unknownMsg struct{}
	updated, cmd := m.Update(unknownMsg{})
	if cmd != nil {
		t.Errorf("cmd = %v, want nil", cmd)
	}

	updatedModel := updated.(Model)
	if updatedModel.Quit() {
		t.Error("model should remain unchanged for unknown messages")
	}
}

// TestView_DisplaysOutputDirectory verifies output directory rendering.
func TestView_DisplaysOutputDirectory(t *testing.T) {
	m := NewModel(Options{WorkDir: "/tmp/test"})
	view := m.View()

	if !contains(view, "输出目录") {
		t.Errorf("View() = %q, want to contain '输出目录'", view)
	}
	if !contains(view, "final") {
		t.Errorf("View() = %q, want to contain 'final'", view)
	}
}

// TestView_DisplaysOutputFiles verifies file list rendering.
func TestView_DisplaysOutputFiles(t *testing.T) {
	result := export.Result{
		Version: "2024-01-01T00:00:00Z",
		Outputs: []checkpoint.OutputArtifact{
			{Kind: "markdown", Path: "final/paper.md", SHA256: "abc123"},
			{Kind: "markdown", Path: "final/report.md", SHA256: "def456"},
			{Kind: "markdown", Path: "final/references.md", SHA256: "ghi789"},
		},
		DocxWritten: true,
	}

	m := NewModel(Options{Result: result})
	view := m.View()

	if !contains(view, "生成文件") {
		t.Errorf("View() = %q, want to contain '生成文件'", view)
	}
	if !contains(view, "final/paper.md") {
		t.Errorf("View() = %q, want to contain 'final/paper.md'", view)
	}
	if !contains(view, "final/report.md") {
		t.Errorf("View() = %q, want to contain 'final/report.md'", view)
	}
	if !contains(view, "final/references.md") {
		t.Errorf("View() = %q, want to contain 'final/references.md'", view)
	}
}

// TestView_EmptyOutputs verifies rendering when no outputs.
func TestView_EmptyOutputs(t *testing.T) {
	result := export.Result{
		Version: "2024-01-01T00:00:00Z",
		Outputs: []checkpoint.OutputArtifact{},
	}

	m := NewModel(Options{Result: result})
	view := m.View()

	if !contains(view, "(无)") {
		t.Errorf("View() = %q, want to contain '(无)' for empty outputs", view)
	}
}

// TestView_DisplaysNextStepsHints verifies next steps section.
func TestView_DisplaysNextStepsHints(t *testing.T) {
	m := NewModel(Options{})
	view := m.View()

	if !contains(view, "下一步") {
		t.Errorf("View() = %q, want to contain '下一步'", view)
	}
	if !contains(view, "recover") {
		t.Errorf("View() = %q, want to contain 'recover' hint", view)
	}
	if !contains(view, "status") {
		t.Errorf("View() = %q, want to contain 'status' hint", view)
	}
	if !contains(view, "config") {
		t.Errorf("View() = %q, want to contain 'config' hint", view)
	}
}

// TestView_DisplaysExitKeys verifies exit key hints.
func TestView_DisplaysExitKeys(t *testing.T) {
	m := NewModel(Options{})
	view := m.View()

	if !contains(view, "[enter]") || !contains(view, "[q]") || !contains(view, "[ctrl+c]") {
		t.Errorf("View() = %q, want to contain exit key hints", view)
	}
	if !contains(view, "退出") {
		t.Errorf("View() = %q, want to contain '退出'", view)
	}
}

// TestView_PathConsistency verifies paths match Result.Outputs.
func TestView_PathConsistency(t *testing.T) {
	// This test ensures that file paths displayed in Done screen
	// match exactly what was produced by ExportFinal (via Result.Outputs).
	result := export.Result{
		Version: "2024-01-01T00:00:00Z",
		Outputs: []checkpoint.OutputArtifact{
			{Kind: "markdown", Path: "final/paper.md", SHA256: "abc"},
			{Kind: "docx", Path: "final/paper.docx", SHA256: "def"},
			{Kind: "markdown", Path: "final/references.md", SHA256: "ghi"},
			{Kind: "json", Path: "final/citation-trace.json", SHA256: "jkl"},
			{Kind: "markdown", Path: "final/report.md", SHA256: "mno"},
		},
		DocxWritten: true,
	}

	m := NewModel(Options{WorkDir: "/tmp/test", Result: result})
	view := m.View()

	// Verify each output path appears in the view
	for _, out := range result.Outputs {
		if !contains(view, out.Path) {
			t.Errorf("View() missing output path %q", out.Path)
		}
	}

	// Verify output directory derivation
	if !contains(view, m.store.Path("final")) {
		t.Errorf("View() missing output directory path")
	}
}

// TestView_SuccessIndicator verifies completion indicator.
func TestView_SuccessIndicator(t *testing.T) {
	m := NewModel(Options{})
	view := m.View()

	if !contains(view, "✓") || !contains(view, "完成") {
		t.Errorf("View() = %q, want to show success indicator", view)
	}
}

// Helper function for substring checking
func contains(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr)
}

func findSubstring(s, substr string) bool {
	if s == substr {
		return true
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
