package references

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/store"
)

func TestConfirmCandidatesWritesConfirmedRejectedAndBibTeX(t *testing.T) {
	s := store.NewAt(filepath.Join(t.TempDir(), "store"))
	candidates := contracts.ReferenceCandidates{Items: []contracts.ReferenceCandidate{
		{
			ID:       "cand_001",
			Title:    "Retrieval Augmented Generation for Review Writing",
			Authors:  []string{"Smith, Jane"},
			Year:     2024,
			DOI:      "10.1000/rag",
			Abstract: "abstract",
			Status:   "pending",
		},
		{
			ID:      "cand_002",
			Title:   "Human in the Loop Review",
			Authors: []string{"Li Wei"},
			Year:    2023,
			URL:     "https://example.org/hitl",
			Status:  "pending",
		},
		{
			ID:      "cand_003",
			Title:   "Rejected Paper",
			Authors: []string{"Ada"},
			Year:    2022,
			Status:  "pending",
		},
	}}
	now := time.Date(2026, 6, 7, 1, 2, 3, 0, time.UTC)

	result, err := ConfirmCandidates(s, candidates, ConfirmationDecision{
		ConfirmedIDs: []string{"cand_001", "cand_002"},
		RejectedIDs:  []string{"cand_003"},
		ConfirmedAt:  now,
	})
	if err != nil {
		t.Fatalf("ConfirmCandidates() error = %v", err)
	}
	if len(result.Confirmed.Items) != 2 || len(result.Rejected.Items) != 1 {
		t.Fatalf("result = %#v", result)
	}
	if result.Confirmed.Items[0].Key != "smith2024RetrievalAugmentedGeneration" {
		t.Fatalf("key = %q", result.Confirmed.Items[0].Key)
	}
	if result.Confirmed.Items[0].ConfirmedAt != now {
		t.Fatalf("confirmed_at = %s", result.Confirmed.Items[0].ConfirmedAt)
	}
	if result.Rejected.Items[0].Status != "rejected" {
		t.Fatalf("rejected = %#v", result.Rejected.Items[0])
	}

	var written contracts.ConfirmedReferences
	if err := store.ReadJSON(s.Path("references", "confirmed.json"), &written); err != nil {
		t.Fatalf("read confirmed.json: %v", err)
	}
	if len(written.Items) != 2 {
		t.Fatalf("written confirmed = %#v", written)
	}
	bib := readFile(t, s.Path("references", "confirmed.bib"))
	if !strings.Contains(bib, "@article{smith2024RetrievalAugmentedGeneration") || !strings.Contains(bib, "doi = {10.1000/rag}") {
		t.Fatalf("bibtex = %s", bib)
	}
}

func TestReferenceKeyConflictSuffixIsStable(t *testing.T) {
	used := map[string]int{}
	candidate := contracts.ReferenceCandidate{
		Title:   "Retrieval Augmented Generation",
		Authors: []string{"Smith"},
		Year:    2024,
	}
	keys := []string{
		UniqueReferenceKey(candidate, used),
		UniqueReferenceKey(candidate, used),
		UniqueReferenceKey(candidate, used),
	}
	want := []string{"smith2024RetrievalAugmentedGeneration", "smith2024RetrievalAugmentedGenerationA", "smith2024RetrievalAugmentedGenerationB"}
	for i := range want {
		if keys[i] != want[i] {
			t.Fatalf("keys = %#v, want %#v", keys, want)
		}
	}
}

func TestConfirmCandidatesRejectsNoConfirmedReferences(t *testing.T) {
	_, err := ConfirmCandidates(store.NewAt(filepath.Join(t.TempDir(), "store")), contracts.ReferenceCandidates{}, ConfirmationDecision{})
	if !IsNoneConfirmed(err) {
		t.Fatalf("error = %v", err)
	}
	var confirmErr ConfirmError
	if !errors.As(err, &confirmErr) || confirmErr.Code != CodeNoneConfirmed {
		t.Fatalf("typed error = %v", err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	return string(data)
}
