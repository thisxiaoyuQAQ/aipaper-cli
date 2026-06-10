package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/checkpoint"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/config"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/store"
)

func TestStateProbeEmptyProjectEntersConfigWizard(t *testing.T) {
	isolateHome(t)
	result, err := StateProbe(t.TempDir())
	if err != nil {
		t.Fatalf("StateProbe() error = %v", err)
	}
	if result.ConfigOK {
		t.Fatalf("ConfigOK = true, want false")
	}
	if result.SuggestedScreen != ScreenConfigWizard {
		t.Fatalf("SuggestedScreen = %q, want %q", result.SuggestedScreen, ScreenConfigWizard)
	}
}

func TestStateProbeWithConfigMissingRequirementsEntersRequirements(t *testing.T) {
	workDir := configuredWorkDir(t)

	result, err := StateProbe(workDir)
	if err != nil {
		t.Fatalf("StateProbe() error = %v", err)
	}
	if !result.ConfigOK {
		t.Fatalf("ConfigOK = false, error = %v", result.ConfigError)
	}
	if result.SuggestedScreen != ScreenRequirements {
		t.Fatalf("SuggestedScreen = %q, want %q", result.SuggestedScreen, ScreenRequirements)
	}
}

func TestStateProbeWithRequirementsMissingManifestEntersMaterialsScan(t *testing.T) {
	workDir := configuredWorkDir(t)
	s := store.New(workDir)
	writeRequirements(t, workDir, s)

	result, err := StateProbe(workDir)
	if err != nil {
		t.Fatalf("StateProbe() error = %v", err)
	}
	if !result.HasRequirements {
		t.Fatalf("HasRequirements = false, want true")
	}
	if result.SuggestedScreen != ScreenMaterialsScan {
		t.Fatalf("SuggestedScreen = %q, want %q", result.SuggestedScreen, ScreenMaterialsScan)
	}
}

func TestStateProbeWithCandidatesMissingConfirmedRefsEntersReferences(t *testing.T) {
	workDir := configuredWorkDir(t)
	s := store.New(workDir)
	writeRequirements(t, workDir, s)
	writeManifest(t, s)
	writeCandidates(t, s)

	result, err := StateProbe(workDir)
	if err != nil {
		t.Fatalf("StateProbe() error = %v", err)
	}
	if !result.HasCandidates {
		t.Fatalf("HasCandidates = false, want true")
	}
	if result.SuggestedScreen != ScreenReferences {
		t.Fatalf("SuggestedScreen = %q, want %q", result.SuggestedScreen, ScreenReferences)
	}
}

func TestStateProbeCandidatesWithoutMaterialsEnterReferences(t *testing.T) {
	workDir := configuredWorkDir(t)
	s := store.New(workDir)
	writeRequirements(t, workDir, s)
	writeCandidates(t, s)

	result, err := StateProbe(workDir)
	if err != nil {
		t.Fatalf("StateProbe() error = %v", err)
	}
	if !result.HasCandidates || result.HasMaterials {
		t.Fatalf("candidate/material state = %#v", result)
	}
	if result.SuggestedScreen != ScreenReferences {
		t.Fatalf("SuggestedScreen = %q, want %q", result.SuggestedScreen, ScreenReferences)
	}
}

func TestStateProbeValidCheckpointEntersRecoverPrompt(t *testing.T) {
	workDir := configuredWorkDir(t)
	s := store.New(workDir)
	writeRun(t, s)
	progress := contracts.Progress{
		Phase:             "draft_chapter",
		Status:            "running",
		CompletedChapters: []string{},
		PendingChapters:   []string{"ch01"},
		UpdatedAt:         fixedTime(),
	}
	if err := checkpoint.Record(s, checkpoint.Checkpoint{
		Step:         7,
		Phase:        "draft_chapter",
		CreatedAt:    fixedTime(),
		Outputs:      []checkpoint.OutputArtifact{},
		NextExpected: "review_chapter",
	}, progress); err != nil {
		t.Fatalf("Record() error = %v", err)
	}

	result, err := StateProbe(workDir)
	if err != nil {
		t.Fatalf("StateProbe() error = %v", err)
	}
	if !result.CheckpointValid || result.CheckpointStep != 7 {
		t.Fatalf("checkpoint result = %#v", result)
	}
	if result.SuggestedScreen != ScreenRecoverPrompt {
		t.Fatalf("SuggestedScreen = %q, want %q", result.SuggestedScreen, ScreenRecoverPrompt)
	}
}

func TestStateProbeValidCheckpointWithoutRunDoesNotEnterRecoverPrompt(t *testing.T) {
	workDir := configuredWorkDir(t)
	s := store.New(workDir)
	progress := contracts.Progress{
		Phase:             "draft_chapter",
		Status:            "running",
		CompletedChapters: []string{},
		PendingChapters:   []string{"ch01"},
		UpdatedAt:         fixedTime(),
	}
	if err := checkpoint.Record(s, checkpoint.Checkpoint{
		Step:         7,
		Phase:        "draft_chapter",
		CreatedAt:    fixedTime(),
		Outputs:      []checkpoint.OutputArtifact{},
		NextExpected: "review_chapter",
	}, progress); err != nil {
		t.Fatalf("Record() error = %v", err)
	}

	result, err := StateProbe(workDir)
	if err != nil {
		t.Fatalf("StateProbe() error = %v", err)
	}
	if !result.CheckpointValid {
		t.Fatalf("CheckpointValid = false, want true")
	}
	if result.HasRun {
		t.Fatalf("HasRun = true, want false")
	}
	if result.SuggestedScreen == ScreenRecoverPrompt {
		t.Fatalf("SuggestedScreen = RecoverPrompt without run.json")
	}
}

func TestStateProbeCompleteFinalOutputsEnterDone(t *testing.T) {
	workDir := configuredWorkDir(t)
	s := store.New(workDir)
	writeRequirements(t, workDir, s)
	writeManifest(t, s)
	writeCandidates(t, s)
	writeConfirmed(t, s)
	for _, rel := range []string{
		"final/paper.md",
		"final/references.md",
		"final/citation-trace.json",
		"final/report.md",
	} {
		writeFile(t, s.Path(filepath.FromSlash(rel)), "ok")
	}

	result, err := StateProbe(workDir)
	if err != nil {
		t.Fatalf("StateProbe() error = %v", err)
	}
	if !result.HasFinalOutputs {
		t.Fatalf("HasFinalOutputs = false, want true")
	}
	if result.SuggestedScreen != ScreenDone {
		t.Fatalf("SuggestedScreen = %q, want %q", result.SuggestedScreen, ScreenDone)
	}
}

func TestStateProbeInvalidRequirementsReturnsToRequirements(t *testing.T) {
	workDir := configuredWorkDir(t)
	s := store.New(workDir)
	if _, err := store.WriteJSON(s.RequirementsPath(), contracts.Requirements{
		Topic:         "partial requirements",
		Language:      "bad-language",
		CitationStyle: "bad-style",
		TargetWords:   0,
		MaterialDir:   "",
	}, store.Overwrite); err != nil {
		t.Fatalf("write requirements: %v", err)
	}

	result, err := StateProbe(workDir)
	if err != nil {
		t.Fatalf("StateProbe() error = %v", err)
	}
	if result.HasRequirements {
		t.Fatalf("HasRequirements = true for invalid requirements")
	}
	if result.SuggestedScreen != ScreenRequirements {
		t.Fatalf("SuggestedScreen = %q, want %q", result.SuggestedScreen, ScreenRequirements)
	}
}

func TestStateProbeFailedOnlyManifestReturnsToMaterialsScan(t *testing.T) {
	workDir := configuredWorkDir(t)
	s := store.New(workDir)
	writeRequirements(t, workDir, s)
	if _, err := store.WriteJSON(s.Path("materials", "manifest.json"), contracts.MaterialManifest{Items: []contracts.MaterialItem{{
		ID:       "material_001",
		Path:     "missing",
		Kind:     "unknown",
		Status:   "failed",
		Degraded: false,
		Error:    "material directory does not exist",
	}}}, store.Overwrite); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	result, err := StateProbe(workDir)
	if err != nil {
		t.Fatalf("StateProbe() error = %v", err)
	}
	if result.HasMaterials {
		t.Fatalf("HasMaterials = true for failed-only manifest")
	}
	if result.SuggestedScreen != ScreenMaterialsScan {
		t.Fatalf("SuggestedScreen = %q, want %q", result.SuggestedScreen, ScreenMaterialsScan)
	}
}

func TestStateProbeParsedManifestRequiresOutputFiles(t *testing.T) {
	workDir := configuredWorkDir(t)
	s := store.New(workDir)
	writeRequirements(t, workDir, s)
	if _, err := store.WriteJSON(s.Path("materials", "manifest.json"), contracts.MaterialManifest{Items: []contracts.MaterialItem{{
		ID:         "material_001",
		Path:       "notes.md",
		Kind:       "markdown",
		Status:     "parsed",
		OutputText: "materials/extracted/missing.md",
		OutputMeta: "materials/parsed/missing.json",
	}}}, store.Overwrite); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	result, err := StateProbe(workDir)
	if err != nil {
		t.Fatalf("StateProbe() error = %v", err)
	}
	if result.HasMaterials {
		t.Fatalf("HasMaterials = true for manifest with missing output files")
	}
	if result.SuggestedScreen != ScreenMaterialsScan {
		t.Fatalf("SuggestedScreen = %q, want %q", result.SuggestedScreen, ScreenMaterialsScan)
	}
}

func TestStateProbeInvalidCheckpointDoesNotEnterRecoverPrompt(t *testing.T) {
	workDir := configuredWorkDir(t)
	s := store.New(workDir)
	if err := store.EnsureLayout(s); err != nil {
		t.Fatalf("EnsureLayout() error = %v", err)
	}
	if _, err := store.WriteJSON(s.LatestCheckpointPath(), checkpoint.Checkpoint{
		Step:      3,
		Phase:     "draft_chapter",
		CreatedAt: fixedTime(),
		Outputs: []checkpoint.OutputArtifact{{
			Kind:   "draft",
			Path:   "drafts/ch01/missing.md",
			SHA256: "bad",
		}},
	}, store.Overwrite); err != nil {
		t.Fatalf("write latest checkpoint: %v", err)
	}

	result, err := StateProbe(workDir)
	if err != nil {
		t.Fatalf("StateProbe() error = %v", err)
	}
	if result.CheckpointValid {
		t.Fatalf("CheckpointValid = true, want false")
	}
	if len(result.CheckpointErrors) == 0 {
		t.Fatalf("CheckpointErrors = empty, want readable validation errors")
	}
	if result.SuggestedScreen == ScreenRecoverPrompt {
		t.Fatalf("SuggestedScreen = RecoverPrompt for invalid checkpoint")
	}
}

func configuredWorkDir(t *testing.T) string {
	t.Helper()
	isolateHome(t)
	workDir := t.TempDir()
	if _, err := config.SaveProject(workDir, testConfig()); err != nil {
		t.Fatalf("SaveProject() error = %v", err)
	}
	return workDir
}

func isolateHome(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
}

func testConfig() config.Config {
	roles := map[string]config.RoleConfig{}
	for _, role := range []string{"coordinator", "architect", "writer", "editor"} {
		roles[role] = config.RoleConfig{Provider: "default", Model: "llama3"}
	}
	return config.Config{
		Provider: "default",
		Model:    "llama3",
		Providers: map[string]config.ProviderConfig{
			"default": {Type: "ollama", BaseURL: "http://localhost:11434"},
		},
		Roles: roles,
	}
}

func writeRequirements(t *testing.T, workDir string, s store.Store) {
	t.Helper()
	materialDir := filepath.Join(workDir, "materials")
	if err := os.MkdirAll(materialDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(materials) error = %v", err)
	}
	if _, err := store.WriteJSON(s.RequirementsPath(), contracts.Requirements{
		Topic:             "RAG literature reviews",
		ResearchQuestions: []string{"How should references be confirmed?"},
		Scope:             "mini review",
		Language:          "en",
		CitationStyle:     "apa",
		TargetWords:       800,
		MaterialDir:       materialDir,
	}, store.Overwrite); err != nil {
		t.Fatalf("write requirements: %v", err)
	}
}

func writeManifest(t *testing.T, s store.Store) {
	t.Helper()
	writeFile(t, s.Path("materials", "extracted", "material_001.md"), "notes")
	writeFile(t, s.Path("materials", "parsed", "material_001.json"), `{"id":"material_001"}`)
	if _, err := store.WriteJSON(s.Path("materials", "manifest.json"), contracts.MaterialManifest{Items: []contracts.MaterialItem{{
		ID:         "material_001",
		Path:       "notes.md",
		Kind:       "markdown",
		Status:     "parsed",
		OutputText: "materials/extracted/material_001.md",
		OutputMeta: "materials/parsed/material_001.json",
	}}}, store.Overwrite); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
}

func writeCandidates(t *testing.T, s store.Store) {
	t.Helper()
	candidates := contracts.ReferenceCandidates{Items: []contracts.ReferenceCandidate{{
		ID:      "cand_001",
		Title:   "Human confirmed references",
		Authors: []string{"Lee"},
		Year:    2026,
		Source:  "mock",
		Status:  "pending",
	}}}
	if _, err := store.WriteJSON(s.Path("references", "candidates.json"), candidates, store.Overwrite); err != nil {
		t.Fatalf("write candidates: %v", err)
	}
}

func writeConfirmed(t *testing.T, s store.Store) {
	t.Helper()
	confirmed := contracts.ConfirmedReferences{Items: []contracts.ConfirmedReference{{
		Key:         "lee2026HumanConfirmedReferences",
		Title:       "Human confirmed references",
		Authors:     []string{"Lee"},
		Year:        2026,
		ConfirmedAt: fixedTime(),
	}}}
	if _, err := store.WriteJSON(s.Path("references", "confirmed.json"), confirmed, store.Overwrite); err != nil {
		t.Fatalf("write confirmed: %v", err)
	}
}

func writeRun(t *testing.T, s store.Store) {
	t.Helper()
	if _, err := store.WriteJSON(s.RunPath(), contracts.Run{
		RunID:        "run-20260608T120000Z",
		CreatedAt:    fixedTime(),
		CostEstimate: map[string]any{},
		Events:       []contracts.RunEvent{},
	}, store.Overwrite); err != nil {
		t.Fatalf("write run: %v", err)
	}
}

func writeFile(t *testing.T, path, value string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, []byte(value), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}

func fixedTime() time.Time {
	return time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC)
}
