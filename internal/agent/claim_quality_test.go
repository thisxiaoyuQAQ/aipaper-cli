package agent

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/artifacts"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/checkpoint"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/quality"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/store"
)

func writeClaimChapter(t *testing.T, s store.Store, chapterID string) {
	t.Helper()
	bundle := artifacts.DraftBundle{
		ChapterID:     chapterID,
		Version:       1,
		DraftMarkdown: "# " + chapterID + "\n\nBody.",
		Claims: contracts.ClaimsFile{
			ChapterID:    chapterID,
			DraftVersion: 1,
			Claims: []contracts.Claim{{
				ID:            "c1",
				Text:          "Deep models improve survey coverage.",
				Importance:    "high",
				ReferenceKeys: []string{"doe2024deep"},
				EvidenceIDs:   []string{"ev_001"},
			}},
		},
		CitationMap: contracts.CitationMap{
			ChapterID:    chapterID,
			DraftVersion: 1,
			Mappings: []contracts.CitationMapping{{
				ParagraphID:   "p1",
				ClaimIDs:      []string{"c1"},
				ReferenceKeys: []string{"doe2024deep"},
			}},
		},
	}
	if _, err := WriteGuardedDraftBundle(s, bundle); err != nil {
		t.Fatalf("write chapter bundle: %v", err)
	}
}

// TestClaimExtractionStepCheckpointAndResume drives a mock Coordinator through
// claim_extraction after a chapter writer completes, records it through the
// existing checkpoint mechanism, and resumes from the latest checkpoint.
func TestClaimExtractionStepCheckpointAndResume(t *testing.T) {
	s := setupQualityStore(t)
	table := quality.EvidenceTable{
		GeneratedAt: time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC),
		Items: []quality.Evidence{{
			ID: "ev_001", ReferenceKey: "doe2024deep", Depth: quality.DepthAbstract,
			Topics: []string{"t"}, KeyFindings: []string{"f"}, Confidence: quality.ConfidenceHigh,
		}},
	}
	if _, err := quality.SaveEvidenceTable(s, table); err != nil {
		t.Fatalf("save evidence table: %v", err)
	}
	writeClaimChapter(t, s, "ch01")

	tools := NewAgentCoreToolRunner(DefaultTools(s, nil))
	coordinator := Coordinator{
		Decisions: scriptedDecisions{
			[]byte(`{"action":"call_tool","tool":"extract_chapter_claims","facts":{"chapter_id":"ch01","draft_version":1}}`),
		},
		Tools: tools,
	}
	result, err := coordinator.Step(context.Background())
	if err != nil {
		t.Fatalf("claim extraction step error = %v", err)
	}
	if result.Response == nil || !result.Response.OK {
		t.Fatalf("claim extraction response = %#v", result.Response)
	}
	recordQualityStep(t, s, 1, StepClaimExtraction, "claim_verification",
		[]string{quality.ClaimGraphJSONRel, quality.ClaimGraphMarkdownRel})

	// Interruption: a fresh process validates the latest checkpoint, learns
	// claim_extraction is done for ch01, and the claim graph is readable.
	validation := checkpoint.ValidateLatest(s)
	if !validation.OK || validation.Phase != StepClaimExtraction {
		t.Fatalf("resume validation = %#v", validation)
	}
	graph, err := quality.LoadClaimGraph(s)
	if err != nil || len(graph.Claims) != 1 || graph.Claims[0].ChapterID != "ch01" {
		t.Fatalf("recovered graph = %#v err %v", graph, err)
	}
}

func TestDefaultToolsIncludeClaimGraphTools(t *testing.T) {
	s := store.NewAt(t.TempDir())
	names := map[string]bool{}
	for _, tool := range DefaultTools(s, nil) {
		names[tool.Name()] = true
	}
	for _, want := range []string{"save_claim_graph", "load_claim_graph", "extract_chapter_claims"} {
		if !names[want] {
			t.Fatalf("tool %s missing, names = %#v", want, names)
		}
	}
}

func TestCoordinatorSystemPromptMentionsClaimExtraction(t *testing.T) {
	prompt := CoordinatorSystemPrompt(PromptOptions{})
	for _, want := range []string{StepClaimExtraction, "extract_chapter_claims"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q", want)
		}
	}
}
