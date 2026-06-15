package referencestui

import (
	"strings"
	"testing"
	"time"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/references"
)

func TestModelSelectRejectAndConfirmDecision(t *testing.T) {
	model := NewModel(sampleCandidates())
	model = model.UpdateKey("space")
	model = model.UpdateKey("down")
	model = model.UpdateKey("r")
	model = model.UpdateKey("enter")

	if !model.Done() || model.Err() != nil {
		t.Fatalf("model done=%v err=%v", model.Done(), model.Err())
	}
	if got := model.SelectedIDs(); len(got) != 1 || got[0] != "cand_001" {
		t.Fatalf("selected = %#v", got)
	}
	if got := model.RejectedIDs(); len(got) != 1 || got[0] != "cand_002" {
		t.Fatalf("rejected = %#v", got)
	}
	now := time.Date(2026, 6, 7, 0, 0, 0, 0, time.UTC)
	decision := model.Decision(now)
	if decision.ConfirmedAt != now || decision.ConfirmedIDs[0] != "cand_001" || decision.RejectedIDs[0] != "cand_002" {
		t.Fatalf("decision = %#v", decision)
	}
}

func TestModelSearchAndSort(t *testing.T) {
	model := NewModel(sampleCandidates())
	model = model.UpdateKey("/")
	for _, key := range []string{"h", "u", "m", "a", "n"} {
		model = model.UpdateKey(key)
	}
	if !model.Searching() || model.Filter() != "human" {
		t.Fatalf("searching=%v filter=%q", model.Searching(), model.Filter())
	}
	if visible := model.VisibleCandidates(); len(visible) != 1 || visible[0].ID != "cand_002" {
		t.Fatalf("visible = %#v", visible)
	}
	model = model.UpdateKey("enter")
	if model.Searching() {
		t.Fatalf("search mode did not exit")
	}

	model = NewModel(sampleCandidates())
	model = model.UpdateKey("s")
	if model.SortMode() != SortRelevance || model.CursorID() != "cand_002" {
		t.Fatalf("sort mode=%v cursor=%s", model.SortMode(), model.CursorID())
	}
	model = model.UpdateKey("s")
	if model.SortMode() != SortYear || model.CursorID() != "cand_003" {
		t.Fatalf("sort mode=%v cursor=%s", model.SortMode(), model.CursorID())
	}
}

func TestModelSelectHighRelevanceAndNoSelectionError(t *testing.T) {
	model := NewModel(sampleCandidates())
	model = model.UpdateKey("a")
	if got := model.SelectedIDs(); len(got) != 2 || got[0] != "cand_001" || got[1] != "cand_002" {
		t.Fatalf("selected = %#v", got)
	}

	empty := NewModel(sampleCandidates())
	empty = empty.UpdateKey("enter")
	if !references.IsNoneConfirmed(empty.Err()) || empty.Done() {
		t.Fatalf("empty err=%v done=%v", empty.Err(), empty.Done())
	}
}

func TestModelViewShowsCandidateDetails(t *testing.T) {
	view := NewModel(sampleCandidates()).View()
	for _, want := range []string{"文献候选", "RAG for Review Writing", "DOI: 10.1000/rag", "相关度: 0.90", "direct match"} {
		if !strings.Contains(view, want) {
			t.Fatalf("view missing %q:\n%s", want, view)
		}
	}
}

func sampleCandidates() contracts.ReferenceCandidates {
	return contracts.ReferenceCandidates{Items: []contracts.ReferenceCandidate{
		{
			ID:              "cand_001",
			Title:           "RAG for Review Writing",
			Authors:         []string{"Smith"},
			Year:            2024,
			Source:          "semantic_scholar",
			DOI:             "10.1000/rag",
			RelevanceScore:  0.9,
			RelevanceReason: "direct match",
			Status:          "pending",
		},
		{
			ID:              "cand_002",
			Title:           "Human in the Loop Review",
			Authors:         []string{"Li"},
			Year:            2023,
			Source:          "crossref",
			URL:             "https://example.org/hitl",
			RelevanceScore:  0.95,
			RelevanceReason: "human confirmation",
			Status:          "pending",
		},
		{
			ID:             "cand_003",
			Title:          "Older Survey",
			Authors:        []string{"Ada"},
			Year:           2025,
			Source:         "arxiv",
			RelevanceScore: 0.7,
			Status:         "pending",
		},
	}}
}
