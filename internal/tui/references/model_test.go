package referencestui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/i18n"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/references"
)

func TestModelSelectRejectAndConfirmDecision(t *testing.T) {
	model := NewModel(sampleCandidates(), Options{MinConfirmed: 1})
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
	model := NewModel(sampleCandidates(), Options{MinConfirmed: 1})
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
	if view := model.View(); !strings.Contains(view, "搜索: human") || !strings.Contains(view, "Enter/Esc：结束搜索") {
		t.Fatalf("search view missing search prompt:\n%s", view)
	}
	model = model.UpdateKey("enter")
	if model.Searching() {
		t.Fatalf("search mode did not exit")
	}

	model = NewModel(sampleCandidates(), Options{MinConfirmed: 1})
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
	model := NewModel(sampleCandidates(), Options{MinConfirmed: 1})
	model = model.UpdateKey("a")
	if got := model.SelectedIDs(); len(got) != 2 || got[0] != "cand_001" || got[1] != "cand_002" {
		t.Fatalf("selected = %#v", got)
	}
	if view := model.View(); !strings.Contains(view, "[x]") {
		t.Fatalf("selected state missing from view:\n%s", view)
	}

	empty := NewModel(sampleCandidates(), Options{MinConfirmed: 1})
	empty = empty.UpdateKey("enter")
	if !references.IsNoneConfirmed(empty.Err()) || empty.Done() {
		t.Fatalf("empty err=%v done=%v", empty.Err(), empty.Done())
	}
	if view := empty.View(); !strings.Contains(view, "错误") || !strings.Contains(view, "至少需要确认") {
		t.Fatalf("error state missing from view:\n%s", view)
	}
}

func TestModelViewShowsCandidateDetails(t *testing.T) {
	view := NewModel(sampleCandidates(), Options{MinConfirmed: 1}).View()
	for _, want := range []string{"文献候选", "排序: original", "▶", "[ ]", "RAG for Review Writing", "Semantic Scholar（学术搜索源）", "可获取性：开放获取", "可靠性：官方公开接口", "DOI: 10.1000/rag", "概要: Retrieval augmented generation for review writing.", "相关度: 0.90", "direct match", "Space：选择"} {
		if !strings.Contains(view, want) {
			t.Fatalf("view missing %q:\n%s", want, view)
		}
	}
	doiIndex := strings.Index(view, "DOI: 10.1000/rag")
	summaryIndex := strings.Index(view, "概要: Retrieval augmented generation for review writing.")
	relevanceIndex := strings.Index(view, "相关度: 0.90")
	if doiIndex == -1 || summaryIndex == -1 || relevanceIndex == -1 || !(doiIndex < summaryIndex && summaryIndex < relevanceIndex) {
		t.Fatalf("summary should render after DOI and before relevance:\n%s", view)
	}
}

func TestModelViewOmitsBlankSummary(t *testing.T) {
	view := NewModel(contracts.ReferenceCandidates{Items: []contracts.ReferenceCandidate{{
		ID:       "cand_blank",
		Title:    "Blank Abstract",
		Source:   "test",
		Abstract: " \n\t ",
		Status:   "pending",
	}}}, Options{MinConfirmed: 1}).View()

	if strings.Contains(view, "概要:") {
		t.Fatalf("view should omit blank summary:\n%s", view)
	}
}

func TestModelViewUsesEnglishSummaryLabel(t *testing.T) {
	view := NewModel(sampleCandidates(), Options{I18N: i18n.New("en"), MinConfirmed: 1}).View()
	for _, want := range []string{"Summary: Retrieval augmented generation for review writing.", "Sort: original", "Space: select"} {
		if !strings.Contains(view, want) {
			t.Fatalf("English view missing %q:\n%s", want, view)
		}
	}
	for _, unwanted := range []string{"排序", "Space：选择"} {
		if strings.Contains(view, unwanted) {
			t.Fatalf("English view should not contain %q:\n%s", unwanted, view)
		}
	}
}

func TestModelRequiresAtLeastFiveReferencesByDefault(t *testing.T) {
	model := NewModel(sampleManyCandidates(5))
	for i := 0; i < 4; i++ {
		model = model.UpdateKey("space")
		model = model.UpdateKey("down")
	}
	model = model.UpdateKey("enter")
	if !references.IsNoneConfirmed(model.Err()) || model.Done() {
		t.Fatalf("model should block fewer than five confirmations, err=%v done=%v", model.Err(), model.Done())
	}

	model = model.UpdateKey("space")
	model = model.UpdateKey("enter")
	if !model.Done() || model.Err() != nil {
		t.Fatalf("model should allow five confirmations, err=%v done=%v", model.Err(), model.Done())
	}
}

func sampleManyCandidates(count int) contracts.ReferenceCandidates {
	items := make([]contracts.ReferenceCandidate, 0, count)
	for i := 1; i <= count; i++ {
		items = append(items, contracts.ReferenceCandidate{
			ID:             fmt.Sprintf("cand_%03d", i),
			Title:          fmt.Sprintf("Paper %d", i),
			Authors:        []string{"Author"},
			Year:           2020 + i,
			Source:         "semantic_scholar",
			RelevanceScore: 0.9,
			Status:         "pending",
		})
	}
	return contracts.ReferenceCandidates{Items: items}
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
			Abstract:        "  Retrieval\naugmented\tgeneration for review writing.  ",
			Reliability:     "official_api",
			Availability:    "open_access",
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
