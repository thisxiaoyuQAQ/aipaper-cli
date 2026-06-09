package e2e

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	runtimeapp "github.com/thisxiaoyuQAQ/aipaper-cli/internal/app"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/artifacts"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/config"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
	finalexport "github.com/thisxiaoyuQAQ/aipaper-cli/internal/export"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/materials"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/references"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/search"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/store"
	tuiapp "github.com/thisxiaoyuQAQ/aipaper-cli/internal/tui/app"
	materialstui "github.com/thisxiaoyuQAQ/aipaper-cli/internal/tui/materials"
	searchtui "github.com/thisxiaoyuQAQ/aipaper-cli/internal/tui/search"
)

func TestTUIFullFlowMockRuntimeContract(t *testing.T) {
	workDir := t.TempDir()
	s := store.New(workDir)
	writeTUIProjectConfig(t, workDir)
	if _, err := runtimeapp.Bootstrap(workDir, mockTUIConfig()); err != nil {
		t.Fatalf("Bootstrap() error = %v", err)
	}
	assertSuggestedScreen(t, workDir, tuiapp.ScreenRequirements)

	materialDir := filepath.Join(workDir, "materials")
	if err := os.MkdirAll(materialDir, 0o755); err != nil {
		t.Fatalf("create materials dir: %v", err)
	}
	writeFile(t, filepath.Join(materialDir, "refs.bib"), `@article{smith2024rag,
  title={Retrieval Augmented Generation Reviews},
  author={Smith and Lee},
  year={2024},
  doi={10.1000/rag}
}`)
	writeFile(t, filepath.Join(materialDir, "notes.md"), "# Notes\n\nConfirmed references should constrain AI literature reviews.\n")

	req := contracts.Requirements{
		Topic:             "Retrieval augmented generation for literature review writing",
		ResearchQuestions: []string{"How should confirmed references constrain generated reviews?"},
		Scope:             "mini review",
		Language:          "en",
		CitationStyle:     "apa",
		TargetWords:       800,
		MaterialDir:       "./materials",
		AllowOnlineSearch: true,
		SearchProviders:   []string{"mock_search"},
	}
	if _, err := store.WriteJSON(s.RequirementsPath(), req, store.Overwrite); err != nil {
		t.Fatalf("write requirements: %v", err)
	}
	assertSuggestedScreen(t, workDir, tuiapp.ScreenMaterialsScan)

	materialResult, err := materials.ProcessDir(materialDir, s)
	if err != nil {
		t.Fatalf("ProcessDir() error = %v", err)
	}
	if len(materialResult.Candidates) != 1 || materialResult.Candidates[0].Source != "bibtex" {
		t.Fatalf("material candidates = %#v, want one bibtex candidate", materialResult.Candidates)
	}

	searchResult, err := search.Run(context.Background(), s, search.Options{
		Requirements: req,
		Providers:    []search.Provider{mockProvider{}},
		Limit:        2,
	})
	if err != nil {
		t.Fatalf("search.Run() error = %v", err)
	}
	combinedCandidates := combineCandidates(materialResult.Candidates, searchResult.Candidates.Items)
	if len(combinedCandidates.Items) != 2 {
		t.Fatalf("combined candidates = %#v, want material + search candidates", combinedCandidates.Items)
	}
	if combinedCandidates.Items[0].Source != "bibtex" {
		t.Fatalf("first combined candidate source = %q, want bibtex retained before search", combinedCandidates.Items[0].Source)
	}
	if _, err := store.WriteJSON(s.Path("references", "candidates.json"), combinedCandidates, store.Overwrite); err != nil {
		t.Fatalf("write combined candidates: %v", err)
	}
	assertSuggestedScreen(t, workDir, tuiapp.ScreenReferences)

	_, err = references.ConfirmCandidates(s, combinedCandidates, references.ConfirmationDecision{ConfirmedAt: fixedTime()})
	if !references.IsNoneConfirmed(err) {
		t.Fatalf("ConfirmCandidates(no ids) error = %v, want none-confirmed guard", err)
	}

	confirmed := confirmAllReferences(t, s, combinedCandidates)
	if len(confirmed.Items) != 2 {
		t.Fatalf("confirmed = %#v, want two references", confirmed.Items)
	}
	assertSuggestedScreen(t, workDir, tuiapp.ScreenWriting)

	writeAcceptedMockChapter(t, s, confirmed.Items[0].Key)
	assertSuggestedScreen(t, workDir, tuiapp.ScreenExportSummary)

	exportInput, err := finalexport.LoadInput(s)
	if err != nil {
		t.Fatalf("LoadInput() error = %v", err)
	}
	exportResult, err := finalexport.ExportFinal(s, exportInput, finalexport.Options{Now: fixedTime()})
	if err != nil {
		t.Fatalf("ExportFinal() error = %v", err)
	}
	if !exportResult.DocxWritten || len(exportResult.Outputs) != 5 {
		t.Fatalf("export result = %#v", exportResult)
	}
	assertFinalArtifacts(t, s, confirmed)
	assertSuggestedScreen(t, workDir, tuiapp.ScreenDone)
}

func TestTUIRootSearchProgressTransitionsToReferences(t *testing.T) {
	workDir := t.TempDir()
	s := store.New(workDir)
	req := contracts.Requirements{
		Topic:             "RAG reviews",
		Language:          "en",
		CitationStyle:     "apa",
		TargetWords:       800,
		MaterialDir:       workDir,
		AllowOnlineSearch: true,
		SearchProviders:   []string{"mock_search"},
	}
	if _, err := store.WriteJSON(s.RequirementsPath(), req, store.Overwrite); err != nil {
		t.Fatalf("write requirements: %v", err)
	}

	materialCandidate := contracts.ReferenceCandidate{
		ID:     "cand_001",
		Title:  "Material BibTeX Reference",
		Source: "bibtex",
		Status: "pending",
	}
	searchCandidate := contracts.ReferenceCandidate{
		Title:  "Search Reference",
		Source: "mock_search",
		Status: "pending",
	}

	root := tuiapp.NewRootModel(tuiapp.RootOptions{WorkDir: workDir, InitialScreen: tuiapp.ScreenRequirements})
	root.CurrentScreen = tuiapp.ScreenSearchProgress
	root.Search = searchtui.NewModel(searchtui.Options{
		WorkDir:      workDir,
		Store:        s,
		Requirements: req,
		MaterialsResult: materialstui.ScanResult{
			MaterialDir: workDir,
			Candidates:  []contracts.ReferenceCandidate{materialCandidate},
		},
		Search: func(context.Context, store.Store, search.Options) (search.Result, error) {
			return search.Result{Candidates: contracts.ReferenceCandidates{Items: []contracts.ReferenceCandidate{searchCandidate}}}, nil
		},
	})

	msg := root.Search.Init()()
	updated, _ := root.Update(msg)
	root = updated.(tuiapp.RootModel)
	if root.Search.Status() != searchtui.StatusComplete {
		t.Fatalf("Search.Status() = %q, want complete; view:\n%s", root.Search.Status(), root.View())
	}

	updated, cmd := root.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Fatalf("cmd = %v, want nil", cmd)
	}
	root = updated.(tuiapp.RootModel)
	if root.CurrentScreen != tuiapp.ScreenReferences {
		t.Fatalf("CurrentScreen = %q, want references", root.CurrentScreen)
	}

	var written contracts.ReferenceCandidates
	if err := store.ReadJSON(s.Path("references", "candidates.json"), &written); err != nil {
		t.Fatalf("read written candidates: %v", err)
	}
	if len(written.Items) != 2 {
		t.Fatalf("written candidates = %#v, want material + search", written.Items)
	}
	if written.Items[0].Source != "bibtex" || written.Items[1].Source != "mock_search" {
		t.Fatalf("candidate sources = %#v, want bibtex then mock_search", written.Items)
	}
}

func TestTUIConfigWizardOutputIsRuntimeResolvable(t *testing.T) {
	workDir := t.TempDir()
	t.Setenv("OPENAI_API_KEY", "test-runtime-key")
	writeTUIProjectConfig(t, workDir)

	cfg, loaded, err := config.Load(config.LoadOptions{WorkDir: workDir})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(loaded) == 0 {
		t.Fatalf("loaded config paths = %#v, want project config", loaded)
	}
	if cfg.Provider != "default" || cfg.Model != "mock-model" {
		t.Fatalf("top-level provider/model = %q/%q", cfg.Provider, cfg.Model)
	}
	for _, role := range []string{"coordinator", "architect", "writer", "editor"} {
		roleCfg, ok := cfg.Roles[role]
		if !ok {
			t.Fatalf("missing role mapping %q", role)
		}
		if roleCfg.Provider != "default" || roleCfg.Model != "mock-model" {
			t.Fatalf("role %s = %#v, want default/mock-model", role, roleCfg)
		}
		runtimeCfg, err := runtimeapp.ResolveRoleRuntime(cfg, role)
		if err != nil {
			t.Fatalf("ResolveRoleRuntime(%s) error = %v", role, err)
		}
		if runtimeCfg.Provider != "default" || runtimeCfg.Model != "mock-model" || runtimeCfg.APIKey != "test-runtime-key" {
			t.Fatalf("runtime role %s = %#v, want resolved provider/model/api key", role, runtimeCfg)
		}
	}

	probe, err := tuiapp.StateProbe(workDir)
	if err != nil {
		t.Fatalf("StateProbe() error = %v", err)
	}
	if !probe.ConfigOK || probe.ConfigError != nil {
		t.Fatalf("probe config ok=%v error=%v", probe.ConfigOK, probe.ConfigError)
	}
}

func TestTUIUserDocsMentionWindowsAndSmokeScope(t *testing.T) {
	userGuide, err := os.ReadFile(filepath.Join("..", "..", "docs", "user-guide.md"))
	if err != nil {
		t.Fatalf("read user guide: %v", err)
	}
	text := string(userGuide)
	for _, want := range []string{
		"aipaper-cli.exe",
		"go build -o aipaper-cli.exe ./cmd/aipaper-cli",
		"output/aipaper/",
		"final/paper.md",
		"references/confirmed.bib",
		"Ctrl+C",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("docs/user-guide.md missing %q", want)
		}
	}
}

func writeTUIProjectConfig(t *testing.T, workDir string) {
	t.Helper()
	if _, err := config.SaveProject(workDir, mockTUIConfig()); err != nil {
		t.Fatalf("SaveProject() error = %v", err)
	}
}

func mockTUIConfig() config.Config {
	return config.Config{
		Provider: "default",
		Model:    "mock-model",
		Providers: map[string]config.ProviderConfig{
			"default": {
				Type:    "openai",
				APIKey:  "env:OPENAI_API_KEY",
				BaseURL: "http://localhost:8080/v1",
				Models:  []string{"mock-model"},
			},
		},
		Roles: map[string]config.RoleConfig{
			"coordinator": {Provider: "default", Model: "mock-model"},
			"architect":   {Provider: "default", Model: "mock-model"},
			"writer":      {Provider: "default", Model: "mock-model"},
			"editor":      {Provider: "default", Model: "mock-model"},
		},
	}
}

func writeAcceptedMockChapter(t *testing.T, s store.Store, referenceKey string) {
	t.Helper()
	writeOutline(t, s)
	state, err := artifacts.NewChapterState("ch01")
	if err != nil {
		t.Fatalf("NewChapterState() error = %v", err)
	}
	state, err = state.AfterDraft(1)
	if err != nil {
		t.Fatalf("AfterDraft() error = %v", err)
	}
	if _, err := artifacts.WriteDraftBundle(s, mockDraftBundle("ch01", 1, referenceKey, "The revised version from the mock TUI runtime writes from confirmed references.")); err != nil {
		t.Fatalf("WriteDraftBundle() error = %v", err)
	}
	review := mockReview("ch01", 1, true, 86, 94)
	if _, err := artifacts.WriteReview(s, review, "Accepted by mock editor."); err != nil {
		t.Fatalf("WriteReview() error = %v", err)
	}
	state, gate, err := state.AfterReview(review)
	if err != nil {
		t.Fatalf("AfterReview() error = %v", err)
	}
	if !gate.Passed {
		t.Fatalf("gate = %#v, want passed", gate)
	}
	if _, err := artifacts.CommitAccepted(s, "ch01", 1, review); err != nil {
		t.Fatalf("CommitAccepted() error = %v", err)
	}
	if _, err := state.Commit(); err != nil {
		t.Fatalf("Commit state error = %v", err)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create parent dir for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func assertSuggestedScreen(t *testing.T, workDir string, want tuiapp.Screen) {
	t.Helper()
	probe, err := tuiapp.StateProbe(workDir)
	if err != nil {
		t.Fatalf("StateProbe() error = %v", err)
	}
	if probe.SuggestedScreen != want {
		t.Fatalf("SuggestedScreen = %q, want %q; probe=%#v", probe.SuggestedScreen, want, probe)
	}
}
