package references

import (
	"testing"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
)

func TestDedupeCandidatesUsesDOIURLAndHashFallback(t *testing.T) {
	input := []contracts.ReferenceCandidate{
		{
			Title:  "RAG for Reviews",
			Source: "semantic_scholar",
			DOI:    "https://doi.org/10.1000/RAG",
			Status: "pending",
		},
		{
			Title:    "RAG for Reviews",
			Authors:  []string{"Smith"},
			Year:     2024,
			Source:   "crossref",
			DOI:      "10.1000/rag",
			Abstract: "more complete",
			Status:   "pending",
		},
		{
			Title:  "URL paper",
			Source: "arxiv",
			URL:    "HTTPS://Example.org/paper/",
			Status: "pending",
		},
		{
			Title:  "URL paper duplicate",
			Source: "pubmed",
			URL:    "https://example.org/paper",
			Status: "pending",
		},
		{
			Title:   "Title Hash Paper!",
			Authors: []string{"Ada Lovelace"},
			Year:    1843,
			Source:  "semantic_scholar",
			Status:  "pending",
		},
		{
			Title:   "title hash paper",
			Authors: []string{"Ada Lovelace"},
			Year:    1843,
			Source:  "crossref",
			Status:  "pending",
		},
	}

	deduped := DedupeCandidates(input)
	if len(deduped) != 3 {
		t.Fatalf("deduped len = %d: %#v", len(deduped), deduped)
	}
	if deduped[0].DedupeGroup != "doi:10.1000/rag" || deduped[0].Abstract != "more complete" {
		t.Fatalf("doi group = %#v", deduped[0])
	}
	if deduped[1].DedupeGroup != "url:https://example.org/paper" {
		t.Fatalf("url group = %#v", deduped[1])
	}
	if got := deduped[2].DedupeGroup; len(got) <= len("hash:") || got[:5] != "hash:" {
		t.Fatalf("hash group = %q", got)
	}
}

func TestAssignCandidateIDs(t *testing.T) {
	candidates := AssignCandidateIDs([]contracts.ReferenceCandidate{{Title: "A"}, {Title: "B"}}, 3)
	if candidates[0].ID != "cand_003" || candidates[1].ID != "cand_004" {
		t.Fatalf("candidates = %#v", candidates)
	}
}
