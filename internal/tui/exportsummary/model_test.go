package exportsummary

import (
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/checkpoint"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/export"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/store"
)

// TestNewModel verifies model initialization with default and custom options.
func TestNewModel(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		m := NewModel(Options{})
		if m.workDir != "." {
			t.Errorf("workDir = %q, want '.'", m.workDir)
		}
		if m.exported {
			t.Error("expected exported=false initially")
		}
		if m.done {
			t.Error("expected done=false initially")
		}
		if m.canceled {
			t.Error("expected canceled=false initially")
		}
	})

	t.Run("custom work dir", func(t *testing.T) {
		m := NewModel(Options{WorkDir: "/tmp/test"})
		if m.workDir != "/tmp/test" {
			t.Errorf("workDir = %q, want '/tmp/test'", m.workDir)
		}
	})

	t.Run("custom export function", func(t *testing.T) {
		called := false
		customExport := func(s store.Store) (export.Result, error) {
			called = true
			return export.Result{}, nil
		}
		m := NewModel(Options{Export: customExport})
		m.export(store.New("."))
		if !called {
			t.Error("custom export function was not used")
		}
	})
}

// TestInit_TriggersExport verifies Init returns export command.
func TestInit_TriggersExport(t *testing.T) {
	mockExport := func(s store.Store) (export.Result, error) {
		return export.Result{
			Version: "2024-01-01T00:00:00Z",
			Outputs: []checkpoint.OutputArtifact{
				{Kind: "markdown", Path: "final/paper.md", SHA256: "abc123"},
			},
			DocxWritten: true,
		}, nil
	}

	m := NewModel(Options{Export: mockExport})
	cmd := m.Init()
	if cmd == nil {
		t.Fatal("Init() returned nil, want export command")
	}

	msg := cmd()
	doneMsg, ok := msg.(exportDoneMsg)
	if !ok {
		t.Fatalf("cmd() returned %T, want exportDoneMsg", msg)
	}
	if doneMsg.err != nil {
		t.Errorf("exportDoneMsg.err = %v, want nil", doneMsg.err)
	}
	if doneMsg.result.Version != "2024-01-01T00:00:00Z" {
		t.Errorf("exportDoneMsg.result.Version = %q, want '2024-01-01T00:00:00Z'", doneMsg.result.Version)
	}
}

// TestUpdate_ExportSuccess verifies successful export message handling.
func TestUpdate_ExportSuccess(t *testing.T) {
	m := NewModel(Options{})

	result := export.Result{
		Version: "2024-01-01T00:00:00Z",
		Outputs: []checkpoint.OutputArtifact{
			{Kind: "markdown", Path: "final/paper.md", SHA256: "abc123"},
			{Kind: "markdown", Path: "final/report.md", SHA256: "def456"},
		},
		DocxWritten: true,
	}

	updated, cmd := m.Update(exportDoneMsg{result: result, err: nil})
	if cmd != nil {
		t.Errorf("Update() cmd = %v, want nil", cmd)
	}

	updatedModel, ok := updated.(Model)
	if !ok {
		t.Fatalf("updated type = %T, want Model", updated)
	}

	if !updatedModel.Exported() {
		t.Error("expected Exported()=true after exportDoneMsg")
	}
	if updatedModel.Err() != nil {
		t.Errorf("Err() = %v, want nil", updatedModel.Err())
	}
	if updatedModel.Result().Version != "2024-01-01T00:00:00Z" {
		t.Errorf("Result().Version = %q, want '2024-01-01T00:00:00Z'", updatedModel.Result().Version)
	}
	if len(updatedModel.Result().Outputs) != 2 {
		t.Errorf("Result().Outputs length = %d, want 2", len(updatedModel.Result().Outputs))
	}
}

// TestUpdate_ExportError_UnconfirmedReference verifies error handling for unconfirmed references.
func TestUpdate_ExportError_UnconfirmedReference(t *testing.T) {
	m := NewModel(Options{})

	exportErr := export.Error{
		Code:    export.CodeUnconfirmedReference,
		Message: "reference 'Smith2024' is not confirmed",
	}

	updated, cmd := m.Update(exportDoneMsg{err: exportErr})
	if cmd != nil {
		t.Errorf("Update() cmd = %v, want nil", cmd)
	}

	updatedModel := updated.(Model)
	if !updatedModel.Exported() {
		t.Error("expected Exported()=true even on error")
	}
	if updatedModel.Err() == nil {
		t.Fatal("Err() = nil, want error")
	}
	if updatedModel.Err().Error() != exportErr.Error() {
		t.Errorf("Err() = %v, want %v", updatedModel.Err(), exportErr)
	}
	if updatedModel.Done() {
		t.Error("expected Done()=false when export failed")
	}
}

// TestUpdate_ExportError_NoAcceptedChapters verifies error handling for no accepted chapters.
func TestUpdate_ExportError_NoAcceptedChapters(t *testing.T) {
	m := NewModel(Options{})

	exportErr := export.Error{
		Code:    export.CodeNoAcceptedChapters,
		Message: "no accepted chapters available for export",
	}

	updated, _ := m.Update(exportDoneMsg{err: exportErr})
	updatedModel := updated.(Model)

	if !updatedModel.Exported() {
		t.Error("expected Exported()=true even on error")
	}
	if updatedModel.Err() == nil {
		t.Fatal("Err() = nil, want error")
	}
	if updatedModel.Done() {
		t.Error("expected Done()=false when export failed")
	}
}

// TestUpdate_DocxDegradation verifies docx exporter failure results in degraded output.
func TestUpdate_DocxDegradation(t *testing.T) {
	m := NewModel(Options{})

	result := export.Result{
		Version: "2024-01-01T00:00:00Z",
		Outputs: []checkpoint.OutputArtifact{
			{Kind: "markdown", Path: "final/paper.md", SHA256: "abc123"},
			{Kind: "markdown", Path: "final/report.md", SHA256: "def456"},
		},
		Issues: []export.Issue{
			{
				Code:    export.CodeDocxFailed,
				Message: "docx conversion failed",
			},
		},
		DocxWritten: false,
	}

	updated, _ := m.Update(exportDoneMsg{result: result, err: nil})
	updatedModel := updated.(Model)

	if !updatedModel.Exported() {
		t.Error("expected Exported()=true")
	}
	if updatedModel.Err() != nil {
		t.Errorf("Err() = %v, want nil (docx failure is non-fatal)", updatedModel.Err())
	}
	if updatedModel.Result().DocxWritten {
		t.Error("expected DocxWritten=false")
	}
	if len(updatedModel.Result().Issues) != 1 {
		t.Fatalf("Issues length = %d, want 1", len(updatedModel.Result().Issues))
	}
	if updatedModel.Result().Issues[0].Code != export.CodeDocxFailed {
		t.Errorf("Issue code = %q, want %q", updatedModel.Result().Issues[0].Code, export.CodeDocxFailed)
	}
	// Markdown files should still be present
	if len(updatedModel.Result().Outputs) != 2 {
		t.Errorf("Outputs length = %d, want 2 (markdown files)", len(updatedModel.Result().Outputs))
	}
}

// TestUpdateKey_Retry verifies retry resets state and triggers new export.
func TestUpdateKey_Retry(t *testing.T) {
	m := NewModel(Options{
		Export: func(s store.Store) (export.Result, error) {
			return export.Result{Version: "v1"}, nil
		},
	})

	// Simulate failed export
	m.exported = true
	m.err = errors.New("previous error")

	updated, cmd := m.UpdateKey("r")
	if cmd == nil {
		t.Fatal("UpdateKey('r') returned nil cmd, want export command")
	}

	if updated.exported {
		t.Error("expected exported=false after retry")
	}
	if updated.err != nil {
		t.Errorf("err = %v, want nil after retry", updated.err)
	}

	// Execute the cmd to verify it triggers export
	msg := cmd()
	doneMsg, ok := msg.(exportDoneMsg)
	if !ok {
		t.Fatalf("cmd() returned %T, want exportDoneMsg", msg)
	}
	if doneMsg.result.Version != "v1" {
		t.Errorf("retry export result version = %q, want 'v1'", doneMsg.result.Version)
	}
}

// TestUpdateKey_Cancel verifies cancel sets canceled flag.
func TestUpdateKey_Cancel(t *testing.T) {
	m := NewModel(Options{})

	t.Run("b key", func(t *testing.T) {
		updated, cmd := m.UpdateKey("b")
		if cmd != nil {
			t.Errorf("UpdateKey('b') cmd = %v, want nil", cmd)
		}
		if !updated.Canceled() {
			t.Error("expected Canceled()=true after 'b'")
		}
	})

	t.Run("esc key", func(t *testing.T) {
		updated, cmd := m.UpdateKey("esc")
		if cmd != nil {
			t.Errorf("UpdateKey('esc') cmd = %v, want nil", cmd)
		}
		if !updated.Canceled() {
			t.Error("expected Canceled()=true after 'esc'")
		}
	})
}

// TestUpdateKey_Done verifies enter key sets done flag only after successful export.
func TestUpdateKey_Done(t *testing.T) {
	t.Run("enter after successful export", func(t *testing.T) {
		m := NewModel(Options{})
		m.exported = true
		m.err = nil

		updated, cmd := m.UpdateKey("enter")
		if cmd != nil {
			t.Errorf("UpdateKey('enter') cmd = %v, want nil", cmd)
		}
		if !updated.Done() {
			t.Error("expected Done()=true after enter with successful export")
		}
	})

	t.Run("enter before export completes", func(t *testing.T) {
		m := NewModel(Options{})
		m.exported = false

		updated, _ := m.UpdateKey("enter")
		if updated.Done() {
			t.Error("expected Done()=false when export not yet complete")
		}
	})

	t.Run("enter after export error", func(t *testing.T) {
		m := NewModel(Options{})
		m.exported = true
		m.err = errors.New("export failed")

		updated, _ := m.UpdateKey("enter")
		if updated.Done() {
			t.Error("expected Done()=false when export failed")
		}
	})
}

// TestApplyExportResult_SynchronousUpdate verifies synchronous result application.
func TestApplyExportResult_SynchronousUpdate(t *testing.T) {
	m := NewModel(Options{})

	result := export.Result{
		Version: "2024-01-01T00:00:00Z",
		Outputs: []checkpoint.OutputArtifact{
			{Kind: "markdown", Path: "final/paper.md", SHA256: "abc123"},
		},
		DocxWritten: true,
	}

	updated := m.ApplyExportResult(result, nil)

	if !updated.Exported() {
		t.Error("expected Exported()=true")
	}
	if updated.Err() != nil {
		t.Errorf("Err() = %v, want nil", updated.Err())
	}
	if updated.Result().Version != "2024-01-01T00:00:00Z" {
		t.Errorf("Result().Version = %q, want '2024-01-01T00:00:00Z'", updated.Result().Version)
	}
}

// TestRun_SynchronousExecution verifies Run method executes export synchronously.
func TestRun_SynchronousExecution(t *testing.T) {
	result := export.Result{
		Version: "v2",
		Outputs: []checkpoint.OutputArtifact{
			{Kind: "markdown", Path: "final/paper.md", SHA256: "xyz789"},
		},
		DocxWritten: false,
	}

	m := NewModel(Options{
		Export: func(s store.Store) (export.Result, error) {
			return result, nil
		},
	})

	updated := m.Run()

	if !updated.Exported() {
		t.Error("expected Exported()=true after Run()")
	}
	if updated.Err() != nil {
		t.Errorf("Err() = %v, want nil", updated.Err())
	}
	if updated.Result().Version != "v2" {
		t.Errorf("Result().Version = %q, want 'v2'", updated.Result().Version)
	}
	if updated.Result().DocxWritten {
		t.Error("expected DocxWritten=false")
	}
}

// TestRun_WithError verifies Run handles export errors.
func TestRun_WithError(t *testing.T) {
	exportErr := export.Error{
		Code:    export.CodeUnconfirmedReference,
		Message: "missing refs",
	}

	m := NewModel(Options{
		Export: func(s store.Store) (export.Result, error) {
			return export.Result{}, exportErr
		},
	})

	updated := m.Run()

	if !updated.Exported() {
		t.Error("expected Exported()=true even on error")
	}
	if updated.Err() == nil {
		t.Fatal("Err() = nil, want error")
	}
	if updated.Err().Error() != exportErr.Error() {
		t.Errorf("Err() = %v, want %v", updated.Err(), exportErr)
	}
}

// TestView_Integration verifies View renders correct content for different states.
func TestView_Integration(t *testing.T) {
	t.Run("before export completes", func(t *testing.T) {
		m := NewModel(Options{})
		view := m.View()
		if view == "" {
			t.Error("View() returned empty string")
		}
		if !contains(view, "导出") {
			t.Errorf("View() = %q, want to contain '导出'", view)
		}
	})

	t.Run("after successful export", func(t *testing.T) {
		m := NewModel(Options{})
		m.exported = true
		m.result = export.Result{
			Version: "2024-01-01T00:00:00Z",
			Outputs: []checkpoint.OutputArtifact{
				{Kind: "markdown", Path: "final/paper.md", SHA256: "abc"},
				{Kind: "markdown", Path: "final/report.md", SHA256: "def"},
			},
			DocxWritten: true,
		}

		view := m.View()
		if !contains(view, "✓") {
			t.Errorf("View() should show success indicator")
		}
		if !contains(view, "final/paper.md") {
			t.Errorf("View() = %q, want to contain 'final/paper.md'", view)
		}
		if !contains(view, "final/report.md") {
			t.Errorf("View() = %q, want to contain 'final/report.md'", view)
		}
	})

	t.Run("after export error", func(t *testing.T) {
		m := NewModel(Options{})
		m.exported = true
		m.err = export.Error{
			Code:    export.CodeNoAcceptedChapters,
			Message: "no chapters",
		}

		view := m.View()
		if !contains(view, "✗") || !contains(view, "失败") {
			t.Errorf("View() should show error indicator")
		}
		if !contains(view, export.CodeNoAcceptedChapters) {
			t.Errorf("View() = %q, want to contain error code", view)
		}
		if !contains(view, "[r]") || !contains(view, "[b]") {
			t.Errorf("View() = %q, want to show retry and cancel options", view)
		}
	})

	t.Run("docx degradation", func(t *testing.T) {
		m := NewModel(Options{})
		m.exported = true
		m.result = export.Result{
			Version: "2024-01-01T00:00:00Z",
			Outputs: []checkpoint.OutputArtifact{
				{Kind: "markdown", Path: "final/paper.md", SHA256: "abc"},
			},
			Issues: []export.Issue{
				{Code: export.CodeDocxFailed, Message: "docx failed"},
			},
			DocxWritten: false,
		}

		view := m.View()
		if !contains(view, "降级") || !contains(view, "paper.docx") {
			t.Errorf("View() = %q, want to show docx degradation message", view)
		}
		if !contains(view, "paper.md") {
			t.Errorf("View() = %q, want to show markdown is available", view)
		}
	})
}

// TestUpdate_KeyMsg_DelegatesTo_UpdateKey verifies tea.KeyMsg handling.
func TestUpdate_KeyMsg_DelegatesTo_UpdateKey(t *testing.T) {
	m := NewModel(Options{})
	m.exported = true

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("enter")})
	if cmd != nil {
		t.Errorf("cmd = %v, want nil", cmd)
	}

	updatedModel := updated.(Model)
	if !updatedModel.Done() {
		t.Error("expected Done()=true after enter KeyMsg")
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
	if updatedModel.exported {
		t.Error("model should remain unchanged for unknown messages")
	}
}

// TestStore_ReturnsBackingStore verifies Store() accessor.
func TestStore_ReturnsBackingStore(t *testing.T) {
	m := NewModel(Options{WorkDir: "/tmp/test"})
	s := m.Store()

	// Verify store path derivation
	finalPath := s.Path("final")
	if !contains(finalPath, "aipaper") {
		t.Errorf("Store.Path('final') = %q, want to contain 'aipaper'", finalPath)
	}
}

// TestView_NonExportError verifies rendering of a generic (non export.Error) error.
func TestView_NonExportError(t *testing.T) {
	m := NewModel(Options{})
	m.exported = true
	m.err = errors.New("disk full")

	view := m.View()
	if !contains(view, "✗") || !contains(view, "失败") {
		t.Errorf("View() should show error indicator")
	}
	if !contains(view, "disk full") {
		t.Errorf("View() = %q, want to contain generic error message", view)
	}
}

// TestView_UnconfirmedReferenceHint verifies the unconfirmed reference hint text.
func TestView_UnconfirmedReferenceHint(t *testing.T) {
	m := NewModel(Options{})
	m.exported = true
	m.err = export.Error{
		Code:    export.CodeUnconfirmedReference,
		Message: "unconfirmed",
	}

	view := m.View()
	if !contains(view, "未确认") || !contains(view, "未生成错误文件") {
		t.Errorf("View() = %q, want unconfirmed reference hint with 'no error file' note", view)
	}
}

// TestView_EmptyOutputs verifies the "(无)" placeholder when no outputs exist.
func TestView_EmptyOutputs(t *testing.T) {
	m := NewModel(Options{})
	m.exported = true
	m.result = export.Result{
		Version:     "v1",
		Outputs:     []checkpoint.OutputArtifact{},
		DocxWritten: true,
	}

	view := m.View()
	if !contains(view, "(无)") {
		t.Errorf("View() = %q, want '(无)' placeholder for empty outputs", view)
	}
}

// TestView_IssuesWithContext verifies issues render chapter and reference context.
func TestView_IssuesWithContext(t *testing.T) {
	m := NewModel(Options{})
	m.exported = true
	m.result = export.Result{
		Version: "v1",
		Outputs: []checkpoint.OutputArtifact{
			{Kind: "markdown", Path: "final/paper.md", SHA256: "abc"},
		},
		Issues: []export.Issue{
			{
				Code:      export.CodeReferenceFormat,
				Message:   "format warning",
				ChapterID: "ch01",
			},
			{
				Code:         export.CodeReferenceFormat,
				Message:      "missing key",
				ReferenceKey: "Smith2024",
			},
		},
		DocxWritten: true,
	}

	view := m.View()
	if !contains(view, "问题提示") {
		t.Errorf("View() = %q, want '问题提示' header", view)
	}
	if !contains(view, "ch01") {
		t.Errorf("View() = %q, want chapter context 'ch01'", view)
	}
	if !contains(view, "Smith2024") {
		t.Errorf("View() = %q, want reference context 'Smith2024'", view)
	}
}

// TestHandleKey_UnknownKey_NoChange verifies unrecognized keys leave state unchanged.
func TestHandleKey_UnknownKey_NoChange(t *testing.T) {
	m := NewModel(Options{})
	m.exported = true

	updated, cmd := m.UpdateKey("x")
	if cmd != nil {
		t.Errorf("UpdateKey('x') cmd = %v, want nil", cmd)
	}
	if updated.Done() || updated.Canceled() {
		t.Error("expected no state change for unknown key")
	}
}

// TestDefaultExport_LoadInputError verifies the default export path surfaces
// LoadInput errors when no store data is present (no disk write performed).
func TestDefaultExport_LoadInputError(t *testing.T) {
	// Use a temp work dir with no store data so LoadInput fails fast.
	m := NewModel(Options{WorkDir: t.TempDir()})

	cmd := m.Init()
	if cmd == nil {
		t.Fatal("Init() returned nil cmd")
	}
	msg := cmd()
	doneMsg, ok := msg.(exportDoneMsg)
	if !ok {
		t.Fatalf("cmd() returned %T, want exportDoneMsg", msg)
	}
	if doneMsg.err == nil {
		t.Error("expected LoadInput error for empty store, got nil")
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
