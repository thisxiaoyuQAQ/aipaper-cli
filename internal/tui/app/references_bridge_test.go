package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/references"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/store"
)

func TestRootModelReferencesConfirmTransitionsToWriting(t *testing.T) {
	workDir := t.TempDir()
	s := store.New(workDir)
	if err := store.EnsureLayout(s); err != nil {
		t.Fatalf("EnsureLayout failed: %v", err)
	}

	// 准备候选文献
	candidates := contracts.ReferenceCandidates{
		Items: []contracts.ReferenceCandidate{
			{
				ID:      "ref1",
				Title:   "Test Paper 1",
				Authors: []string{"Author A"},
				Year:    2024,
				DOI:     "10.1234/test1",
			},
			{
				ID:      "ref2",
				Title:   "Test Paper 2",
				Authors: []string{"Author B"},
				Year:    2023,
				URL:     "https://example.com/paper2",
			},
		},
	}
	if _, err := store.WriteJSON(s.Path("references", "candidates.json"), candidates, store.Overwrite); err != nil {
		t.Fatalf("write candidates failed: %v", err)
	}

	// 创建 RootModel 并初始化到 References 屏幕
	m := NewRootModel(RootOptions{
		WorkDir:       workDir,
		InitialScreen: ScreenReferences,
	})

	if m.CurrentScreen != ScreenReferences {
		t.Fatalf("expected screen %q, got %q", ScreenReferences, m.CurrentScreen)
	}

	// 模拟用户选择第一个文献
	model, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")})
	m = model.(RootModel)

	// 模拟用户确认
	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = model.(RootModel)

	// 验证转场到 WritingProgress
	if m.CurrentScreen != ScreenWriting {
		t.Errorf("expected screen %q after confirm, got %q", ScreenWriting, m.CurrentScreen)
	}

	// 验证文件写入
	var confirmed contracts.ConfirmedReferences
	if err := store.ReadJSON(s.Path("references", "confirmed.json"), &confirmed); err != nil {
		t.Fatalf("read confirmed.json failed: %v", err)
	}
	if len(confirmed.Items) != 1 {
		t.Errorf("expected 1 confirmed reference, got %d", len(confirmed.Items))
	}
	if confirmed.Items[0].Title != "Test Paper 1" {
		t.Errorf("expected confirmed title %q, got %q", "Test Paper 1", confirmed.Items[0].Title)
	}

	// 验证 BibTeX 文件存在
	bibPath := s.Path(filepath.FromSlash("references/confirmed.bib"))
	if _, err := os.Stat(bibPath); os.IsNotExist(err) {
		t.Errorf("confirmed.bib not created")
	}
}

func TestRootModelReferencesNoneConfirmedBlocksWriting(t *testing.T) {
	workDir := t.TempDir()
	s := store.New(workDir)
	if err := store.EnsureLayout(s); err != nil {
		t.Fatalf("EnsureLayout failed: %v", err)
	}

	// 准备候选文献
	candidates := contracts.ReferenceCandidates{
		Items: []contracts.ReferenceCandidate{
			{
				ID:      "ref1",
				Title:   "Test Paper 1",
				Authors: []string{"Author A"},
				Year:    2024,
			},
		},
	}
	if _, err := store.WriteJSON(s.Path("references", "candidates.json"), candidates, store.Overwrite); err != nil {
		t.Fatalf("write candidates failed: %v", err)
	}

	m := NewRootModel(RootOptions{
		WorkDir:       workDir,
		InitialScreen: ScreenReferences,
	})

	// 直接按 Enter 而不选择任何文献
	model, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = model.(RootModel)

	// 应该停留在 References 屏幕并显示错误
	if m.CurrentScreen != ScreenReferences {
		t.Errorf("expected to stay on %q, got %q", ScreenReferences, m.CurrentScreen)
	}
	if m.err == nil {
		t.Errorf("expected error for none confirmed, got nil")
	}
	if !references.IsNoneConfirmed(m.err) {
		t.Errorf("expected IsNoneConfirmed error, got %v", m.err)
	}
}

func TestRootModelReferencesCancelQuits(t *testing.T) {
	workDir := t.TempDir()
	s := store.New(workDir)
	if err := store.EnsureLayout(s); err != nil {
		t.Fatalf("EnsureLayout failed: %v", err)
	}

	candidates := contracts.ReferenceCandidates{
		Items: []contracts.ReferenceCandidate{
			{
				ID:    "ref1",
				Title: "Test Paper 1",
			},
		},
	}
	if _, err := store.WriteJSON(s.Path("references", "candidates.json"), candidates, store.Overwrite); err != nil {
		t.Fatalf("write candidates failed: %v", err)
	}

	m := NewRootModel(RootOptions{
		WorkDir:       workDir,
		InitialScreen: ScreenReferences,
	})

	// 按 q 取消
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})

	if cmd == nil {
		t.Errorf("expected quit command, got nil")
	}
}

func TestRootModelReferencesLoadCandidatesOnInit(t *testing.T) {
	workDir := t.TempDir()
	s := store.New(workDir)
	if err := store.EnsureLayout(s); err != nil {
		t.Fatalf("EnsureLayout failed: %v", err)
	}

	candidates := contracts.ReferenceCandidates{
		Items: []contracts.ReferenceCandidate{
			{ID: "ref1", Title: "Paper 1"},
			{ID: "ref2", Title: "Paper 2"},
			{ID: "ref3", Title: "Paper 3"},
		},
	}
	if _, err := store.WriteJSON(s.Path("references", "candidates.json"), candidates, store.Overwrite); err != nil {
		t.Fatalf("write candidates failed: %v", err)
	}

	m := NewRootModel(RootOptions{
		WorkDir:       workDir,
		InitialScreen: ScreenReferences,
	})

	if len(m.References.VisibleCandidates()) != 3 {
		t.Errorf("expected 3 visible candidates, got %d", len(m.References.VisibleCandidates()))
	}
}

func TestRootModelReferencesHandlesMissingCandidatesFile(t *testing.T) {
	workDir := t.TempDir()
	s := store.New(workDir)
	if err := store.EnsureLayout(s); err != nil {
		t.Fatalf("EnsureLayout failed: %v", err)
	}

	// 不写入 candidates.json

	m := NewRootModel(RootOptions{
		WorkDir:       workDir,
		InitialScreen: ScreenReferences,
	})

	if m.err == nil {
		t.Errorf("expected error for missing candidates.json, got nil")
	}
}

func TestNewReferencesModelLoadsFromStore(t *testing.T) {
	workDir := t.TempDir()
	s := store.New(workDir)
	if err := store.EnsureLayout(s); err != nil {
		t.Fatalf("EnsureLayout failed: %v", err)
	}

	candidates := contracts.ReferenceCandidates{
		Items: []contracts.ReferenceCandidate{
			{ID: "ref1", Title: "Paper 1"},
		},
	}
	if _, err := store.WriteJSON(s.Path("references", "candidates.json"), candidates, store.Overwrite); err != nil {
		t.Fatalf("write candidates failed: %v", err)
	}

	model, err := newReferencesModel(workDir, nil)
	if err != nil {
		t.Fatalf("newReferencesModel failed: %v", err)
	}

	if len(model.VisibleCandidates()) != 1 {
		t.Errorf("expected 1 visible candidate, got %d", len(model.VisibleCandidates()))
	}
}

func TestUpdateReferencesConfirmCallsConfirmCandidates(t *testing.T) {
	workDir := t.TempDir()
	s := store.New(workDir)
	if err := store.EnsureLayout(s); err != nil {
		t.Fatalf("EnsureLayout failed: %v", err)
	}

	candidates := contracts.ReferenceCandidates{
		Items: []contracts.ReferenceCandidate{
			{
				ID:      "ref1",
				Title:   "Test Paper",
				Authors: []string{"Author A"},
				Year:    2024,
			},
		},
	}
	if _, err := store.WriteJSON(s.Path("references", "candidates.json"), candidates, store.Overwrite); err != nil {
		t.Fatalf("write candidates failed: %v", err)
	}

	m := NewRootModel(RootOptions{
		WorkDir:       workDir,
		InitialScreen: ScreenReferences,
	})

	// 选择文献
	m.References = m.References.UpdateKey(" ")

	// 确认
	m.References = m.References.UpdateKey("enter")

	// 调用 updateReferences
	result, _ := m.updateReferences("enter")
	m = result.(RootModel)

	if m.CurrentScreen != ScreenWriting {
		t.Errorf("expected screen %q, got %q", ScreenWriting, m.CurrentScreen)
	}

	// 验证文件写入
	var confirmed contracts.ConfirmedReferences
	if err := store.ReadJSON(s.Path("references", "confirmed.json"), &confirmed); err != nil {
		t.Fatalf("read confirmed.json failed: %v", err)
	}
	if len(confirmed.Items) != 1 {
		t.Errorf("expected 1 confirmed reference, got %d", len(confirmed.Items))
	}
}

func TestUpdateReferencesPreservesErrorFromModel(t *testing.T) {
	workDir := t.TempDir()
	s := store.New(workDir)
	if err := store.EnsureLayout(s); err != nil {
		t.Fatalf("EnsureLayout failed: %v", err)
	}

	candidates := contracts.ReferenceCandidates{
		Items: []contracts.ReferenceCandidate{
			{ID: "ref1", Title: "Test Paper"},
		},
	}
	if _, err := store.WriteJSON(s.Path("references", "candidates.json"), candidates, store.Overwrite); err != nil {
		t.Fatalf("write candidates failed: %v", err)
	}

	m := NewRootModel(RootOptions{
		WorkDir:       workDir,
		InitialScreen: ScreenReferences,
	})

	// 直接确认不选择任何文献（references model 会设置错误）
	result, _ := m.updateReferences("enter")
	m = result.(RootModel)

	if m.err == nil {
		t.Errorf("expected error from references model, got nil")
	}
}

func TestReferencesDecisionUsesCurrentTime(t *testing.T) {
	workDir := t.TempDir()
	s := store.New(workDir)
	if err := store.EnsureLayout(s); err != nil {
		t.Fatalf("EnsureLayout failed: %v", err)
	}

	candidates := contracts.ReferenceCandidates{
		Items: []contracts.ReferenceCandidate{
			{
				ID:      "ref1",
				Title:   "Test Paper",
				Authors: []string{"Author A"},
				Year:    2024,
			},
		},
	}
	if _, err := store.WriteJSON(s.Path("references", "candidates.json"), candidates, store.Overwrite); err != nil {
		t.Fatalf("write candidates failed: %v", err)
	}

	m := NewRootModel(RootOptions{
		WorkDir:       workDir,
		InitialScreen: ScreenReferences,
	})

	// 选择并确认
	m.References = m.References.UpdateKey(" ")
	m.References = m.References.UpdateKey("enter")

	before := time.Now().UTC().Add(-1 * time.Second)
	result, _ := m.updateReferences("enter")
	after := time.Now().UTC().Add(1 * time.Second)
	m = result.(RootModel)

	// 读取确认结果验证时间戳
	var confirmed contracts.ConfirmedReferences
	if err := store.ReadJSON(s.Path("references", "confirmed.json"), &confirmed); err != nil {
		t.Fatalf("read confirmed.json failed: %v", err)
	}

	confirmedAt := confirmed.Items[0].ConfirmedAt
	if confirmedAt.Before(before) || confirmedAt.After(after) {
		t.Errorf("ConfirmedAt time %v not in expected range [%v, %v]", confirmedAt, before, after)
	}
}
