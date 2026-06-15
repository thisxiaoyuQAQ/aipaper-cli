package agent

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/checkpoint"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/quality"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/store"
)

func setupQualityStore(t *testing.T) store.Store {
	t.Helper()
	s := store.NewAt(t.TempDir())
	if _, err := store.WriteJSON(s.RequirementsPath(), contracts.Requirements{
		Topic:           "A Survey",
		Language:        "en",
		CitationStyle:   "gbt7714",
		ArticleTemplate: quality.ArticleTemplateZhCoursePaper,
		QualityMode:     quality.ModeEnhanced,
		TargetWords:     500,
		MaterialDir:     t.TempDir(),
	}, store.Overwrite); err != nil {
		t.Fatalf("write requirements.json: %v", err)
	}
	confirmed := contracts.ConfirmedReferences{Items: []contracts.ConfirmedReference{{
		Key:               "doe2024deep",
		Title:             "Deep Survey",
		Authors:           []string{"Doe, J."},
		Year:              2024,
		SourceMaterialIDs: []string{},
		ConfirmedAt:       time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC),
	}}}
	if _, err := store.WriteJSON(s.Path("references", "confirmed.json"), confirmed, store.Overwrite); err != nil {
		t.Fatalf("write confirmed.json: %v", err)
	}
	outline := map[string]any{
		"title_suggestion": "A Survey",
		"chapters": []map[string]any{
			{"chapter_id": "ch01", "title": "Intro"},
		},
	}
	if _, err := store.WriteJSON(s.Path("outline", "outline.json"), outline, store.Overwrite); err != nil {
		t.Fatalf("write outline.json: %v", err)
	}
	return s
}

func recordQualityStep(t *testing.T, s store.Store, step int, phase, next string, outputs []string) checkpoint.Checkpoint {
	t.Helper()
	artifacts := make([]checkpoint.OutputArtifact, 0, len(outputs))
	for _, rel := range outputs {
		sum, err := store.FileSHA256(s.Path(filepath.FromSlash(rel)))
		if err != nil {
			t.Fatalf("hash %s: %v", rel, err)
		}
		artifacts = append(artifacts, checkpoint.OutputArtifact{Kind: phase, Path: rel, SHA256: sum})
	}
	cp := checkpoint.Checkpoint{
		Step:         step,
		Phase:        phase,
		CreatedAt:    time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC).Add(time.Duration(step) * time.Minute),
		Outputs:      artifacts,
		NextExpected: next,
	}
	progress := contracts.Progress{
		Phase:             phase,
		CompletedChapters: []string{},
		PendingChapters:   []string{"ch01"},
		Status:            "running",
		UpdatedAt:         cp.CreatedAt,
	}
	if err := checkpoint.Record(s, cp, progress); err != nil {
		t.Fatalf("checkpoint.Record(step %d) error = %v", step, err)
	}
	return cp
}

const (
	evidenceTableArgs = `{"table":{"generated_at":"2026-06-10T12:00:00Z","items":[` +
		`{"id":"ev_001","reference_key":"doe2024deep","depth":"abstract","topics":["t"],"key_findings":["f"],"confidence":"high"}]}}`
	sectionPlanArgs = `{"plan":{"generated_at":"2026-06-10T12:00:00Z","sections":[` +
		`{"chapter_id":"ch01","questions":["q"],"required_evidence_ids":["ev_001"]}]}}`
)

// TestQualityStepsCheckpointAndResume drives a mock Coordinator through the two
// new quality steps, records both through the existing checkpoint mechanism,
// then simulates an interruption and resumes from the latest checkpoint.
func TestQualityStepsCheckpointAndResume(t *testing.T) {
	s := setupQualityStore(t)
	tools := NewAgentCoreToolRunner(DefaultTools(s, nil))

	// Phase 1: evidence_extraction succeeds and is checkpointed, then the run
	// is interrupted before section_quality_plan.
	first := Coordinator{
		Decisions: scriptedDecisions{
			[]byte(`{"action":"call_tool","tool":"save_evidence_table","facts":` + evidenceTableArgs + `}`),
		},
		Tools: tools,
	}
	result, err := first.Step(context.Background())
	if err != nil {
		t.Fatalf("evidence step error = %v", err)
	}
	if result.Response == nil || !result.Response.OK {
		t.Fatalf("evidence step response = %#v", result.Response)
	}
	recordQualityStep(t, s, 1, StepEvidenceExtraction, StepSectionQualityPlan,
		[]string{quality.EvidenceTableJSONRel, quality.EvidenceTableMarkdownRel})

	// Interruption: a fresh process validates the latest checkpoint and learns
	// it must resume at section_quality_plan without redoing evidence_extraction.
	validation := checkpoint.ValidateLatest(s)
	if !validation.OK {
		t.Fatalf("validation after interruption = %#v", validation)
	}
	if validation.Phase != StepEvidenceExtraction || validation.Next != StepSectionQualityPlan {
		t.Fatalf("resume point = %#v", validation)
	}

	// Phase 2: resumed run executes section_quality_plan and checkpoints it.
	resumed := Coordinator{
		Decisions: scriptedDecisions{
			[]byte(`{"action":"call_tool","tool":"save_section_quality_plan","facts":` + sectionPlanArgs + `}`),
			[]byte(`{"action":"finish","reason":"quality plan persisted"}`),
		},
		Tools: tools,
	}
	result, err = resumed.Step(context.Background())
	if err != nil {
		t.Fatalf("section plan step error = %v", err)
	}
	if result.Response == nil || !result.Response.OK {
		t.Fatalf("section plan step response = %#v", result.Response)
	}
	recordQualityStep(t, s, 2, StepSectionQualityPlan, "draft_chapter",
		[]string{quality.SectionPlanJSONRel, quality.SectionPlanMarkdownRel})

	final, err := resumed.Step(context.Background())
	if err != nil {
		t.Fatalf("finish step error = %v", err)
	}
	if !final.Done {
		t.Fatalf("finish result = %#v", final)
	}

	// Latest checkpoint covers section_quality_plan and all artifact hashes match.
	validation = checkpoint.ValidateLatest(s)
	if !validation.OK || validation.Phase != StepSectionQualityPlan || validation.Step != 2 {
		t.Fatalf("final validation = %#v", validation)
	}
	if len(validation.Checked) != 2 {
		t.Fatalf("checked artifacts = %#v", validation.Checked)
	}
}

func TestQualityStepCheckpointIdempotentReplay(t *testing.T) {
	s := setupQualityStore(t)
	tools := NewAgentCoreToolRunner(DefaultTools(s, nil))
	coordinator := Coordinator{
		Decisions: scriptedDecisions{
			[]byte(`{"action":"call_tool","tool":"save_evidence_table","facts":` + evidenceTableArgs + `}`),
		},
		Tools: tools,
	}
	if _, err := coordinator.Step(context.Background()); err != nil {
		t.Fatalf("evidence step error = %v", err)
	}
	cp := recordQualityStep(t, s, 1, StepEvidenceExtraction, StepSectionQualityPlan,
		[]string{quality.EvidenceTableJSONRel, quality.EvidenceTableMarkdownRel})

	// Replaying the same step with identical content is idempotent (CreateOnly).
	progress := contracts.Progress{
		Phase:             cp.Phase,
		CompletedChapters: []string{},
		PendingChapters:   []string{"ch01"},
		Status:            "running",
		UpdatedAt:         cp.CreatedAt,
	}
	if err := checkpoint.Record(s, cp, progress); err != nil {
		t.Fatalf("idempotent replay error = %v", err)
	}

	// Same step number with different content must conflict.
	conflicting := cp
	conflicting.NextExpected = "draft_chapter"
	err := checkpoint.Record(s, conflicting, progress)
	if err == nil || !strings.Contains(err.Error(), "STORE_CHECKPOINT_CONFLICT") {
		t.Fatalf("conflicting replay error = %v", err)
	}
}

func TestDefaultToolsIncludeQualityTools(t *testing.T) {
	s := store.NewAt(t.TempDir())
	names := map[string]bool{}
	for _, tool := range DefaultTools(s, nil) {
		names[tool.Name()] = true
	}
	for _, want := range []string{
		"save_evidence_table", "load_evidence_table",
		"save_section_quality_plan", "load_section_quality_plan",
	} {
		if !names[want] {
			t.Fatalf("tool %s missing, names = %#v", want, names)
		}
	}
}

func TestCoordinatorSystemPromptMentionsQualitySteps(t *testing.T) {
	prompt := CoordinatorSystemPrompt(PromptOptions{})
	for _, want := range []string{
		StepEvidenceExtraction, StepSectionQualityPlan,
		"save_evidence_table", "save_section_quality_plan",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q", want)
		}
	}
}
