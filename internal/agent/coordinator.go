package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/voocel/agentcore"
)

type DecisionProvider interface {
	NextDecision(ctx context.Context, observations []ToolObservation) ([]byte, error)
}

type ToolRunner interface {
	RunTool(ctx context.Context, name string, args json.RawMessage) (ToolResponse, error)
}

type ToolObservation struct {
	Tool     string       `json:"tool"`
	Response ToolResponse `json:"response"`
}

type Coordinator struct {
	Decisions    DecisionProvider
	Tools        ToolRunner
	Observations []ToolObservation
}

type StepResult struct {
	Decision Decision      `json:"decision"`
	Tool     string        `json:"tool,omitempty"`
	Response *ToolResponse `json:"response,omitempty"`
	Done     bool          `json:"done"`
}

func (c *Coordinator) Step(ctx context.Context) (StepResult, error) {
	if c.Decisions == nil {
		return StepResult{}, AgentError(CodeInvalidJSON, "coordinator decision provider is required", false)
	}
	raw, err := c.Decisions.NextDecision(ctx, append([]ToolObservation(nil), c.Observations...))
	if err != nil {
		return StepResult{}, err
	}
	decision, err := ParseDecision(raw)
	if err != nil {
		return StepResult{}, err
	}
	switch decision.Action {
	case "finish":
		return StepResult{Decision: decision, Done: true}, nil
	case "call_tool":
		if decision.Tool == "" {
			return StepResult{}, AgentError(CodeInvalidJSON, "call_tool decision requires tool", false)
		}
		if c.Tools == nil {
			return StepResult{}, AgentError(CodeInvalidJSON, "coordinator tool runner is required", false)
		}
		args, err := json.Marshal(decision.Facts)
		if err != nil {
			return StepResult{}, AgentError(CodeInvalidJSON, fmt.Sprintf("marshal tool args: %v", err), false)
		}
		response, err := c.Tools.RunTool(ctx, decision.Tool, args)
		if err != nil {
			return StepResult{}, err
		}
		c.Observations = append(c.Observations, ToolObservation{Tool: decision.Tool, Response: response})
		return StepResult{Decision: decision, Tool: decision.Tool, Response: &response}, nil
	default:
		return StepResult{}, AgentError(CodeInvalidJSON, fmt.Sprintf("unsupported coordinator action %q", decision.Action), false)
	}
}

type AgentCoreToolRunner struct {
	tools map[string]agentcore.Tool
}

func NewAgentCoreToolRunner(tools []agentcore.Tool) AgentCoreToolRunner {
	index := map[string]agentcore.Tool{}
	for _, tool := range tools {
		index[tool.Name()] = tool
	}
	return AgentCoreToolRunner{tools: index}
}

func (r AgentCoreToolRunner) RunTool(ctx context.Context, name string, args json.RawMessage) (ToolResponse, error) {
	tool, ok := r.tools[name]
	if !ok {
		return ToolResponse{}, AgentError(CodeToolExecutionFailed, fmt.Sprintf("tool %q is not registered", name), false)
	}
	raw, err := tool.Execute(ctx, args)
	if err != nil {
		return ToolResponse{}, err
	}
	var response ToolResponse
	if err := json.Unmarshal(raw, &response); err != nil {
		return ToolResponse{}, AgentError(CodeToolExecutionFailed, fmt.Sprintf("parse tool %q response: %v", name, err), false)
	}
	return response, nil
}
