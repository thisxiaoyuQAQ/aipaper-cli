package search

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/references"
	domainsearch "github.com/thisxiaoyuQAQ/aipaper-cli/internal/search"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/store"
	materialstui "github.com/thisxiaoyuQAQ/aipaper-cli/internal/tui/materials"
)

func TestSearchProgress_UsesExpandedReferenceSearchDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	s := store.New(tmpDir)
	if err := store.EnsureLayout(s); err != nil {
		t.Fatal(err)
	}

	var captured domainsearch.Options
	searchFn := func(ctx context.Context, s store.Store, opts domainsearch.Options) (domainsearch.Result, error) {
		captured = opts
		return domainsearch.Result{}, nil
	}

	m := NewModel(Options{
		WorkDir: tmpDir,
		Store:   s,
		Requirements: contracts.Requirements{
			AllowOnlineSearch: true,
		},
		Search: searchFn,
	})
	_ = m.searchCmd()()

	if captured.Limit != 20 || captured.MinCandidateCount != 30 || captured.ExpansionLimit != 5 || !captured.ExpansionEnabled {
		t.Fatalf("search options = %#v", captured)
	}
}

func TestSearchProgress_SearchDisabled_MaterialCandidatesRetained(t *testing.T) {
	tmpDir := t.TempDir()
	s := store.New(tmpDir)
	if err := store.EnsureLayout(s); err != nil {
		t.Fatal(err)
	}

	materialCandidates := []contracts.ReferenceCandidate{
		{Title: "Material Paper 1", Authors: []string{"Author A"}, Source: "bibtex", Status: "pending"},
		{Title: "Material Paper 2", Authors: []string{"Author B"}, Source: "bibtex", Status: "pending"},
	}

	searchFn := func(ctx context.Context, s store.Store, opts domainsearch.Options) (domainsearch.Result, error) {
		return domainsearch.Result{
			Candidates: contracts.ReferenceCandidates{Items: []contracts.ReferenceCandidate{}},
		}, nil
	}

	m := NewModel(Options{
		WorkDir: tmpDir,
		Store:   s,
		Requirements: contracts.Requirements{
			AllowOnlineSearch: false,
		},
		MaterialsResult: materialstui.ScanResult{
			Candidates: materialCandidates,
		},
		Search: searchFn,
	})

	msg := m.searchCmd()()
	searchMsg, ok := msg.(SearchFinishedMsg)
	if !ok {
		t.Fatalf("expected SearchFinishedMsg, got %T", msg)
	}

	m.applySearchFinished(searchMsg)

	if m.status != StatusDisabled {
		t.Errorf("expected status %s, got %s", StatusDisabled, m.status)
	}
	if len(m.finalCandidates) != 2 {
		t.Errorf("expected 2 final candidates, got %d", len(m.finalCandidates))
	}
	if m.finalCandidates[0].ID != "cand_001" {
		t.Errorf("expected first candidate ID cand_001, got %s", m.finalCandidates[0].ID)
	}
	if m.finalCandidates[1].ID != "cand_002" {
		t.Errorf("expected second candidate ID cand_002, got %s", m.finalCandidates[1].ID)
	}
}

func TestSearchProgress_SearchEnabled_BothCandidatesRetained(t *testing.T) {
	tmpDir := t.TempDir()
	s := store.New(tmpDir)
	if err := store.EnsureLayout(s); err != nil {
		t.Fatal(err)
	}

	materialCandidates := []contracts.ReferenceCandidate{
		{Title: "Material Paper", Authors: []string{"Author A"}, Source: "bibtex", Status: "pending"},
	}

	searchFn := func(ctx context.Context, s store.Store, opts domainsearch.Options) (domainsearch.Result, error) {
		return domainsearch.Result{
			Candidates: contracts.ReferenceCandidates{Items: []contracts.ReferenceCandidate{
				{Title: "Search Paper 1", Authors: []string{"Author B"}, Source: "semantic_scholar", Status: "pending"},
				{Title: "Search Paper 2", Authors: []string{"Author C"}, Source: "crossref", Status: "pending"},
			}},
		}, nil
	}

	m := NewModel(Options{
		WorkDir: tmpDir,
		Store:   s,
		Requirements: contracts.Requirements{
			AllowOnlineSearch: true,
		},
		MaterialsResult: materialstui.ScanResult{
			Candidates: materialCandidates,
		},
		Search: searchFn,
	})

	msg := m.searchCmd()()
	searchMsg := msg.(SearchFinishedMsg)
	m.applySearchFinished(searchMsg)

	if m.status != StatusComplete {
		t.Errorf("expected status %s, got %s", StatusComplete, m.status)
	}
	if m.materialCount != 1 {
		t.Errorf("expected 1 material candidate, got %d", m.materialCount)
	}
	if m.searchCount != 2 {
		t.Errorf("expected 2 search candidates, got %d", m.searchCount)
	}
	if len(m.finalCandidates) != 3 {
		t.Errorf("expected 3 final candidates, got %d", len(m.finalCandidates))
	}
}

func TestSearchProgress_Deduplication_DOI(t *testing.T) {
	tmpDir := t.TempDir()
	s := store.New(tmpDir)
	if err := store.EnsureLayout(s); err != nil {
		t.Fatal(err)
	}

	materialCandidates := []contracts.ReferenceCandidate{
		{Title: "Paper Title", Authors: []string{"Author A"}, DOI: "10.1234/abc", Source: "bibtex", Status: "pending"},
	}

	searchFn := func(ctx context.Context, s store.Store, opts domainsearch.Options) (domainsearch.Result, error) {
		return domainsearch.Result{
			Candidates: contracts.ReferenceCandidates{Items: []contracts.ReferenceCandidate{
				{
					Title:    "Paper Title",
					Authors:  []string{"Author A"},
					DOI:      "10.1234/abc",
					Abstract: "This is the abstract",
					Source:   "semantic_scholar",
					Status:   "pending",
				},
			}},
		}, nil
	}

	m := NewModel(Options{
		WorkDir: tmpDir,
		Store:   s,
		Requirements: contracts.Requirements{
			AllowOnlineSearch: true,
		},
		MaterialsResult: materialstui.ScanResult{
			Candidates: materialCandidates,
		},
		Search: searchFn,
	})

	msg := m.searchCmd()()
	searchMsg := msg.(SearchFinishedMsg)
	m.applySearchFinished(searchMsg)

	if len(m.finalCandidates) != 1 {
		t.Errorf("expected 1 final candidate after deduplication, got %d", len(m.finalCandidates))
	}
	if m.finalCandidates[0].Abstract == "" {
		t.Error("expected merged candidate to have abstract from search result")
	}
	if m.finalCandidates[0].ID != "cand_001" {
		t.Errorf("expected ID cand_001, got %s", m.finalCandidates[0].ID)
	}
}

func TestSearchProgress_Deduplication_URL(t *testing.T) {
	tmpDir := t.TempDir()
	s := store.New(tmpDir)
	if err := store.EnsureLayout(s); err != nil {
		t.Fatal(err)
	}

	materialCandidates := []contracts.ReferenceCandidate{
		{Title: "Paper Title", Authors: []string{"Author A"}, URL: "https://example.com/paper", Source: "bibtex", Status: "pending"},
	}

	searchFn := func(ctx context.Context, s store.Store, opts domainsearch.Options) (domainsearch.Result, error) {
		return domainsearch.Result{
			Candidates: contracts.ReferenceCandidates{Items: []contracts.ReferenceCandidate{
				{
					Title:   "Paper Title Extended",
					Authors: []string{"Author A", "Author B"},
					URL:     "https://example.com/paper",
					Venue:   "Conference 2023",
					Source:  "crossref",
					Status:  "pending",
				},
			}},
		}, nil
	}

	m := NewModel(Options{
		WorkDir: tmpDir,
		Store:   s,
		Requirements: contracts.Requirements{
			AllowOnlineSearch: true,
		},
		MaterialsResult: materialstui.ScanResult{
			Candidates: materialCandidates,
		},
		Search: searchFn,
	})

	msg := m.searchCmd()()
	searchMsg := msg.(SearchFinishedMsg)
	m.applySearchFinished(searchMsg)

	if len(m.finalCandidates) != 1 {
		t.Errorf("expected 1 final candidate after deduplication, got %d", len(m.finalCandidates))
	}
	if m.finalCandidates[0].Venue == "" {
		t.Error("expected merged candidate to have venue from search result")
	}
	if len(m.finalCandidates[0].Authors) < 2 {
		t.Error("expected merged candidate to have more complete authors list")
	}
}

func TestSearchProgress_ProviderPartialFailure_DoesNotBlock(t *testing.T) {
	tmpDir := t.TempDir()
	s := store.New(tmpDir)
	if err := store.EnsureLayout(s); err != nil {
		t.Fatal(err)
	}

	searchFn := func(ctx context.Context, s store.Store, opts domainsearch.Options) (domainsearch.Result, error) {
		return domainsearch.Result{
			Candidates: contracts.ReferenceCandidates{Items: []contracts.ReferenceCandidate{
				{Title: "Paper from provider 1", Authors: []string{"Author A"}, Source: "semantic_scholar", Status: "pending"},
			}},
			Errors: []domainsearch.ProviderError{
				{Code: domainsearch.CodeSearchTimeout, Source: "crossref", Message: "request timeout", Retryable: true},
			},
		}, nil
	}

	m := NewModel(Options{
		WorkDir: tmpDir,
		Store:   s,
		Requirements: contracts.Requirements{
			AllowOnlineSearch: true,
		},
		MaterialsResult: materialstui.ScanResult{
			Candidates: []contracts.ReferenceCandidate{},
		},
		Search: searchFn,
	})

	msg := m.searchCmd()()
	searchMsg := msg.(SearchFinishedMsg)
	m.applySearchFinished(searchMsg)

	if m.status != StatusComplete {
		t.Errorf("expected status %s, got %s", StatusComplete, m.status)
	}
	if len(m.searchErrors) != 1 {
		t.Errorf("expected 1 error, got %d", len(m.searchErrors))
	}
	if len(m.finalCandidates) != 1 {
		t.Errorf("expected 1 final candidate, got %d", len(m.finalCandidates))
	}
	view := m.View()
	for _, want := range []string{"搜索进度", "搜索完成", "本地引用候选：0", "搜索候选：1", "最终候选（去重后）：1", "搜索源提示", "crossref: 请求超时，已继续使用其他来源", "按 Enter 继续"} {
		if !strings.Contains(view, want) {
			t.Fatalf("search view missing %q:\n%s", want, view)
		}
	}
}

func TestSearchProgress_AllProvidersFailed_CanRetrySkipOrBack(t *testing.T) {
	tmpDir := t.TempDir()
	s := store.New(tmpDir)
	if err := store.EnsureLayout(s); err != nil {
		t.Fatal(err)
	}

	searchFn := func(ctx context.Context, s store.Store, opts domainsearch.Options) (domainsearch.Result, error) {
		return domainsearch.Result{
			Candidates: contracts.ReferenceCandidates{Items: []contracts.ReferenceCandidate{}},
			Errors: []domainsearch.ProviderError{
				{Code: "TIMEOUT", Source: "semantic_scholar", Message: "timeout", Retryable: true},
				{Code: "RATE_LIMITED", Source: "crossref", Message: "rate limited", Retryable: true},
			},
		}, nil
	}

	m := NewModel(Options{
		WorkDir: tmpDir,
		Store:   s,
		Requirements: contracts.Requirements{
			AllowOnlineSearch: true,
		},
		MaterialsResult: materialstui.ScanResult{
			Candidates: []contracts.ReferenceCandidate{
				{Title: "Material Paper", Authors: []string{"Author A"}, Source: "bibtex", Status: "pending"},
			},
		},
		Search: searchFn,
	})

	msg := m.searchCmd()()
	searchMsg := msg.(SearchFinishedMsg)
	m.applySearchFinished(searchMsg)

	if m.status != StatusAllFailed {
		t.Errorf("expected status %s, got %s", StatusAllFailed, m.status)
	}
	if len(m.searchErrors) != 2 {
		t.Errorf("expected 2 errors, got %d", len(m.searchErrors))
	}

	// Test retry action
	m, _ = m.UpdateKey("r")
	if m.status != StatusSearching {
		t.Errorf("expected retry to restart search, got status %s", m.status)
	}

	// Reset and test skip
	m.status = StatusAllFailed
	m, _ = m.UpdateKey("s")
	if m.action != ActionSkip {
		t.Errorf("expected skip action, got %s", m.action)
	}

	// Reset and test back
	m.action = ActionNone
	m, _ = m.UpdateKey("b")
	if m.action != ActionBack {
		t.Errorf("expected back action, got %s", m.action)
	}
}

func TestSearchProgress_SearchDisabled_DoesNotCallSearchProvider(t *testing.T) {
	tmpDir := t.TempDir()
	s := store.New(tmpDir)
	if err := store.EnsureLayout(s); err != nil {
		t.Fatal(err)
	}

	called := false
	m := NewModel(Options{
		WorkDir: tmpDir,
		Store:   s,
		Requirements: contracts.Requirements{
			AllowOnlineSearch: false,
		},
		MaterialsResult: materialstui.ScanResult{
			Candidates: []contracts.ReferenceCandidate{{Title: "Material Paper", Source: "bibtex", Status: "pending"}},
		},
		Search: func(context.Context, store.Store, domainsearch.Options) (domainsearch.Result, error) {
			called = true
			return domainsearch.Result{}, nil
		},
	})

	msg := m.Init()()
	searchMsg, ok := msg.(SearchFinishedMsg)
	if !ok {
		t.Fatalf("expected SearchFinishedMsg, got %T", msg)
	}
	m.applySearchFinished(searchMsg)

	if called {
		t.Fatalf("search provider was called even though online search is disabled")
	}
	if m.status != StatusDisabled || len(m.finalCandidates) != 1 {
		t.Fatalf("status=%q final=%#v, want disabled with material candidate", m.status, m.finalCandidates)
	}
}

func TestSearchProgress_FinalIDsStartFromCand001(t *testing.T) {
	tmpDir := t.TempDir()
	s := store.New(tmpDir)
	if err := store.EnsureLayout(s); err != nil {
		t.Fatal(err)
	}

	materialCandidates := []contracts.ReferenceCandidate{
		{Title: "Paper 1", Authors: []string{"A"}, Source: "bibtex"},
		{Title: "Paper 2", Authors: []string{"B"}, Source: "bibtex"},
	}

	searchFn := func(ctx context.Context, s store.Store, opts domainsearch.Options) (domainsearch.Result, error) {
		return domainsearch.Result{
			Candidates: contracts.ReferenceCandidates{Items: []contracts.ReferenceCandidate{
				{Title: "Paper 3", Authors: []string{"C"}, Source: "semantic_scholar"},
			}},
		}, nil
	}

	m := NewModel(Options{
		WorkDir: tmpDir,
		Store:   s,
		Requirements: contracts.Requirements{
			AllowOnlineSearch: true,
		},
		MaterialsResult: materialstui.ScanResult{
			Candidates: materialCandidates,
		},
		Search: searchFn,
	})

	msg := m.searchCmd()()
	searchMsg := msg.(SearchFinishedMsg)
	m.applySearchFinished(searchMsg)

	if len(m.finalCandidates) != 3 {
		t.Fatalf("expected 3 candidates, got %d", len(m.finalCandidates))
	}
	expectedIDs := []string{"cand_001", "cand_002", "cand_003"}
	for i, candidate := range m.finalCandidates {
		if candidate.ID != expectedIDs[i] {
			t.Errorf("candidate %d: expected ID %s, got %s", i, expectedIDs[i], candidate.ID)
		}
	}
}

func TestSearchProgress_SearchError_CanRetry(t *testing.T) {
	tmpDir := t.TempDir()
	s := store.New(tmpDir)
	if err := store.EnsureLayout(s); err != nil {
		t.Fatal(err)
	}

	searchFn := func(ctx context.Context, s store.Store, opts domainsearch.Options) (domainsearch.Result, error) {
		return domainsearch.Result{}, errors.New("search failed")
	}

	m := NewModel(Options{
		WorkDir: tmpDir,
		Store:   s,
		Requirements: contracts.Requirements{
			AllowOnlineSearch: true,
		},
		MaterialsResult: materialstui.ScanResult{
			Candidates: []contracts.ReferenceCandidate{},
		},
		Search: searchFn,
	})

	msg := m.searchCmd()()
	searchMsg := msg.(SearchFinishedMsg)
	m.applySearchFinished(searchMsg)

	if m.status != StatusError {
		t.Errorf("expected status %s, got %s", StatusError, m.status)
	}
	if m.err == nil {
		t.Error("expected error to be set")
	}

	// Test retry
	m, _ = m.UpdateKey("r")
	if m.status != StatusSearching {
		t.Errorf("expected retry to restart search, got status %s", m.status)
	}
}

func TestSearchProgress_ShowsLocalReferenceHintForParsedMaterials(t *testing.T) {
	tmpDir := t.TempDir()
	s := store.New(tmpDir)
	if err := store.EnsureLayout(s); err != nil {
		t.Fatal(err)
	}

	m := NewModel(Options{
		WorkDir: tmpDir,
		Store:   s,
		Requirements: contracts.Requirements{
			AllowOnlineSearch: true,
		},
		MaterialsResult: materialstui.ScanResult{
			Stats: materialstui.ScanStats{Parsed: 1},
		},
		Search: func(context.Context, store.Store, domainsearch.Options) (domainsearch.Result, error) {
			return domainsearch.Result{
				Candidates: contracts.ReferenceCandidates{Items: []contracts.ReferenceCandidate{
					{Title: "Search Paper", Authors: []string{"Author"}, Source: "openalex", Status: "pending"},
				}},
			}, nil
		},
	})

	msg := m.searchCmd()()
	m.applySearchFinished(msg.(SearchFinishedMsg))
	view := m.View()
	for _, want := range []string{"本地引用候选：0", "只有 BibTeX/RIS/CSV 等文献导出文件会直接生成引用候选"} {
		if !strings.Contains(view, want) {
			t.Fatalf("search view missing %q:\n%s", want, view)
		}
	}
}

func TestSearchProgress_DedupeGroup_SetCorrectly(t *testing.T) {
	tmpDir := t.TempDir()
	s := store.New(tmpDir)
	if err := store.EnsureLayout(s); err != nil {
		t.Fatal(err)
	}

	materialCandidates := []contracts.ReferenceCandidate{
		{Title: "Paper Title", Authors: []string{"Author A"}, DOI: "10.1234/test", Source: "bibtex"},
	}

	searchFn := func(ctx context.Context, s store.Store, opts domainsearch.Options) (domainsearch.Result, error) {
		return domainsearch.Result{
			Candidates: contracts.ReferenceCandidates{Items: []contracts.ReferenceCandidate{}},
		}, nil
	}

	m := NewModel(Options{
		WorkDir: tmpDir,
		Store:   s,
		Requirements: contracts.Requirements{
			AllowOnlineSearch: true,
		},
		MaterialsResult: materialstui.ScanResult{
			Candidates: materialCandidates,
		},
		Search: searchFn,
	})

	msg := m.searchCmd()()
	searchMsg := msg.(SearchFinishedMsg)
	m.applySearchFinished(searchMsg)

	if len(m.finalCandidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(m.finalCandidates))
	}
	expectedGroup := references.CandidateDedupeGroup(materialCandidates[0])
	if m.finalCandidates[0].DedupeGroup != expectedGroup {
		t.Errorf("expected dedupe group %s, got %s", expectedGroup, m.finalCandidates[0].DedupeGroup)
	}
}
