package search

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/store"
)

type providerFunc struct {
	name string
	fn   func(context.Context, Query) ([]contracts.ReferenceCandidate, error)
}

func (p providerFunc) Name() string { return p.name }

func (p providerFunc) Search(ctx context.Context, query Query) ([]contracts.ReferenceCandidate, error) {
	return p.fn(ctx, query)
}

func TestRunSearchesDedupesAndWritesCandidates(t *testing.T) {
	s := store.NewAt(filepath.Join(t.TempDir(), "store"))
	req := contracts.Requirements{
		Topic:             "retrieval augmented generation",
		ResearchQuestions: []string{"literature review"},
		AllowOnlineSearch: true,
	}
	providers := []Provider{
		providerFunc{name: "semantic_scholar", fn: func(context.Context, Query) ([]contracts.ReferenceCandidate, error) {
			return []contracts.ReferenceCandidate{{
				Title:   "RAG for Review Writing",
				Authors: []string{"Smith"},
				Year:    2024,
				Source:  "semantic_scholar",
				DOI:     "10.1000/rag",
			}}, nil
		}},
		providerFunc{name: "crossref", fn: func(context.Context, Query) ([]contracts.ReferenceCandidate, error) {
			return []contracts.ReferenceCandidate{{
				Title:    "RAG for Review Writing",
				Authors:  []string{"Smith"},
				Year:     2024,
				Source:   "crossref",
				DOI:      "https://doi.org/10.1000/RAG",
				Abstract: "more complete",
			}}, nil
		}},
		providerFunc{name: "arxiv", fn: func(context.Context, Query) ([]contracts.ReferenceCandidate, error) {
			return nil, ProviderError{Code: CodeSearchRateLimited, Source: "arxiv", Message: "rate limited", Retryable: true}
		}},
	}

	result, err := Run(context.Background(), s, Options{Requirements: req, Providers: providers})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(result.Candidates.Items) != 1 {
		t.Fatalf("candidates = %#v", result.Candidates.Items)
	}
	candidate := result.Candidates.Items[0]
	if candidate.ID != "cand_001" || candidate.Abstract != "more complete" || candidate.DedupeGroup != "doi:10.1000/rag" {
		t.Fatalf("candidate = %#v", candidate)
	}
	if len(result.Errors) != 1 || result.Errors[0].Code != CodeSearchRateLimited {
		t.Fatalf("errors = %#v", result.Errors)
	}

	var written contracts.ReferenceCandidates
	if err := store.ReadJSON(s.Path("references", "candidates.json"), &written); err != nil {
		t.Fatalf("read candidates.json: %v", err)
	}
	if len(written.Items) != 1 || written.Items[0].ID != "cand_001" {
		t.Fatalf("written = %#v", written)
	}
	md := readText(t, s.Path("references", "candidates.md"))
	if !strings.Contains(md, "RAG for Review Writing") {
		t.Fatalf("candidates.md = %s", md)
	}
}

func TestRunExpandsWhenCandidateCountBelowTarget(t *testing.T) {
	s := store.NewAt(filepath.Join(t.TempDir(), "store"))
	req := contracts.Requirements{
		Topic:              "genshin impact marketing",
		ResearchQuestions:  []string{"How do player reviews shape marketing?"},
		ChapterPreferences: []string{"wuthering waves player feedback"},
		AllowOnlineSearch:  true,
	}
	var queries []string
	provider := providerFunc{name: "semantic_scholar", fn: func(_ context.Context, query Query) ([]contracts.ReferenceCandidate, error) {
		queries = append(queries, query.Text)
		if len(queries) == 1 {
			return []contracts.ReferenceCandidate{{Title: "Initial Paper", Authors: []string{"A"}, Year: 2024, Source: "semantic_scholar"}}, nil
		}
		return []contracts.ReferenceCandidate{{Title: "Expanded Paper", Authors: []string{"B"}, Year: 2025, Source: "semantic_scholar"}}, nil
	}}

	result, err := Run(context.Background(), s, Options{
		Requirements:      req,
		Providers:         []Provider{provider},
		Limit:             1,
		ExpansionEnabled:  true,
		MinCandidateCount: 2,
		ExpansionLimit:    1,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(queries) != 2 {
		t.Fatalf("queries = %#v", queries)
	}
	if len(result.Candidates.Items) != 2 {
		t.Fatalf("candidates = %#v", result.Candidates.Items)
	}
	if result.Candidates.Items[1].ExpansionSource == "" || !strings.Contains(result.Candidates.Items[1].RelevanceReason, "Expanded from query") {
		t.Fatalf("expanded candidate = %#v", result.Candidates.Items[1])
	}
}

func TestRunWritesEmptyCandidatesWhenOnlineSearchDisabled(t *testing.T) {
	s := store.NewAt(filepath.Join(t.TempDir(), "store"))
	result, err := Run(context.Background(), s, Options{
		Requirements: contracts.Requirements{Topic: "ignored", AllowOnlineSearch: false},
		Providers: []Provider{providerFunc{name: "unused", fn: func(context.Context, Query) ([]contracts.ReferenceCandidate, error) {
			t.Fatalf("provider should not be called")
			return nil, nil
		}}},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(result.Candidates.Items) != 0 || len(result.Errors) != 0 {
		t.Fatalf("result = %#v", result)
	}
	if !strings.Contains(readText(t, s.Path("references", "candidates.md")), "No reference candidates") {
		t.Fatalf("empty candidates.md missing message")
	}
}

func TestRunReturnsStructuredEmptyResult(t *testing.T) {
	result, err := Run(context.Background(), store.NewAt(filepath.Join(t.TempDir(), "store")), Options{
		Requirements: contracts.Requirements{Topic: "no hits", AllowOnlineSearch: true},
		Providers: []Provider{providerFunc{name: "empty", fn: func(context.Context, Query) ([]contracts.ReferenceCandidate, error) {
			return nil, nil
		}}},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(result.Errors) != 1 || result.Errors[0].Code != CodeCandidatesEmpty {
		t.Fatalf("errors = %#v", result.Errors)
	}
}

func TestSemanticScholarProviderParsesResponseAndRateLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("query") == "limited" {
			http.Error(w, "slow down", http.StatusTooManyRequests)
			return
		}
		writeJSON(t, w, map[string]any{"data": []map[string]any{{
			"title":         "Semantic Scholar Paper",
			"year":          2024,
			"abstract":      "abstract",
			"venue":         "Venue",
			"citationCount": 5,
			"url":           "https://example.org/semantic",
			"externalIds":   map[string]any{"DOI": "10.1000/semantic"},
			"authors":       []map[string]any{{"name": "Ada"}},
		}}})
	}))
	defer server.Close()

	provider := NewSemanticScholarProvider(HTTPProviderConfig{BaseURL: server.URL, Client: server.Client()})
	candidates, err := provider.Search(context.Background(), Query{Text: "rag", Limit: 1})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(candidates) != 1 || candidates[0].Source != SemanticScholarProviderName || candidates[0].DOI != "10.1000/semantic" {
		t.Fatalf("candidates = %#v", candidates)
	}

	_, err = provider.Search(context.Background(), Query{Text: "limited", Limit: 1})
	var providerErr ProviderError
	if !errors.As(err, &providerErr) || providerErr.Code != CodeSearchRateLimited || !providerErr.Retryable {
		t.Fatalf("rate limit error = %v", err)
	}
}

func TestProviderMapsCanceledContextToTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("server should not be reached for canceled context")
	}))
	defer server.Close()

	provider := NewSemanticScholarProvider(HTTPProviderConfig{BaseURL: server.URL, Client: server.Client()})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := provider.Search(ctx, Query{Text: "rag", Limit: 1})
	var providerErr ProviderError
	if !errors.As(err, &providerErr) || providerErr.Code != CodeSearchTimeout || !providerErr.Retryable {
		t.Fatalf("timeout error = %v", err)
	}
}

func TestCrossrefProviderParsesResponseAndRejectsMissingTitle(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("query") == "missing" {
			writeJSON(t, w, map[string]any{"message": map[string]any{"items": []map[string]any{{"DOI": "10.1000/missing"}}}})
			return
		}
		writeJSON(t, w, map[string]any{"message": map[string]any{"items": []map[string]any{{
			"DOI":                    "10.1000/crossref",
			"title":                  []string{"Crossref Paper"},
			"abstract":               "<jats:p>abstract</jats:p>",
			"URL":                    "https://example.org/crossref",
			"container-title":        []string{"Journal"},
			"is-referenced-by-count": 3,
			"author":                 []map[string]any{{"given": "Ada", "family": "Lovelace"}},
			"issued":                 map[string]any{"date-parts": [][]int{{2023, 1, 2}}},
		}}}})
	}))
	defer server.Close()

	provider := NewCrossrefProvider(HTTPProviderConfig{BaseURL: server.URL, Client: server.Client()})
	candidates, err := provider.Search(context.Background(), Query{Text: "rag", Limit: 1})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if got := candidates[0]; got.Title != "Crossref Paper" || got.Year != 2023 || got.Abstract != "abstract" {
		t.Fatalf("candidate = %#v", got)
	}

	_, err = provider.Search(context.Background(), Query{Text: "missing", Limit: 1})
	var providerErr ProviderError
	if !errors.As(err, &providerErr) || providerErr.Code != CodeSearchFieldMissing {
		t.Fatalf("missing title error = %v", err)
	}
}

func TestArxivProviderParsesAtomFeed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/atom+xml")
		_, _ = w.Write([]byte(`<?xml version="1.0"?>
<feed xmlns="http://www.w3.org/2005/Atom" xmlns:arxiv="http://arxiv.org/schemas/atom">
  <entry>
    <id>https://arxiv.org/abs/2401.00001</id>
    <title>Arxiv Paper</title>
    <summary>Arxiv abstract</summary>
    <published>2024-01-01T00:00:00Z</published>
    <author><name>Ada Lovelace</name></author>
    <link rel="alternate" href="https://arxiv.org/abs/2401.00001"/>
    <arxiv:doi>10.1000/arxiv</arxiv:doi>
  </entry>
</feed>`))
	}))
	defer server.Close()

	provider := NewArxivProvider(HTTPProviderConfig{BaseURL: server.URL, Client: server.Client()})
	candidates, err := provider.Search(context.Background(), Query{Text: "rag", Limit: 1})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if got := candidates[0]; got.Title != "Arxiv Paper" || got.Year != 2024 || got.Venue != "arXiv" {
		t.Fatalf("candidate = %#v", got)
	}
}

func TestPubMedProviderParsesESearchAndESummary(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/esearch.fcgi":
			writeJSON(t, w, map[string]any{"esearchresult": map[string]any{"idlist": []string{"123"}}})
		case "/esummary.fcgi":
			writeJSON(t, w, map[string]any{"result": map[string]any{
				"uids": []string{"123"},
				"123": map[string]any{
					"uid":             "123",
					"title":           "PubMed Paper",
					"fulljournalname": "Journal",
					"pubdate":         "2022 Jan",
					"authors":         []map[string]any{{"name": "Ada Lovelace"}},
					"articleids":      []map[string]any{{"idtype": "doi", "value": "10.1000/pubmed"}},
				},
			}})
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	provider := NewPubMedProvider(HTTPProviderConfig{BaseURL: server.URL, Client: server.Client()})
	candidates, err := provider.Search(context.Background(), Query{Text: "rag", Limit: 1})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if got := candidates[0]; got.Title != "PubMed Paper" || got.Year != 2022 || got.DOI != "10.1000/pubmed" {
		t.Fatalf("candidate = %#v", got)
	}
}

func TestOpenAlexProviderParsesResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, map[string]any{"results": []map[string]any{{
			"id":               "https://openalex.org/W123",
			"display_name":     "OpenAlex Paper",
			"publication_year": 2024,
			"doi":              "https://doi.org/10.1000/openalex",
			"cited_by_count":   7,
			"abstract_inverted_index": map[string]any{
				"This": []int{0}, "abstract": []int{2}, "is": []int{1},
			},
			"authorships": []map[string]any{{"author": map[string]any{"display_name": "Ada"}}},
			"primary_location": map[string]any{
				"landing_page_url": "https://example.org/openalex",
				"source":           map[string]any{"display_name": "Journal"},
			},
			"open_access": map[string]any{"is_oa": true, "oa_url": "https://example.org/fulltext"},
		}}})
	}))
	defer server.Close()

	provider := NewOpenAlexProvider(HTTPProviderConfig{BaseURL: server.URL, Client: server.Client()})
	candidates, err := provider.Search(context.Background(), Query{Text: "rag", Limit: 1})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if got := candidates[0]; got.Source != OpenAlexProviderName || got.Title != "OpenAlex Paper" || got.Abstract != "This is abstract" || got.Availability != "open_access" || got.AccessURL != "https://example.org/fulltext" {
		t.Fatalf("candidate = %#v", got)
	}
}

func TestDOAJProviderParsesResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, map[string]any{"results": []map[string]any{{
			"id": "doaj-1",
			"bibjson": map[string]any{
				"title":      "DOAJ Paper",
				"abstract":   "<p>OA abstract</p>",
				"year":       "2023",
				"author":     []map[string]any{{"name": "Grace"}},
				"identifier": []map[string]any{{"type": "doi", "id": "10.1000/doaj"}},
				"link":       []map[string]any{{"type": "fulltext", "url": "https://example.org/doaj-full"}},
				"journal":    map[string]any{"title": "OA Journal"},
			},
		}}})
	}))
	defer server.Close()

	provider := NewDOAJProvider(HTTPProviderConfig{BaseURL: server.URL, Client: server.Client()})
	candidates, err := provider.Search(context.Background(), Query{Text: "rag", Limit: 1})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if got := candidates[0]; got.Source != DOAJProviderName || got.Title != "DOAJ Paper" || got.Year != 2023 || got.Availability != "open_access" || got.DOI != "10.1000/doaj" {
		t.Fatalf("candidate = %#v", got)
	}
}

func TestDefaultProvidersIncludesOpenAlexAndDOAJ(t *testing.T) {
	providers := DefaultProviders(HTTPProviderConfig{})
	var names []string
	for _, provider := range providers {
		names = append(names, provider.Name())
	}
	joined := strings.Join(names, ",")
	for _, want := range []string{OpenAlexProviderName, DOAJProviderName} {
		if !strings.Contains(joined, want) {
			t.Fatalf("providers = %#v, missing %s", names, want)
		}
	}
}

func TestRunSkipsRateLimitedProviderDuringExpansion(t *testing.T) {
	s := store.NewAt(filepath.Join(t.TempDir(), "store"))
	req := contracts.Requirements{
		Topic:             "domestic literature search",
		AllowOnlineSearch: true,
	}

	limitedCalls := 0
	fallbackCalls := 0
	providers := []Provider{
		providerFunc{name: "semantic_scholar", fn: func(context.Context, Query) ([]contracts.ReferenceCandidate, error) {
			limitedCalls++
			return nil, ProviderError{Code: CodeSearchRateLimited, Source: "semantic_scholar", Message: "rate limited", Retryable: true}
		}},
		providerFunc{name: "openalex", fn: func(context.Context, Query) ([]contracts.ReferenceCandidate, error) {
			fallbackCalls++
			if fallbackCalls == 1 {
				return []contracts.ReferenceCandidate{{Title: "Initial", Authors: []string{"A"}, Source: "openalex"}}, nil
			}
			return []contracts.ReferenceCandidate{{Title: "Expanded", Authors: []string{"B"}, Source: "openalex"}}, nil
		}},
	}

	result, err := Run(context.Background(), s, Options{
		Requirements:      req,
		Providers:         providers,
		Limit:             1,
		ExpansionEnabled:  true,
		MinCandidateCount: 2,
		ExpansionLimit:    1,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if limitedCalls != 1 {
		t.Fatalf("rate-limited provider calls = %d, want 1", limitedCalls)
	}
	if fallbackCalls != 2 {
		t.Fatalf("fallback provider calls = %d, want 2", fallbackCalls)
	}
	if len(result.Errors) != 1 || result.Errors[0].Code != CodeSearchRateLimited {
		t.Fatalf("errors = %#v", result.Errors)
	}
	if len(result.Candidates.Items) != 2 {
		t.Fatalf("candidates = %#v", result.Candidates.Items)
	}
}

func TestRunDeduplicatesRepeatedProviderErrors(t *testing.T) {
	s := store.NewAt(filepath.Join(t.TempDir(), "store"))
	req := contracts.Requirements{Topic: "empty", AllowOnlineSearch: true}
	provider := providerFunc{name: "semantic_scholar", fn: func(context.Context, Query) ([]contracts.ReferenceCandidate, error) {
		return nil, ProviderError{Code: CodeSearchRateLimited, Source: "semantic_scholar", Message: "rate limited", Retryable: true}
	}}

	result, err := Run(context.Background(), s, Options{
		Requirements:      req,
		Providers:         []Provider{provider},
		Limit:             1,
		ExpansionEnabled:  true,
		MinCandidateCount: 2,
		ExpansionLimit:    2,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(result.Errors) != 1 {
		t.Fatalf("errors = %#v", result.Errors)
	}
}

func writeJSON(t *testing.T, w http.ResponseWriter, value any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
}

func readText(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	return string(data)
}
