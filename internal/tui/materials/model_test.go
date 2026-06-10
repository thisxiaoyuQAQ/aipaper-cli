package materials

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
	domainmaterials "github.com/thisxiaoyuQAQ/aipaper-cli/internal/materials"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/store"
)

func TestModelMissingDirectoryCreatesAndPrompts(t *testing.T) {
	workDir := t.TempDir()
	materialDir := filepath.Join(workDir, "materials")
	model := NewModel(Options{WorkDir: workDir, MaterialDir: materialDir})

	msg := model.Init()()
	updated, cmd := model.Update(msg)
	if cmd != nil {
		t.Fatalf("cmd = %v, want nil", cmd)
	}
	if updated.Status() != StatusEmpty {
		t.Fatalf("Status() = %q, want %q", updated.Status(), StatusEmpty)
	}
	if !updated.CreatedDir() {
		t.Fatalf("CreatedDir() = false, want true")
	}
	if info, err := os.Stat(materialDir); err != nil || !info.IsDir() {
		t.Fatalf("material dir was not created: info=%v err=%v", info, err)
	}
	if !strings.Contains(updated.View(), "was created") {
		t.Fatalf("View() = %q, want created prompt", updated.View())
	}
}

func TestModelEmptyDirectoryWritesEmptyManifest(t *testing.T) {
	workDir := t.TempDir()
	materialDir := filepath.Join(workDir, "materials")
	if err := os.MkdirAll(materialDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	model := NewModel(Options{WorkDir: workDir, MaterialDir: materialDir})

	updated, _ := model.Update(model.Init()())
	if updated.Status() != StatusEmpty {
		t.Fatalf("Status() = %q, want %q", updated.Status(), StatusEmpty)
	}
	if updated.Stats().Total != 0 {
		t.Fatalf("Stats() = %#v, want no items", updated.Stats())
	}
	if _, err := os.Stat(store.New(workDir).Path("materials", "manifest.json")); err != nil {
		t.Fatalf("manifest was not written: %v", err)
	}
}

func TestModelScansTextMarkdownAndBibTeX(t *testing.T) {
	workDir := t.TempDir()
	materialDir := filepath.Join(workDir, "materials")
	writeFile(t, filepath.Join(materialDir, "notes.md"), "# Notes\n")
	writeFile(t, filepath.Join(materialDir, "plain.txt"), "plain text\n")
	writeFile(t, filepath.Join(materialDir, "refs.bib"), `@article{smith2024rag,
  title={Retrieval Augmented Generation Reviews},
  author={Smith and Lee},
  year={2024},
  doi={10.1000/rag}
}`)

	model := NewModel(Options{WorkDir: workDir, MaterialDir: materialDir})
	updated, _ := model.Update(model.Init()())
	if updated.Status() != StatusComplete {
		t.Fatalf("Status() = %q, want %q; view:\n%s", updated.Status(), StatusComplete, updated.View())
	}
	stats := updated.Stats()
	if stats.Parsed != 3 || stats.Candidates != 1 {
		t.Fatalf("Stats() = %#v, want 3 parsed and 1 candidate", stats)
	}
	if candidates := updated.Candidates(); len(candidates) != 1 || candidates[0].Source != "bibtex" {
		t.Fatalf("Candidates() = %#v, want one bibtex candidate", candidates)
	}
	result := updated.Result()
	if len(result.Candidates) != 1 {
		t.Fatalf("Result().Candidates = %#v, want one candidate", result.Candidates)
	}
}

func TestModelShowsDegradedFormats(t *testing.T) {
	workDir := t.TempDir()
	materialDir := filepath.Join(workDir, "materials")
	writeFile(t, filepath.Join(materialDir, "table.csv"), "title,year\nA,2024\n")
	writeFile(t, filepath.Join(materialDir, "link.url"), "https://example.com/paper\n")

	model := NewModel(Options{WorkDir: workDir, MaterialDir: materialDir})
	updated, _ := model.Update(model.Init()())
	if updated.Status() != StatusComplete {
		t.Fatalf("Status() = %q, want %q; view:\n%s", updated.Status(), StatusComplete, updated.View())
	}
	if updated.Stats().Degraded != 2 {
		t.Fatalf("Stats().Degraded = %d, want 2", updated.Stats().Degraded)
	}
	updated, _ = updated.UpdateKey("d")
	if !updated.Details() || !strings.Contains(updated.View(), "degraded") {
		t.Fatalf("details view = %q, want degraded item details", updated.View())
	}
}

func TestModelSingleFileFailureDoesNotBlock(t *testing.T) {
	workDir := t.TempDir()
	materialDir := filepath.Join(workDir, "materials")
	writeFile(t, filepath.Join(materialDir, "notes.md"), "# Notes\n")
	writeFile(t, filepath.Join(materialDir, "bad.bib"), "@article{bad")

	model := NewModel(Options{WorkDir: workDir, MaterialDir: materialDir})
	updated, _ := model.Update(model.Init()())
	if updated.Status() != StatusComplete {
		t.Fatalf("Status() = %q, want %q; view:\n%s", updated.Status(), StatusComplete, updated.View())
	}
	if updated.Stats().Parsed != 1 || updated.Stats().Failed != 1 {
		t.Fatalf("Stats() = %#v, want one parsed and one failed", updated.Stats())
	}
	updated, _ = updated.UpdateKey("enter")
	if updated.Action() != ActionContinue {
		t.Fatalf("Action() = %q, want %q", updated.Action(), ActionContinue)
	}
}

func TestModelAllFailedCanRetrySkipOrReturn(t *testing.T) {
	workDir := t.TempDir()
	materialDir := filepath.Join(workDir, "materials")
	writeFile(t, filepath.Join(materialDir, "bad.bib"), "@article{bad")

	model := NewModel(Options{WorkDir: workDir, MaterialDir: materialDir})
	updated, _ := model.Update(model.Init()())
	if updated.Status() != StatusAllFailed {
		t.Fatalf("Status() = %q, want %q; view:\n%s", updated.Status(), StatusAllFailed, updated.View())
	}
	if updated.Stats().Failed != 1 {
		t.Fatalf("Stats().Failed = %d, want 1", updated.Stats().Failed)
	}

	skipped, cmd := updated.UpdateKey("s")
	if cmd != nil {
		t.Fatalf("cmd = %v, want nil", cmd)
	}
	if skipped.Action() != ActionSkip || !skipped.Result().Skipped {
		t.Fatalf("skip action/result = %q %#v", skipped.Action(), skipped.Result())
	}

	back, _ := updated.UpdateKey("b")
	if back.Action() != ActionBack {
		t.Fatalf("Action() = %q, want %q", back.Action(), ActionBack)
	}

	retrying, cmd := updated.UpdateKey("r")
	if retrying.Status() != StatusScanning {
		t.Fatalf("Status() after retry = %q, want %q", retrying.Status(), StatusScanning)
	}
	if cmd == nil {
		t.Fatalf("retry cmd = nil, want scan command")
	}
	if _, ok := cmd().(tea.Msg); !ok {
		t.Fatalf("retry command did not return a tea message")
	}
}

func TestModelViewRedactsSensitivePathDetails(t *testing.T) {
	secretDir := filepath.Join(t.TempDir(), "private")
	absolutePath := filepath.Join(secretDir, "notes.md")
	model := NewModel(Options{WorkDir: t.TempDir(), MaterialDir: "materials"})
	updated, _ := model.Update(ScanFinishedMsg{
		Result: domainResultWithSensitiveDetails(absolutePath),
	})
	updated, _ = updated.UpdateKey("d")

	view := updated.View()
	if strings.Contains(view, secretDir) {
		t.Fatalf("View() leaked absolute directory: %q", view)
	}
	if strings.Contains(view, "token=secret") {
		t.Fatalf("View() leaked URL query: %q", view)
	}
	if !strings.Contains(view, "notes.md") || !strings.Contains(view, "https://example.com/paper") {
		t.Fatalf("View() = %q, want sanitized filename and URL", view)
	}
}

func domainResultWithSensitiveDetails(path string) domainmaterials.Result {
	return domainmaterials.Result{
		Manifest: contracts.MaterialManifest{Items: []contracts.MaterialItem{{
			ID:       "material_001",
			Path:     path,
			Kind:     "markdown",
			Status:   domainmaterials.StatusFailed,
			Error:    fmt.Sprintf("invalid url material: https://example.com/paper?token=secret in %s", path),
			Degraded: false,
		}}},
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}
