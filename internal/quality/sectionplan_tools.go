package quality

import (
	"context"
	"encoding/json"

	"github.com/voocel/agentcore"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/store"
)

// NewSaveSectionQualityPlanTool persists a validated section quality plan to the store.
func NewSaveSectionQualityPlanTool(s store.Store) agentcore.Tool {
	return jsonTool{
		name:        "save_section_quality_plan",
		description: "Validate and atomically save the section quality plan to quality/section-quality-plan.json and .md. Every chapter_id must match an outline chapter and every required evidence id must exist in the evidence table.",
		schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"plan": map[string]any{
					"type":        "object",
					"description": "SectionQualityPlan JSON: {generated_at: RFC3339 UTC, sections: [SectionPlan]}",
				},
			},
			"required": []string{"plan"},
		},
		execute: func(_ context.Context, args json.RawMessage) ToolResponse {
			var payload struct {
				Plan SectionQualityPlan `json:"plan"`
			}
			if err := strictUnmarshal(args, &payload); err != nil {
				return toolError(NewError(CodeSectionPlanInvalid, "invalid save_section_quality_plan arguments: "+err.Error(), false))
			}
			outputs, err := SaveSectionQualityPlan(s, payload.Plan)
			if err != nil {
				if qErr, ok := AsError(err); ok {
					return toolError(qErr)
				}
				return toolError(NewError(CodeSectionPlanIOFailed, err.Error(), false))
			}
			return ToolResponse{OK: true, Data: map[string]any{
				"outputs":  outputs,
				"sections": len(payload.Plan.Sections),
			}}
		},
	}
}

// NewLoadSectionQualityPlanTool reads the section quality plan from the store.
func NewLoadSectionQualityPlanTool(s store.Store) agentcore.Tool {
	return jsonTool{
		name:        "load_section_quality_plan",
		description: "Load quality/section-quality-plan.json from the Store and return the structured section quality plan.",
		schema: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		execute: func(context.Context, json.RawMessage) ToolResponse {
			plan, err := LoadSectionQualityPlan(s)
			if err != nil {
				if qErr, ok := AsError(err); ok {
					return toolError(qErr)
				}
				return toolError(NewError(CodeSectionPlanIOFailed, err.Error(), false))
			}
			return ToolResponse{OK: true, Data: plan}
		},
	}
}
