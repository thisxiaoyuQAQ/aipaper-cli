package agent

// B3 (BUG-20260613-01 plan B) boundary additions: architect_run and editor_run
// tools, following the writer_run pattern exactly — the Host injects facts and
// pre-checks, the runner only produces content, validated persistence happens
// inside the runner through the existing guarded write paths. Existing tool
// semantics in tools.go are not changed.

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/voocel/agentcore"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/store"
)

// ArchitectRunner produces outline / evidence_extraction / section_quality_plan
// artifacts. args carries {"task": "..."}.
type ArchitectRunner interface {
	RunArchitect(ctx context.Context, args json.RawMessage) (any, error)
}

type ArchitectRunnerFunc func(ctx context.Context, args json.RawMessage) (any, error)

func (f ArchitectRunnerFunc) RunArchitect(ctx context.Context, args json.RawMessage) (any, error) {
	return f(ctx, args)
}

// EditorRunner produces verifier verdicts and chapter reviews, and executes
// the deterministic commit / human_review terminal steps. args carries
// {"task": "...", "chapter_id": "..."}.
type EditorRunner interface {
	RunEditor(ctx context.Context, args json.RawMessage) (any, error)
}

type EditorRunnerFunc func(ctx context.Context, args json.RawMessage) (any, error)

func (f EditorRunnerFunc) RunEditor(ctx context.Context, args json.RawMessage) (any, error) {
	return f(ctx, args)
}

// RoleTools returns the full tool set for the real multi-agent runtime:
// DefaultTools plus the architect_run / editor_run role tools.
func RoleTools(s store.Store, writer WriterRunner, architect ArchitectRunner, editor EditorRunner) []agentcore.Tool {
	return append(DefaultTools(s, writer), NewArchitectTool(s, architect), NewEditorTool(s, editor))
}

func NewArchitectTool(s store.Store, runner ArchitectRunner) agentcore.Tool {
	if runner == nil {
		runner = ArchitectRunnerFunc(func(context.Context, json.RawMessage) (any, error) {
			return nil, AgentError(CodeToolExecutionFailed, "architect runtime is not implemented yet", false)
		})
	}
	return jsonTool{
		name:        "architect_run",
		description: "Invoke Architect for one task: outline (chapter outline), evidence_extraction (evidence table from confirmed references and parsed materials), or section_quality_plan (per-chapter quality plan). evidence_extraction and section_quality_plan refuse to run without confirmed references.",
		schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"task": map[string]any{
					"type": "string",
					"enum": []string{"outline", StepEvidenceExtraction, StepSectionQualityPlan},
				},
			},
			"required": []string{"task"},
		},
		execute: func(ctx context.Context, args json.RawMessage) ToolResponse {
			var payload struct {
				Task string `json:"task"`
			}
			if err := json.Unmarshal(args, &payload); err != nil {
				return toolError(CodeInvalidJSON, fmt.Sprintf("parse architect_run args: %v", err), false)
			}
			if payload.Task != "outline" {
				confirmed, err := readConfirmedReferences(s)
				if err != nil {
					return toolError(CodeToolExecutionFailed, fmt.Sprintf("read confirmed references: %v", err), false)
				}
				if len(confirmed.Items) == 0 {
					return toolError(CodeNoneConfirmed, "Architect quality steps cannot run before the user confirms at least one reference", false)
				}
			}
			result, err := runner.RunArchitect(ctx, args)
			if err != nil {
				if agentErr, ok := AsError(err); ok {
					return ToolResponse{OK: false, Error: &agentErr}
				}
				return toolError(CodeToolExecutionFailed, err.Error(), false)
			}
			return toolOK(result)
		},
	}
}

func NewEditorTool(s store.Store, runner EditorRunner) agentcore.Tool {
	if runner == nil {
		runner = EditorRunnerFunc(func(context.Context, json.RawMessage) (any, error) {
			return nil, AgentError(CodeToolExecutionFailed, "editor runtime is not implemented yet", false)
		})
	}
	return jsonTool{
		name:        "editor_run",
		description: "Invoke Editor/Verifier for one chapter task: verify (verifier verdicts over the chapter's claim graph nodes), review (structured review with rewrite_instructions plus chapter-gate and quality-gate facts), commit (persist accepted.md after the gate passed), or human_review (mark a chapter past the rewrite limit as terminal needs_human_review). The tool refuses to run without confirmed references.",
		schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"task": map[string]any{
					"type": "string",
					"enum": []string{"verify", "review", "commit", "human_review"},
				},
				"chapter_id": map[string]any{"type": "string"},
			},
			"required": []string{"task", "chapter_id"},
		},
		execute: func(ctx context.Context, args json.RawMessage) ToolResponse {
			confirmed, err := readConfirmedReferences(s)
			if err != nil {
				return toolError(CodeToolExecutionFailed, fmt.Sprintf("read confirmed references: %v", err), false)
			}
			if len(confirmed.Items) == 0 {
				return toolError(CodeNoneConfirmed, "Editor cannot run before the user confirms at least one reference", false)
			}
			result, err := runner.RunEditor(ctx, args)
			if err != nil {
				if agentErr, ok := AsError(err); ok {
					return ToolResponse{OK: false, Error: &agentErr}
				}
				return toolError(CodeToolExecutionFailed, err.Error(), false)
			}
			return toolOK(result)
		},
	}
}

// roleToolsPromptSections extends the Coordinator prompt with the multi-agent
// workflow over the role tools. Decision rules stay with the Coordinator; this
// only describes what each tool executes.
func roleToolsPromptSections() []string {
	return []string{
		"Role tools: architect_run(task=outline|evidence_extraction|section_quality_plan) produces and persists Architect artifacts; writer_run(chapter_id) writes or rewrites one chapter through the evidence guard; editor_run(task=verify|review|commit|human_review, chapter_id) runs the Verifier/Editor and the deterministic terminal steps. Each role tool persists its artifacts and records the step checkpoint itself.",
		"Standard workflow: read requirements/progress/confirmed facts first; then architect_run outline -> evidence_extraction -> section_quality_plan; then for each pending chapter: writer_run -> extract_chapter_claims -> editor_run verify -> editor_run review -> decide from the returned gate facts: rewrite via writer_run (the rewrite instructions are injected automatically), or editor_run commit when the gate passed, or editor_run human_review when the rewrite limit is exhausted. Finish with a short summary when no chapter is pending.",
		"Resume rule: always read progress_read first and continue from the first unfinished step; never repeat a step whose artifacts a valid recovery prompt lists.",
	}
}
