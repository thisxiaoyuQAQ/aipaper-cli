package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/voocel/agentcore"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/checkpoint"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/quality"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/store"
)

type ToolResponse struct {
	OK    bool   `json:"ok"`
	Data  any    `json:"data,omitempty"`
	Error *Error `json:"error,omitempty"`
}

type WriterRunner interface {
	RunWriter(ctx context.Context, args json.RawMessage) (any, error)
}

type WriterRunnerFunc func(ctx context.Context, args json.RawMessage) (any, error)

func (f WriterRunnerFunc) RunWriter(ctx context.Context, args json.RawMessage) (any, error) {
	return f(ctx, args)
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

func DefaultTools(s store.Store, writer WriterRunner) []agentcore.Tool {
	tools := []agentcore.Tool{
		NewRequirementsTool(s),
		NewProgressTool(s),
		NewConfirmedReferencesTool(s),
		NewCheckpointValidationTool(s),
		NewWriterTool(s, writer),
	}
	return append(tools, quality.Tools(s)...)
}

func NewRequirementsTool(s store.Store) agentcore.Tool {
	return jsonTool{
		name:        "requirements_read",
		description: "Read requirements.json from the Store and return structured requirements facts.",
		schema:      emptyObjectSchema(),
		execute: func(context.Context, json.RawMessage) ToolResponse {
			var req contracts.Requirements
			if err := store.ReadJSON(s.RequirementsPath(), &req); err != nil {
				return toolError(CodeToolExecutionFailed, fmt.Sprintf("read requirements.json: %v", err), false)
			}
			return toolOK(req)
		},
	}
}

func NewProgressTool(s store.Store) agentcore.Tool {
	return jsonTool{
		name:        "progress_read",
		description: "Read progress.json from the Store and return current phase, step, chapter, and status facts.",
		schema:      emptyObjectSchema(),
		execute: func(context.Context, json.RawMessage) ToolResponse {
			var progress contracts.Progress
			if err := store.ReadJSON(s.ProgressPath(), &progress); err != nil {
				return toolError(CodeToolExecutionFailed, fmt.Sprintf("read progress.json: %v", err), false)
			}
			return toolOK(progress)
		},
	}
}

func NewConfirmedReferencesTool(s store.Store) agentcore.Tool {
	return jsonTool{
		name:        "references_confirmed_read",
		description: "Read confirmed references and return count and keys. Missing confirmed.json means zero confirmed references.",
		schema:      emptyObjectSchema(),
		execute: func(context.Context, json.RawMessage) ToolResponse {
			confirmed, err := readConfirmedReferences(s)
			if err != nil {
				return toolError(CodeToolExecutionFailed, fmt.Sprintf("read confirmed references: %v", err), false)
			}
			keys := make([]string, 0, len(confirmed.Items))
			for _, item := range confirmed.Items {
				keys = append(keys, item.Key)
			}
			return toolOK(map[string]any{
				"count": len(confirmed.Items),
				"keys":  keys,
			})
		},
	}
}

func NewCheckpointValidationTool(s store.Store) agentcore.Tool {
	return jsonTool{
		name:        "checkpoint_validate_latest",
		description: "Validate latest checkpoint artifacts and return recovery consistency facts.",
		schema:      emptyObjectSchema(),
		execute: func(context.Context, json.RawMessage) ToolResponse {
			return toolOK(checkpoint.ValidateLatest(s))
		},
	}
}

func NewWriterTool(s store.Store, runner WriterRunner) agentcore.Tool {
	if runner == nil {
		runner = WriterRunnerFunc(func(context.Context, json.RawMessage) (any, error) {
			return nil, AgentError(CodeToolExecutionFailed, "writer runtime is not implemented yet", false)
		})
	}
	return jsonTool{
		name:        "writer_run",
		description: "Invoke Writer for a chapter. The tool refuses to run when references/confirmed.json has no confirmed references.",
		schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"chapter_id": map[string]any{"type": "string"},
			},
			"required": []string{"chapter_id"},
		},
		execute: func(ctx context.Context, args json.RawMessage) ToolResponse {
			confirmed, err := readConfirmedReferences(s)
			if err != nil {
				return toolError(CodeToolExecutionFailed, fmt.Sprintf("read confirmed references: %v", err), false)
			}
			if len(confirmed.Items) == 0 {
				return toolError(CodeNoneConfirmed, "Writer cannot run before the user confirms at least one reference", false)
			}
			result, err := runner.RunWriter(ctx, args)
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

func readConfirmedReferences(s store.Store) (contracts.ConfirmedReferences, error) {
	var confirmed contracts.ConfirmedReferences
	err := store.ReadJSON(s.Path("references", "confirmed.json"), &confirmed)
	if errors.Is(err, os.ErrNotExist) {
		return contracts.ConfirmedReferences{Items: []contracts.ConfirmedReference{}}, nil
	}
	if err != nil {
		return contracts.ConfirmedReferences{}, err
	}
	return confirmed, nil
}

func toolOK(data any) ToolResponse {
	return ToolResponse{OK: true, Data: data}
}

func toolError(code, message string, retryable bool) ToolResponse {
	err := AgentError(code, message, retryable)
	return ToolResponse{OK: false, Error: &err}
}

func emptyObjectSchema() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	}
}
