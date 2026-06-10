package quality

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/voocel/agentcore"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/store"
)

// ToolResponse mirrors the project-wide tool result contract:
// {ok:true,data} on success, {ok:false,error:{code,message,retryable,details}} on failure.
type ToolResponse struct {
	OK    bool   `json:"ok"`
	Data  any    `json:"data,omitempty"`
	Error *Error `json:"error,omitempty"`
}

type jsonTool struct {
	name        string
	description string
	schema      map[string]any
	execute     func(context.Context, json.RawMessage) ToolResponse
}

func (t jsonTool) Name() string           { return t.name }
func (t jsonTool) Description() string    { return t.description }
func (t jsonTool) Schema() map[string]any { return t.schema }

func (t jsonTool) Execute(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
	data, err := json.Marshal(t.execute(ctx, args))
	if err != nil {
		return nil, err
	}
	return data, nil
}

// Tools returns the quality Host tools for registration into the agent runtime.
func Tools(s store.Store) []agentcore.Tool {
	return []agentcore.Tool{
		NewSaveEvidenceTableTool(s),
		NewLoadEvidenceTableTool(s),
		NewSaveSectionQualityPlanTool(s),
		NewLoadSectionQualityPlanTool(s),
		NewSaveClaimGraphTool(s),
		NewLoadClaimGraphTool(s),
		NewExtractChapterClaimsTool(s),
	}
}

// NewSaveEvidenceTableTool persists a validated evidence table to the store.
func NewSaveEvidenceTableTool(s store.Store) agentcore.Tool {
	return jsonTool{
		name:        "save_evidence_table",
		description: "Validate and atomically save the evidence table to quality/evidence-table.json and .md. Every reference_key must already be confirmed; snippet/fulltext depths require extracted material text.",
		schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"table": map[string]any{
					"type":        "object",
					"description": "EvidenceTable JSON: {generated_at: RFC3339 UTC, items: [Evidence]}",
				},
			},
			"required": []string{"table"},
		},
		execute: func(_ context.Context, args json.RawMessage) ToolResponse {
			var payload struct {
				Table EvidenceTable `json:"table"`
			}
			if err := strictUnmarshal(args, &payload); err != nil {
				return toolError(NewError(CodeEvidenceInvalid, "invalid save_evidence_table arguments: "+err.Error(), false))
			}
			outputs, err := SaveEvidenceTable(s, payload.Table)
			if err != nil {
				if qErr, ok := AsError(err); ok {
					return toolError(qErr)
				}
				return toolError(NewError(CodeEvidenceIOFailed, err.Error(), false))
			}
			return ToolResponse{OK: true, Data: map[string]any{
				"outputs": outputs,
				"count":   len(payload.Table.Items),
			}}
		},
	}
}

// NewLoadEvidenceTableTool reads the evidence table from the store.
func NewLoadEvidenceTableTool(s store.Store) agentcore.Tool {
	return jsonTool{
		name:        "load_evidence_table",
		description: "Load quality/evidence-table.json from the Store and return the structured evidence table.",
		schema: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		execute: func(context.Context, json.RawMessage) ToolResponse {
			table, err := LoadEvidenceTable(s)
			if err != nil {
				if qErr, ok := AsError(err); ok {
					return toolError(qErr)
				}
				return toolError(NewError(CodeEvidenceIOFailed, err.Error(), false))
			}
			return ToolResponse{OK: true, Data: table}
		},
	}
}

func toolError(err Error) ToolResponse {
	return ToolResponse{OK: false, Error: &err}
}

func strictUnmarshal(data json.RawMessage, value any) error {
	if len(data) == 0 {
		return fmt.Errorf("empty arguments")
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	return dec.Decode(value)
}
