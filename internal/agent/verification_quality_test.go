package agent

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/checkpoint"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/quality"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/store"
)

// TestClaimVerificationStepCheckpointAndResume drives a mock Coordinator
// through claim_extraction → claim_verification → quality_gate_check, records
// the verification step through the existing checkpoint mechanism, and resumes
// from the latest checkpoint.
func TestClaimVerificationStepCheckpointAndResume(t *testing.T) {
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
			[]byte(`{"action":"call_tool","tool":"save_verification_result","facts":{"verdicts":[{"claim_id":"claim_001","support":"supported","verifier_note":"abstract directly states the finding"}]}}`),
			[]byte(`{"action":"call_tool","tool":"quality_gate_check","facts":{"mode":"enhanced"}}`),
		},
		Tools: tools,
	}
	for i := 0; i < 3; i++ {
		result, err := coordinator.Step(context.Background())
		if err != nil {
			t.Fatalf("step %d error = %v", i, err)
		}
		if result.Response == nil || !result.Response.OK {
			t.Fatalf("step %d response = %#v", i, result.Response)
		}
	}
	gate := coordinator.Observations[2].Response
	data, ok := gate.Data.(map[string]any)
	if !ok || data["conclusion"] != quality.GatePass {
		t.Fatalf("gate observation = %#v", gate.Data)
	}
	recordQualityStep(t, s, 1, StepClaimVerification, "review_chapter",
		[]string{quality.VerificationResultJSONRel, quality.ClaimGraphJSONRel, quality.ClaimGraphMarkdownRel})

	// Interruption: a fresh process validates the latest checkpoint, learns
	// claim_verification is done, and both artifacts are readable.
	validation := checkpoint.ValidateLatest(s)
	if !validation.OK || validation.Phase != StepClaimVerification {
		t.Fatalf("resume validation = %#v", validation)
	}
	result, err := quality.LoadVerificationResult(s)
	if err != nil || len(result.Verdicts) != 1 || result.Verdicts[0].Support != quality.SupportSupported {
		t.Fatalf("recovered verification result = %#v err %v", result, err)
	}
	graph, err := quality.LoadClaimGraph(s)
	if err != nil || graph.Claims[0].Support != quality.SupportSupported {
		t.Fatalf("recovered graph = %#v err %v", graph, err)
	}
}

func TestDefaultToolsIncludeVerificationTools(t *testing.T) {
	s := store.NewAt(t.TempDir())
	names := map[string]bool{}
	for _, tool := range DefaultTools(s, nil) {
		names[tool.Name()] = true
	}
	for _, want := range []string{"save_verification_result", "quality_gate_check"} {
		if !names[want] {
			t.Fatalf("tool %s missing, names = %#v", want, names)
		}
	}
}

func TestCoordinatorSystemPromptMentionsClaimVerification(t *testing.T) {
	prompt := CoordinatorSystemPrompt(PromptOptions{})
	for _, want := range []string{
		StepClaimVerification, "save_verification_result", "quality_gate_check",
		"uncertainty never passes",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q", want)
		}
	}
}
