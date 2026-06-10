package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	runtimeapp "github.com/thisxiaoyuQAQ/aipaper-cli/internal/app"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/config"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
	domainmaterials "github.com/thisxiaoyuQAQ/aipaper-cli/internal/materials"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/store"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/tui/configwizard"
	materialstui "github.com/thisxiaoyuQAQ/aipaper-cli/internal/tui/materials"
	requirementstui "github.com/thisxiaoyuQAQ/aipaper-cli/internal/tui/requirements"
	searchtui "github.com/thisxiaoyuQAQ/aipaper-cli/internal/tui/search"
)

func TestRootModelTransitionsScreen(t *testing.T) {
	model := NewRootModel(RootOptions{InitialScreen: ScreenConfigWizard})

	updated, cmd := model.Update(ScreenTransitionMsg{
		Next: ScreenRequirements,
		Data: "payload",
	})
	if cmd != nil {
		t.Fatalf("cmd = %v, want nil", cmd)
	}
	root, ok := updated.(RootModel)
	if !ok {
		t.Fatalf("updated model type = %T, want RootModel", updated)
	}
	if root.CurrentScreen != ScreenRequirements {
		t.Fatalf("CurrentScreen = %q, want %q", root.CurrentScreen, ScreenRequirements)
	}
	if root.ScreenData != "payload" {
		t.Fatalf("ScreenData = %#v, want payload", root.ScreenData)
	}
	if root.Err() != nil {
		t.Fatalf("Err() = %v, want nil", root.Err())
	}
}

func TestRootModelRejectsUnknownScreen(t *testing.T) {
	model := NewRootModel(RootOptions{InitialScreen: ScreenRequirements})

	updated, _ := model.Update(ScreenTransitionMsg{Next: Screen("unknown")})
	root := updated.(RootModel)
	if root.CurrentScreen != ScreenRequirements {
		t.Fatalf("CurrentScreen = %q, want %q", root.CurrentScreen, ScreenRequirements)
	}
	if root.Err() == nil {
		t.Fatalf("Err() = nil, want unknown screen error")
	}
}

func TestRootModelQuitKeyReturnsQuitCommand(t *testing.T) {
	model := NewRootModel(RootOptions{InitialScreen: ScreenRequirements})

	_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatalf("cmd = nil, want tea.Quit")
	}
	if msg := cmd(); msg != (tea.QuitMsg{}) {
		t.Fatalf("cmd() = %#v, want tea.QuitMsg", msg)
	}
}

func TestRootModelRequirementsSubmitWritesStoreAndTransitions(t *testing.T) {
	workDir := t.TempDir()
	model := NewRootModel(RootOptions{WorkDir: workDir, InitialScreen: ScreenRequirements})
	model.Requirements = model.Requirements.SetField(requirementstui.FieldTopic, "RAG reviews")

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatalf("cmd = nil, want materials scan command")
	}
	root := updated.(RootModel)
	if root.CurrentScreen != ScreenMaterialsScan {
		t.Fatalf("CurrentScreen = %q, want %q", root.CurrentScreen, ScreenMaterialsScan)
	}
	if root.Err() != nil {
		t.Fatalf("Err() = %v, want nil", root.Err())
	}
	if _, err := os.Stat(filepath.Join(workDir, "materials")); err != nil {
		t.Fatalf("default materials dir was not created: %v", err)
	}

	var req contracts.Requirements
	if err := store.ReadJSON(store.New(workDir).RequirementsPath(), &req); err != nil {
		t.Fatalf("read requirements: %v", err)
	}
	if req.Topic != "RAG reviews" {
		t.Fatalf("Topic = %q, want RAG reviews", req.Topic)
	}
	if req.MaterialDir != "./materials" {
		t.Fatalf("MaterialDir = %q, want ./materials", req.MaterialDir)
	}
	if req.TargetWords != 8000 || req.Language != "zh-CN" || req.CitationStyle != "gbt7714" {
		t.Fatalf("defaults = %#v", req)
	}
	if req.ResearchQuestions == nil || req.ChapterPreferences == nil || req.Constraints == nil {
		t.Fatalf("empty slices should be serialized as arrays: %#v", req)
	}
}

func TestRootModelRequirementsSubmitStartsMaterialsScan(t *testing.T) {
	workDir := t.TempDir()
	materialDir := filepath.Join(workDir, "materials")
	writeFile(t, filepath.Join(materialDir, "refs.bib"), `@article{smith2024rag,
  title={Retrieval Augmented Generation Reviews},
  author={Smith and Lee},
  year={2024},
  doi={10.1000/rag}
}`)
	model := NewRootModel(RootOptions{WorkDir: workDir, InitialScreen: ScreenRequirements})
	model.Requirements = model.Requirements.SetField(requirementstui.FieldTopic, "RAG reviews")

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatalf("cmd = nil, want materials scan command")
	}
	root := updated.(RootModel)
	if root.CurrentScreen != ScreenMaterialsScan {
		t.Fatalf("CurrentScreen = %q, want %q", root.CurrentScreen, ScreenMaterialsScan)
	}

	updated, _ = root.Update(cmd())
	root = updated.(RootModel)
	if root.Materials.Status() != materialstui.StatusComplete {
		t.Fatalf("Materials.Status() = %q, want complete; view:\n%s", root.Materials.Status(), root.View())
	}
	if len(root.Materials.Candidates()) != 1 {
		t.Fatalf("Materials.Candidates() = %#v, want one bibtex candidate", root.Materials.Candidates())
	}
}

func TestRootModelRequirementsSubmitCreatesCustomMaterialDir(t *testing.T) {
	workDir := t.TempDir()
	customDir := "./custom-materials"
	model := NewRootModel(RootOptions{WorkDir: workDir, InitialScreen: ScreenRequirements})
	model.Requirements = model.Requirements.SetField(requirementstui.FieldTopic, "RAG reviews")
	model.Requirements = model.Requirements.SetField(requirementstui.FieldMaterialDir, customDir)

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatalf("cmd = nil, want materials scan command")
	}
	root := updated.(RootModel)
	if root.CurrentScreen != ScreenMaterialsScan {
		t.Fatalf("CurrentScreen = %q, want %q", root.CurrentScreen, ScreenMaterialsScan)
	}
	if _, err := os.Stat(filepath.Join(workDir, "custom-materials")); err != nil {
		t.Fatalf("custom material dir was not created: %v", err)
	}

	updated, _ = root.Update(cmd())
	root = updated.(RootModel)
	if root.Materials.Status() != materialstui.StatusEmpty || !root.Materials.CreatedDir() {
		t.Fatalf("materials status=%q created=%v view:\n%s", root.Materials.Status(), root.Materials.CreatedDir(), root.View())
	}
	if !strings.Contains(root.View(), "was created") {
		t.Fatalf("View() = %q, want created prompt", root.View())
	}

	var req contracts.Requirements
	if err := store.ReadJSON(store.New(workDir).RequirementsPath(), &req); err != nil {
		t.Fatalf("read requirements: %v", err)
	}
	if req.MaterialDir != customDir {
		t.Fatalf("MaterialDir = %q, want %q", req.MaterialDir, customDir)
	}
}

func TestRootModelMaterialsContinuePassesCandidatesToSearch(t *testing.T) {
	workDir := t.TempDir()
	req := contracts.Requirements{
		Topic:             "RAG Reviews",
		Language:          "en",
		CitationStyle:     "apa",
		TargetWords:       800,
		MaterialDir:       "./materials",
		AllowOnlineSearch: false,
	}
	if _, err := store.WriteJSON(store.New(workDir).RequirementsPath(), req, store.Overwrite); err != nil {
		t.Fatalf("write requirements: %v", err)
	}
	model := NewRootModel(RootOptions{WorkDir: workDir, InitialScreen: ScreenRequirements})
	candidates := []contracts.ReferenceCandidate{{
		ID:     "cand_001",
		Title:  "RAG Reviews",
		Source: "bibtex",
		Status: "pending",
	}}
	model.CurrentScreen = ScreenMaterialsScan
	model.Materials, _ = materialstui.NewModel(materialstui.Options{
		WorkDir:     workDir,
		MaterialDir: "materials",
		Scan: func(string, store.Store) (domainmaterials.Result, error) {
			return domainmaterials.Result{}, nil
		},
	}).Update(materialstui.ScanFinishedMsg{
		MaterialDir: "materials",
		Result: domainmaterials.Result{
			Manifest: contracts.MaterialManifest{Items: []contracts.MaterialItem{{
				ID:         "material_001",
				Path:       "refs.bib",
				Kind:       "bibtex",
				Status:     domainmaterials.StatusParsed,
				OutputText: "materials/extracted/material_001.md",
				OutputMeta: "materials/parsed/material_001.json",
			}}},
			Candidates: candidates,
		},
	})

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatalf("cmd = nil, want search init command")
	}
	root := updated.(RootModel)
	if root.CurrentScreen != ScreenSearchProgress {
		t.Fatalf("CurrentScreen = %q, want %q", root.CurrentScreen, ScreenSearchProgress)
	}
	result, ok := root.ScreenData.(materialstui.ScanResult)
	if !ok {
		t.Fatalf("ScreenData type = %T, want materialstui.ScanResult", root.ScreenData)
	}
	if len(result.Candidates) != 1 || result.Candidates[0].Source != "bibtex" {
		t.Fatalf("ScreenData candidates = %#v, want retained bibtex candidate", result.Candidates)
	}

	updated, _ = root.Update(cmd())
	root = updated.(RootModel)
	if root.Search.Status() != searchtui.StatusDisabled || root.Search.FinalCount() != 1 {
		t.Fatalf("search status=%q final=%d", root.Search.Status(), root.Search.FinalCount())
	}
}

func TestRootModelSearchContinueLoadsReferences(t *testing.T) {
	workDir := t.TempDir()
	s := store.New(workDir)
	candidates := contracts.ReferenceCandidates{Items: []contracts.ReferenceCandidate{{
		ID:     "cand_001",
		Title:  "RAG Reviews",
		Source: "bibtex",
		Status: "pending",
	}}}
	if _, err := store.WriteJSON(s.Path("references", "candidates.json"), candidates, store.Overwrite); err != nil {
		t.Fatalf("write candidates: %v", err)
	}

	model := NewRootModel(RootOptions{WorkDir: workDir, InitialScreen: ScreenRequirements})
	model.CurrentScreen = ScreenSearchProgress
	model.Search = searchtui.NewModel(searchtui.Options{
		WorkDir: workDir,
		Store:   s,
		Requirements: contracts.Requirements{
			AllowOnlineSearch: false,
		},
		MaterialsResult: materialstui.ScanResult{
			Candidates: candidates.Items,
		},
	})
	updatedSearch, _ := model.Search.Update(model.Search.Init()())
	model.Search = updatedSearch

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Fatalf("cmd = %v, want nil", cmd)
	}
	root := updated.(RootModel)
	if root.CurrentScreen != ScreenReferences {
		t.Fatalf("CurrentScreen = %q, want %q", root.CurrentScreen, ScreenReferences)
	}
	if !strings.Contains(root.View(), "RAG Reviews") {
		t.Fatalf("references view = %q, want loaded candidate", root.View())
	}
}

func TestRootModelMaterialsBackReturnsToRequirements(t *testing.T) {
	model := NewRootModel(RootOptions{WorkDir: "work", InitialScreen: ScreenSearchProgress})
	model.CurrentScreen = ScreenMaterialsScan
	model.Materials, _ = materialstui.NewModel(materialstui.Options{
		WorkDir:     "work",
		MaterialDir: "materials",
	}).Update(materialstui.ScanFinishedMsg{CreatedDir: true})

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	root := updated.(RootModel)
	if root.CurrentScreen != ScreenRequirements {
		t.Fatalf("CurrentScreen = %q, want %q", root.CurrentScreen, ScreenRequirements)
	}
}

func TestRootModelMaterialsTransitionKeepsCurrentScreenWhenSetupFails(t *testing.T) {
	model := NewRootModel(RootOptions{WorkDir: t.TempDir(), InitialScreen: ScreenRequirements})

	updated, cmd := model.Update(ScreenTransitionMsg{Next: ScreenMaterialsScan})
	if cmd != nil {
		t.Fatalf("cmd = %v, want nil", cmd)
	}
	root := updated.(RootModel)
	if root.CurrentScreen != ScreenRequirements {
		t.Fatalf("CurrentScreen = %q, want %q", root.CurrentScreen, ScreenRequirements)
	}
	if root.Err() == nil || !strings.Contains(root.Err().Error(), "load requirements") {
		t.Fatalf("Err() = %v, want load requirements error", root.Err())
	}
}

func TestRootModelRequirementsSubmitRejectsInvalidInput(t *testing.T) {
	workDir := t.TempDir()
	model := NewRootModel(RootOptions{WorkDir: workDir, InitialScreen: ScreenRequirements})

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Fatalf("cmd = %v, want nil", cmd)
	}
	root := updated.(RootModel)
	if root.CurrentScreen != ScreenRequirements {
		t.Fatalf("CurrentScreen = %q, want %q", root.CurrentScreen, ScreenRequirements)
	}
	if root.Err() == nil || !strings.Contains(root.Err().Error(), "topic") {
		t.Fatalf("Err() = %v, want topic validation error", root.Err())
	}
	if _, err := os.Stat(store.New(workDir).RequirementsPath()); !os.IsNotExist(err) {
		t.Fatalf("requirements.json stat error = %v, want not exist", err)
	}
}

func TestRootModelConfigWizardSaveTransitionsToRequirements(t *testing.T) {
	model := NewRootModel(RootOptions{WorkDir: "work", InitialScreen: ScreenConfigWizard})
	model.ConfigWizard = configwizard.NewModel(configwizard.Options{
		WorkDir: "work",
		Save: func(workDir string, cfg config.Config) (string, error) {
			if workDir != "work" {
				t.Fatalf("workDir = %q, want work", workDir)
			}
			if cfg.Provider != "default" || cfg.Model == "" {
				t.Fatalf("saved config = %#v", cfg)
			}
			return "work/aipaper.json", nil
		},
	})

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updated, _ = updated.(RootModel).Update(tea.KeyMsg{Type: tea.KeyEnter})
	updated, cmd := updated.(RootModel).Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Fatalf("cmd = %v, want nil", cmd)
	}
	root := updated.(RootModel)
	if root.CurrentScreen != ScreenRequirements {
		t.Fatalf("CurrentScreen = %q, want %q", root.CurrentScreen, ScreenRequirements)
	}
	if root.ScreenData != "work/aipaper.json" {
		t.Fatalf("ScreenData = %#v, want saved path", root.ScreenData)
	}
}

func TestRootModelUsesStateProbeWhenInitialScreenIsEmpty(t *testing.T) {
	workDir := t.TempDir()
	s := store.New(workDir)
	writeRequirements(t, workDir, s)

	model := NewRootModel(RootOptions{
		WorkDir: workDir,
		StateProbe: func(probeWorkDir string) (ProbeResult, error) {
			if probeWorkDir != workDir {
				t.Fatalf("workDir = %q, want %q", probeWorkDir, workDir)
			}
			return ProbeResult{WorkDir: probeWorkDir, SuggestedScreen: ScreenMaterialsScan}, nil
		},
	})

	if model.CurrentScreen != ScreenMaterialsScan {
		t.Fatalf("CurrentScreen = %q, want %q", model.CurrentScreen, ScreenMaterialsScan)
	}
}

func TestRootModelRecoverPromptContinueTransitionsToWriting(t *testing.T) {
	model := NewRootModel(RootOptions{
		WorkDir:       "work",
		InitialScreen: ScreenRecoverPrompt,
		Recover: func(workDir string) (runtimeapp.RecoveryResult, error) {
			if workDir != "work" {
				t.Fatalf("workDir = %q, want work", workDir)
			}
			return runtimeapp.RecoveryResult{
				OK:             true,
				ResumedFrom:    7,
				RecoveryPrompt: "resume from step 7",
			}, nil
		},
	})
	model.Probe = ProbeResult{WorkDir: "work", CheckpointStep: 7, CheckpointValid: true}
	model.RecoverPrompt = NewRecoverPromptModel(model.Probe)

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Fatalf("cmd = %v, want nil", cmd)
	}
	root := updated.(RootModel)
	if root.CurrentScreen != ScreenWriting {
		t.Fatalf("CurrentScreen = %q, want %q", root.CurrentScreen, ScreenWriting)
	}
	data, ok := root.ScreenData.(WritingResumeData)
	if !ok {
		t.Fatalf("ScreenData type = %T, want WritingResumeData", root.ScreenData)
	}
	if data.RecoveryPrompt != "resume from step 7" || data.Recovery.ResumedFrom != 7 {
		t.Fatalf("resume data = %#v", data)
	}
}

func TestRootModelRecoverPromptRestartRequiresConfirmation(t *testing.T) {
	model := NewRootModel(RootOptions{InitialScreen: ScreenRecoverPrompt})
	model.Probe = ProbeResult{WorkDir: "work", CheckpointStep: 7, CheckpointValid: true}
	model.RecoverPrompt = NewRecoverPromptModel(model.Probe)

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	root := updated.(RootModel)
	if root.CurrentScreen != ScreenRecoverPrompt {
		t.Fatalf("CurrentScreen after r = %q, want %q", root.CurrentScreen, ScreenRecoverPrompt)
	}

	updated, _ = root.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	root = updated.(RootModel)
	if root.CurrentScreen != ScreenRequirements {
		t.Fatalf("CurrentScreen after y = %q, want %q", root.CurrentScreen, ScreenRequirements)
	}
}

func TestRootModelRecoverPromptRestartConfirmationIgnoresContinueKeys(t *testing.T) {
	calledRecover := false
	model := NewRootModel(RootOptions{
		InitialScreen: ScreenRecoverPrompt,
		Recover: func(string) (runtimeapp.RecoveryResult, error) {
			calledRecover = true
			return runtimeapp.RecoveryResult{OK: true}, nil
		},
	})
	model.Probe = ProbeResult{WorkDir: "work", CheckpointStep: 7, CheckpointValid: true}
	model.RecoverPrompt = NewRecoverPromptModel(model.Probe)

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	root := updated.(RootModel)
	updated, _ = root.Update(tea.KeyMsg{Type: tea.KeyEnter})
	root = updated.(RootModel)
	if root.CurrentScreen != ScreenRecoverPrompt {
		t.Fatalf("CurrentScreen after enter = %q, want %q", root.CurrentScreen, ScreenRecoverPrompt)
	}
	if calledRecover {
		t.Fatalf("recover called while restart confirmation was active")
	}

	updated, _ = root.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	root = updated.(RootModel)
	if root.CurrentScreen != ScreenRecoverPrompt {
		t.Fatalf("CurrentScreen after c = %q, want %q", root.CurrentScreen, ScreenRecoverPrompt)
	}
	if calledRecover {
		t.Fatalf("recover called while restart confirmation was active")
	}
}
