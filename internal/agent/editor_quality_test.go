package agent

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/artifacts"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/quality"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/store"
)

// setupEditorQualityStore extends the writer store with a drafted chapter and
// its extracted claim graph so reviews can reference claim ids.
func setupEditorQualityStore(t *testing.T) (store.Store, quality.ClaimGraph) {
	t.Helper()
	s := setupWriterQualityStore(t)
	if _, err := WriteGuardedDraftBundle(s, writerTestBundle([]string{"ev_001"}, "doe2024deep")); err != nil {
		t.Fatalf("write chapter bundle: %v", err)
	}
	graph, _, err := quality.ExtractChapterClaimGraph(s, "ch01", 1, false, time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("extract claim graph: %v", err)
	}
	return s, graph
}

func editorTestReview(version int, instructions []contracts.RewriteInstruction) contracts.Review {
	return contracts.Review{
		ChapterID:    "ch01",
		DraftVersion: version,
		Scores: contracts.ReviewScores{
			Overall: 70, CitationConsistency: 92, StructureLogic: 80, Coverage: 75, Readability: 85,
		},
		Passed:              false,
		UnsupportedClaims:   []string{},
		RequiredFixes:       []string{"ground the survey claim in evidence"},
		OptionalFixes:       []string{},
		RewriteInstructions: instructions,
	}
}

func requiredInstruction(claimID string) contracts.RewriteInstruction {
	return contracts.RewriteInstruction{
		ClaimID:              claimID,
		Location:             "ch01 paragraph 1",
		Problem:              "overstated",
		Instruction:          "Restate the claim within the scope of the cited survey and cite the abstract finding.",
		SuggestedEvidenceIDs: []string{"ev_001"},
		Severity:             RewriteSeverityRequired,
	}
}

func TestReviewRewriteInstructionsRoundTripAndLegacyCompat(t *testing.T) {
	s, graph := setupEditorQualityStore(t)
	review := editorTestReview(1, []contracts.RewriteInstruction{requiredInstruction(graph.Claims[0].ID)})
	if _, err := WriteGuardedReview(s, review, "Needs a grounded rewrite."); err != nil {
		t.Fatalf("WriteGuardedReview() error = %v", err)
	}
	var loaded contracts.Review
	if err := store.ReadJSON(s.Path(filepath.FromSlash("drafts/ch01/review-v1.json")), &loaded); err != nil {
		t.Fatalf("ReadJSON(review) error = %v", err)
	}
	ins := loaded.RewriteInstructions
	if len(ins) != 1 || ins[0].ClaimID != graph.Claims[0].ID || ins[0].Severity != RewriteSeverityRequired ||
		len(ins[0].SuggestedEvidenceIDs) != 1 || ins[0].SuggestedEvidenceIDs[0] != "ev_001" {
		t.Fatalf("round-tripped instructions = %#v", ins)
	}

	// Legacy review.json without the field still loads strictly and carries
	// no instructions.
	legacy := `{"chapter_id":"ch01","draft_version":9,"scores":{"overall":85,"citation_consistency":95,` +
		`"structure_logic":80,"coverage":80,"readability":85},"passed":true,"unsupported_claims":[],` +
		`"required_fixes":[],"optional_fixes":[]}`
	legacyPath := s.Path(filepath.FromSlash("drafts/ch01/review-legacy.json"))
	if err := os.WriteFile(legacyPath, []byte(legacy), 0o644); err != nil {
		t.Fatalf("write legacy review: %v", err)
	}
	var compat contracts.Review
	if err := store.ReadJSON(legacyPath, &compat); err != nil {
		t.Fatalf("legacy review must still load: %v", err)
	}
	if len(compat.RewriteInstructions) != 0 {
		t.Fatalf("legacy instructions = %#v", compat.RewriteInstructions)
	}
}

func TestGuardReviewInstructionsRejectsUnknownEvidence(t *testing.T) {
	s, graph := setupEditorQualityStore(t)
	bad := requiredInstruction(graph.Claims[0].ID)
	bad.SuggestedEvidenceIDs = []string{"ev_999"}
	_, err := WriteGuardedReview(s, editorTestReview(1, []contracts.RewriteInstruction{bad}), "")
	agentErr, ok := AsError(err)
	if !ok || agentErr.Code != CodeInstructionUnknownEvidence {
		t.Fatalf("error = %v", err)
	}
	if _, statErr := os.Stat(s.Path(filepath.FromSlash("drafts/ch01/review-v1.json"))); !os.IsNotExist(statErr) {
		t.Fatalf("review unexpectedly written (stat err = %v)", statErr)
	}
}

func TestGuardReviewInstructionsRejectsUnknownClaimAndBadSeverity(t *testing.T) {
	s, graph := setupEditorQualityStore(t)
	unknownClaim := requiredInstruction("claim_999")
	_, err := WriteGuardedReview(s, editorTestReview(1, []contracts.RewriteInstruction{unknownClaim}), "")
	if agentErr, ok := AsError(err); !ok || agentErr.Code != CodeInstructionUnknownClaim {
		t.Fatalf("unknown claim error = %v", err)
	}

	badSeverity := requiredInstruction(graph.Claims[0].ID)
	badSeverity.Severity = "blocking"
	_, err = WriteGuardedReview(s, editorTestReview(1, []contracts.RewriteInstruction{badSeverity}), "")
	if agentErr, ok := AsError(err); !ok || agentErr.Code != CodeInstructionInvalid {
		t.Fatalf("bad severity error = %v", err)
	}

	empty := requiredInstruction(graph.Claims[0].ID)
	empty.Instruction = ""
	_, err = WriteGuardedReview(s, editorTestReview(1, []contracts.RewriteInstruction{empty}), "")
	if agentErr, ok := AsError(err); !ok || agentErr.Code != CodeInstructionInvalid {
		t.Fatalf("empty instruction error = %v", err)
	}
}

func TestGuardReviewInstructionsRequiresCoverageForNeedsRewrite(t *testing.T) {
	s, graph := setupEditorQualityStore(t)
	graph.Claims[0].NeedsRewrite = true
	if _, err := quality.SaveClaimGraph(s, graph); err != nil {
		t.Fatalf("save graph: %v", err)
	}
	_, err := WriteGuardedReview(s, editorTestReview(1, nil), "")
	agentErr, ok := AsError(err)
	if !ok || agentErr.Code != CodeInstructionMissingForRewrite {
		t.Fatalf("error = %v", err)
	}
	if agentErr.Details["claim_id"] != graph.Claims[0].ID {
		t.Fatalf("details = %#v", agentErr.Details)
	}
	// Covering the claim makes the same review pass the guard.
	covered := editorTestReview(1, []contracts.RewriteInstruction{requiredInstruction(graph.Claims[0].ID)})
	if _, err := WriteGuardedReview(s, covered, ""); err != nil {
		t.Fatalf("covered review error = %v", err)
	}
}

func TestGuardReviewInstructionsRequiresCoverageForVerifiedRewriteClaims(t *testing.T) {
	s, graph := setupEditorQualityStore(t)
	graph.Claims[0].Support = quality.SupportOverstated
	if _, err := quality.SaveClaimGraph(s, graph); err != nil {
		t.Fatalf("save graph: %v", err)
	}
	_, err := WriteGuardedReview(s, editorTestReview(1, nil), "")
	if agentErr, ok := AsError(err); !ok || agentErr.Code != CodeInstructionMissingForRewrite {
		t.Fatalf("overstated claim without instruction error = %v", err)
	}

	covered := editorTestReview(1, []contracts.RewriteInstruction{requiredInstruction(graph.Claims[0].ID)})
	if _, err := WriteGuardedReview(s, covered, ""); err != nil {
		t.Fatalf("covered overstated review error = %v", err)
	}
}

func TestReviewGateWithInstructionsBlocksDirectPass(t *testing.T) {
	passing := contracts.Review{
		ChapterID: "ch01", DraftVersion: 1, Passed: true,
		Scores: contracts.ReviewScores{Overall: 90, CitationConsistency: 95},
	}
	// No instructions: the existing gate result stands.
	if gate := ReviewGateWithInstructions(passing, 0); !gate.Passed {
		t.Fatalf("clean gate = %#v", gate)
	}
	// Optional instructions never block.
	passing.RewriteInstructions = []contracts.RewriteInstruction{{
		Location: "p1", Problem: "weak evidence", Instruction: "tighten wording",
		Severity: RewriteSeverityOptional,
	}}
	if gate := ReviewGateWithInstructions(passing, 0); !gate.Passed {
		t.Fatalf("optional gate = %#v", gate)
	}
	// A required instruction forces another rewrite round even if scores pass.
	passing.RewriteInstructions[0].Severity = RewriteSeverityRequired
	gate := ReviewGateWithInstructions(passing, 0)
	if gate.Passed || gate.Status != artifacts.StatusRevisionRequired {
		t.Fatalf("required gate = %#v", gate)
	}
	// Past the existing two-round limit it escalates to needs_human_review.
	gate = ReviewGateWithInstructions(passing, artifacts.MaxRevisionRounds)
	if gate.Passed || gate.Status != artifacts.StatusNeedsHumanReview {
		t.Fatalf("exceeded gate = %#v", gate)
	}
}

// TestMockRewriteLoopCarriesInstructionsAndConverges drives the full module 28
// loop: the Editor writes a guarded review with a required instruction, the
// rewrite Writer receives that instruction through writer_run, rewrites using
// the suggested evidence, and the re-verified second round passes.
func TestMockRewriteLoopCarriesInstructionsAndConverges(t *testing.T) {
	s, graph := setupEditorQualityStore(t)
	claimID := graph.Claims[0].ID

	// Round 1: verifier marks the claim overstated, Editor reviews with one
	// required rewrite instruction.
	if _, _, _, err := quality.SaveVerificationResult(s, []quality.ClaimVerdict{{
		ClaimID: claimID, Support: quality.SupportOverstated, RiskLevel: quality.RiskHigh,
	}}, time.Date(2026, 6, 10, 12, 1, 0, 0, time.UTC)); err != nil {
		t.Fatalf("save verification: %v", err)
	}
	review := editorTestReview(1, []contracts.RewriteInstruction{requiredInstruction(claimID)})
	if _, err := WriteGuardedReview(s, review, "Overstated claim must be scoped to the survey."); err != nil {
		t.Fatalf("WriteGuardedReview(v1) error = %v", err)
	}
	gate := ReviewGateWithInstructions(review, 0)
	if gate.Passed || gate.Status != artifacts.StatusRevisionRequired {
		t.Fatalf("round 1 gate = %#v", gate)
	}

	// Round 2: writer_run injects the previous instructions; the runner
	// rewrites with the suggested evidence and persists v2 through the guard.
	var got WriterChapterInput
	runner := WriterRunnerFunc(func(_ context.Context, args json.RawMessage) (any, error) {
		if err := json.Unmarshal(args, &got); err != nil {
			return nil, err
		}
		bundle := writerTestBundle(got.RewriteInstructions[0].SuggestedEvidenceIDs, "doe2024deep")
		bundle.Version = 2
		bundle.Claims.DraftVersion = 2
		bundle.Claims.Claims[0].Text = "The cited survey reports improved coverage for deep models."
		bundle.CitationMap.DraftVersion = 2
		result, err := WriteGuardedDraftBundle(s, bundle)
		if err != nil {
			return nil, err
		}
		return map[string]any{"paths": result.Paths}, nil
	})
	coordinator := Coordinator{
		Decisions: scriptedDecisions{
			[]byte(`{"action":"call_tool","tool":"writer_run","facts":{"chapter_id":"ch01"}}`),
		},
		Tools: NewAgentCoreToolRunner(DefaultTools(s, runner)),
	}
	step, err := coordinator.Step(context.Background())
	if err != nil || step.Response == nil || !step.Response.OK {
		t.Fatalf("rewrite step = %#v err = %v", step.Response, err)
	}
	if len(got.RewriteInstructions) != 1 || got.RewriteInstructions[0].ClaimID != claimID ||
		got.RewriteInstructions[0].Severity != RewriteSeverityRequired {
		t.Fatalf("writer input instructions = %#v", got.RewriteInstructions)
	}

	// Re-extract and re-verify: round 2 supports the claim, the Editor has no
	// remaining required instructions, and the gate converges to accepted.
	graph2, _, err := quality.ExtractChapterClaimGraph(s, "ch01", 2, false, time.Date(2026, 6, 10, 12, 2, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("re-extract: %v", err)
	}
	if _, _, _, err := quality.SaveVerificationResult(s, []quality.ClaimVerdict{{
		ClaimID: graph2.Claims[0].ID, Support: quality.SupportSupported,
	}}, time.Date(2026, 6, 10, 12, 3, 0, 0, time.UTC)); err != nil {
		t.Fatalf("save verification v2: %v", err)
	}
	passReview := contracts.Review{
		ChapterID: "ch01", DraftVersion: 2, Passed: true,
		Scores:            contracts.ReviewScores{Overall: 88, CitationConsistency: 95, StructureLogic: 85, Coverage: 85, Readability: 88},
		UnsupportedClaims: []string{}, RequiredFixes: []string{}, OptionalFixes: []string{},
	}
	if _, err := WriteGuardedReview(s, passReview, "Accepted after grounded rewrite."); err != nil {
		t.Fatalf("WriteGuardedReview(v2) error = %v", err)
	}
	finalGate := ReviewGateWithInstructions(passReview, 1)
	if !finalGate.Passed || finalGate.Status != artifacts.StatusAccepted {
		t.Fatalf("round 2 gate = %#v", finalGate)
	}
}

// TestMockRewriteLoopExceedsRoundsNeedsHumanReview verifies the unchanged
// two-round ceiling: a review still carrying required instructions after the
// limit escalates to needs_human_review instead of looping.
func TestMockRewriteLoopExceedsRoundsNeedsHumanReview(t *testing.T) {
	s, graph := setupEditorQualityStore(t)
	review := editorTestReview(1, []contracts.RewriteInstruction{requiredInstruction(graph.Claims[0].ID)})
	if _, err := WriteGuardedReview(s, review, ""); err != nil {
		t.Fatalf("WriteGuardedReview() error = %v", err)
	}
	gate := ReviewGateWithInstructions(review, artifacts.MaxRevisionRounds)
	if gate.Passed || gate.Status != artifacts.StatusNeedsHumanReview {
		t.Fatalf("exceeded gate = %#v", gate)
	}
}

func TestBuildWriterChapterInputWithoutReviewHistoryCarriesNoInstructions(t *testing.T) {
	s := setupWriterQualityStore(t)
	input, err := BuildWriterChapterInput(s, json.RawMessage(`{"chapter_id":"ch01"}`))
	if err != nil {
		t.Fatalf("BuildWriterChapterInput() error = %v", err)
	}
	if len(input.RewriteInstructions) != 0 {
		t.Fatalf("instructions = %#v", input.RewriteInstructions)
	}
}

func TestCoordinatorSystemPromptMentionsRewriteInstructions(t *testing.T) {
	prompt := CoordinatorSystemPrompt(PromptOptions{})
	for _, want := range []string{
		"rewrite_instructions",
		"suggested_evidence_ids",
		"needs_rewrite must receive at least one instruction",
		"cannot pass",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q", want)
		}
	}
}
