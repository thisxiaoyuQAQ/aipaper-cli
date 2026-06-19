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

// setupWriterQualityStore extends setupQualityStore with the quality artifacts
// the writer protocol consumes: an evidence table (ev_001 -> doe2024deep) and
// a section quality plan binding chapter ch01 to ev_001.
func setupWriterQualityStore(t *testing.T) store.Store {
	t.Helper()
	s := setupQualityStore(t)
	table := quality.EvidenceTable{
		GeneratedAt: time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC),
		Items: []quality.Evidence{{
			ID:           "ev_001",
			ReferenceKey: "doe2024deep",
			Depth:        quality.DepthAbstract,
			Topics:       []string{"survey"},
			KeyFindings:  []string{"deep models survey finding"},
			Confidence:   quality.ConfidenceHigh,
		}},
	}
	if _, err := quality.SaveEvidenceTable(s, table); err != nil {
		t.Fatalf("SaveEvidenceTable() error = %v", err)
	}
	plan := quality.SectionQualityPlan{
		GeneratedAt: time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC),
		Sections: []quality.SectionPlan{{
			ChapterID:                "ch01",
			Questions:                []string{"What does the survey cover?"},
			RequiredEvidenceIDs:      []string{"ev_001"},
			ForbiddenGeneralizations: []string{"all models"},
		}},
	}
	if _, err := quality.SaveSectionQualityPlan(s, plan); err != nil {
		t.Fatalf("SaveSectionQualityPlan() error = %v", err)
	}
	return s
}

func writerTestBundle(evidenceIDs []string, referenceKey string) artifacts.DraftBundle {
	return artifacts.DraftBundle{
		ChapterID:     "ch01",
		Version:       1,
		DraftMarkdown: "# Chapter 1\nBody grounded in evidence.",
		Claims: contracts.ClaimsFile{
			ChapterID:    "ch01",
			DraftVersion: 1,
			Claims: []contracts.Claim{{
				ID:            "ch01_claim_001",
				Text:          "Survey evidence supports this claim.",
				Importance:    "high",
				ReferenceKeys: []string{referenceKey},
				EvidenceIDs:   evidenceIDs,
			}},
		},
		CitationMap: contracts.CitationMap{
			ChapterID:    "ch01",
			DraftVersion: 1,
			Mappings: []contracts.CitationMapping{{
				ParagraphID:   "ch01_p001",
				ClaimIDs:      []string{"ch01_claim_001"},
				ReferenceKeys: []string{referenceKey},
			}},
		},
		WriterNotes: "Gap: only abstract-level evidence available for the survey claim.",
	}
}

func TestBuildWriterChapterInputInjectsPlanAndEvidence(t *testing.T) {
	s := setupWriterQualityStore(t)
	input, err := BuildWriterChapterInput(s, json.RawMessage(`{"chapter_id":"ch01"}`))
	if err != nil {
		t.Fatalf("BuildWriterChapterInput() error = %v", err)
	}
	if input.ChapterID != "ch01" || input.QualityPlan == nil || input.QualityPlan.ChapterID != "ch01" {
		t.Fatalf("input = %#v", input)
	}
	if len(input.Evidence) != 1 || input.Evidence[0].ID != "ev_001" || input.Evidence[0].ReferenceKey != "doe2024deep" {
		t.Fatalf("evidence = %#v", input.Evidence)
	}
	if len(input.Warnings) != 0 {
		t.Fatalf("warnings = %#v", input.Warnings)
	}
	guidance := strings.Join(input.TemplateGuidance, "\n")
	if !strings.Contains(guidance, "Paper quality writing policy") || !strings.Contains(guidance, "writer_notes") {
		t.Fatalf("template guidance missing paper quality policy: %s", guidance)
	}
	forbidden := strings.Join(input.ForbiddenDraftPatterns, "\n")
	if !strings.Contains(forbidden, "证据不足") || !strings.Contains(forbidden, "metadata_only") {
		t.Fatalf("forbidden patterns missing paper quality policy: %s", forbidden)
	}
}

func TestBuildWriterChapterInputCompatWithoutQualityArtifacts(t *testing.T) {
	s := setupQualityStore(t) // confirmed refs + outline only, no quality artifacts
	input, err := BuildWriterChapterInput(s, json.RawMessage(`{"chapter_id":"ch01"}`))
	if err != nil {
		t.Fatalf("BuildWriterChapterInput() error = %v", err)
	}
	if input.QualityPlan != nil || len(input.Evidence) != 0 {
		t.Fatalf("compat input = %#v", input)
	}
	if len(input.Warnings) != 1 || !strings.Contains(input.Warnings[0], "compatibility mode") {
		t.Fatalf("warnings = %#v", input.Warnings)
	}
}

func TestWriterToolInjectsQualityPlanIntoRunnerArgs(t *testing.T) {
	s := setupWriterQualityStore(t)
	var got WriterChapterInput
	tool := NewWriterTool(s, WriterRunnerFunc(func(_ context.Context, args json.RawMessage) (any, error) {
		if err := json.Unmarshal(args, &got); err != nil {
			t.Fatalf("Unmarshal(writer args) error = %v", err)
		}
		return map[string]any{"ok": true}, nil
	}))
	response := executeTool(t, tool, `{"chapter_id":"ch01"}`)
	if !response.OK {
		t.Fatalf("writer tool failed: %#v", response)
	}
	if got.ChapterID != "ch01" || got.QualityPlan == nil || len(got.Evidence) != 1 {
		t.Fatalf("runner input = %#v", got)
	}
	if got.Evidence[0].ID != "ev_001" {
		t.Fatalf("runner evidence = %#v", got.Evidence)
	}
}

func TestWriteGuardedDraftBundleBlocksClaimWithoutEvidence(t *testing.T) {
	s := setupWriterQualityStore(t)
	_, err := WriteGuardedDraftBundle(s, writerTestBundle(nil, "doe2024deep"))
	agentErr, ok := AsError(err)
	if !ok || agentErr.Code != CodeClaimMissingEvidence {
		t.Fatalf("error = %v", err)
	}
	assertNoChapterArtifacts(t, s)
}

func TestWriteGuardedDraftBundleBlocksUnknownEvidence(t *testing.T) {
	s := setupWriterQualityStore(t)
	_, err := WriteGuardedDraftBundle(s, writerTestBundle([]string{"ev_999"}, "doe2024deep"))
	agentErr, ok := AsError(err)
	if !ok || agentErr.Code != CodeClaimUnknownEvidence {
		t.Fatalf("error = %v", err)
	}
	if agentErr.Details["claim_id"] != "ch01_claim_001" {
		t.Fatalf("details = %#v", agentErr.Details)
	}
	assertNoChapterArtifacts(t, s)
}

func TestWriteGuardedDraftBundleBlocksForgedReferenceKey(t *testing.T) {
	s := setupWriterQualityStore(t)
	_, err := WriteGuardedDraftBundle(s, writerTestBundle([]string{"ev_001"}, "forged2024key"))
	agentErr, ok := AsError(err)
	if !ok || agentErr.Code != CodeClaimUnconfirmedReference {
		t.Fatalf("error = %v", err)
	}
	assertNoChapterArtifacts(t, s)
}

func TestWriteGuardedDraftBundleWritesValidBundle(t *testing.T) {
	s := setupWriterQualityStore(t)
	result, err := WriteGuardedDraftBundle(s, writerTestBundle([]string{"ev_001"}, "doe2024deep"))
	if err != nil {
		t.Fatalf("WriteGuardedDraftBundle() error = %v", err)
	}
	if len(result.Outputs) != 4 {
		t.Fatalf("outputs = %#v", result.Outputs)
	}
	var claims contracts.ClaimsFile
	if err := store.ReadJSON(s.Path(filepath.FromSlash("drafts/ch01/claims-v1.json")), &claims); err != nil {
		t.Fatalf("ReadJSON(claims) error = %v", err)
	}
	if len(claims.Claims) != 1 || len(claims.Claims[0].EvidenceIDs) != 1 || claims.Claims[0].EvidenceIDs[0] != "ev_001" {
		t.Fatalf("persisted claims = %#v", claims.Claims)
	}
}

// TestMockWriterFlowInjectsPlanAndPersistsClaims drives a mock Coordinator
// through writer_run: the Host injects the chapter quality plan and evidence
// into the runner input, and the runner persists claims through the guarded
// write so every claim is bound to valid evidence.
func TestMockWriterFlowInjectsPlanAndPersistsClaims(t *testing.T) {
	s := setupWriterQualityStore(t)
	runner := WriterRunnerFunc(func(_ context.Context, args json.RawMessage) (any, error) {
		var input WriterChapterInput
		if err := json.Unmarshal(args, &input); err != nil {
			return nil, err
		}
		if input.QualityPlan == nil || len(input.Evidence) == 0 {
			t.Fatalf("writer runner input missing quality context: %#v", input)
		}
		bundle := writerTestBundle(input.QualityPlan.RequiredEvidenceIDs, input.Evidence[0].ReferenceKey)
		result, err := WriteGuardedDraftBundle(s, bundle)
		if err != nil {
			return nil, err
		}
		return map[string]any{"chapter_id": input.ChapterID, "paths": result.Paths}, nil
	})
	coordinator := Coordinator{
		Decisions: scriptedDecisions{
			[]byte(`{"action":"call_tool","tool":"writer_run","facts":{"chapter_id":"ch01"}}`),
			[]byte(`{"action":"finish","reason":"chapter drafted"}`),
		},
		Tools: NewAgentCoreToolRunner(DefaultTools(s, runner)),
	}
	result, err := coordinator.Step(context.Background())
	if err != nil {
		t.Fatalf("writer step error = %v", err)
	}
	if result.Response == nil || !result.Response.OK {
		t.Fatalf("writer step response = %#v", result.Response)
	}

	var claims contracts.ClaimsFile
	if err := store.ReadJSON(s.Path(filepath.FromSlash("drafts/ch01/claims-v1.json")), &claims); err != nil {
		t.Fatalf("ReadJSON(claims) error = %v", err)
	}
	for _, claim := range claims.Claims {
		if len(claim.EvidenceIDs) == 0 {
			t.Fatalf("claim %q persisted without evidence bindings", claim.ID)
		}
	}

	final, err := coordinator.Step(context.Background())
	if err != nil || !final.Done {
		t.Fatalf("finish step = %#v, err = %v", final, err)
	}
}

func TestCoordinatorSystemPromptMentionsWriterEvidenceProtocol(t *testing.T) {
	prompt := CoordinatorSystemPrompt(PromptOptions{})
	for _, want := range []string{
		"evidence_ids",
		"never fabricate",
		"block the chapter output",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q", want)
		}
	}
}

func assertNoChapterArtifacts(t *testing.T, s store.Store) {
	t.Helper()
	for _, rel := range []string{
		"drafts/ch01/draft-v1.md",
		"drafts/ch01/claims-v1.json",
		"drafts/ch01/citation-map-v1.json",
	} {
		if _, err := os.Stat(s.Path(filepath.FromSlash(rel))); !os.IsNotExist(err) {
			t.Fatalf("artifact %s unexpectedly exists (err = %v)", rel, err)
		}
	}
}
