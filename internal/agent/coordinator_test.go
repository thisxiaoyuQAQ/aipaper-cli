package agent

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/store"
)

func TestCoordinatorStepDoesNotCallWriterWithoutConfirmedReferences(t *testing.T) {
	s := store.NewAt(t.TempDir())
	writerCalls := 0
	tools := DefaultTools(s, WriterRunnerFunc(func(context.Context, json.RawMessage) (any, error) {
		writerCalls++
		return map[string]any{"ok": true}, nil
	}))
	coordinator := Coordinator{
		Decisions: scriptedDecisions{
			[]byte(`{"action":"call_tool","tool":"writer_run","facts":{"chapter_id":"ch01"}}`),
		},
		Tools: NewAgentCoreToolRunner(tools),
	}

	result, err := coordinator.Step(context.Background())
	if err != nil {
		t.Fatalf("Step() error = %v", err)
	}
	if result.Response == nil || result.Response.OK {
		t.Fatalf("response = %#v", result.Response)
	}
	if result.Response.Error == nil || result.Response.Error.Code != CodeNoneConfirmed {
		t.Fatalf("response error = %#v", result.Response.Error)
	}
	if writerCalls != 0 {
		t.Fatalf("writer was called %d times", writerCalls)
	}
	if len(coordinator.Observations) != 1 || coordinator.Observations[0].Tool != "writer_run" {
		t.Fatalf("observations = %#v", coordinator.Observations)
	}
}

func TestCoordinatorStepReturnsStructuredErrorForInvalidJSON(t *testing.T) {
	coordinator := Coordinator{Decisions: scriptedDecisions{[]byte(`{"action":`)}}
	_, err := coordinator.Step(context.Background())
	if err == nil {
		t.Fatalf("Step() unexpectedly succeeded")
	}
	var agentErr Error
	if !errors.As(err, &agentErr) || agentErr.Code != CodeInvalidJSON {
		t.Fatalf("Step() error = %v", err)
	}
}

func TestCoordinatorStepDoesNotInferRewriteFromReviewFacts(t *testing.T) {
	coordinator := Coordinator{
		Decisions: scriptedDecisions{
			[]byte(`{"action":"call_tool","tool":"review_fact","facts":{"passed":false,"overall":60}}`),
			[]byte(`{"action":"finish","reason":"coordinator decides later"}`),
		},
		Tools: staticToolRunner{responses: map[string]ToolResponse{
			"review_fact": toolOK(map[string]any{"passed": false, "overall": 60}),
		}},
	}

	first, err := coordinator.Step(context.Background())
	if err != nil {
		t.Fatalf("first Step() error = %v", err)
	}
	if first.Done || first.Tool != "review_fact" {
		t.Fatalf("first result = %#v", first)
	}
	second, err := coordinator.Step(context.Background())
	if err != nil {
		t.Fatalf("second Step() error = %v", err)
	}
	if !second.Done {
		t.Fatalf("second result = %#v", second)
	}
}

type scriptedDecisions [][]byte

func (s scriptedDecisions) NextDecision(_ context.Context, observations []ToolObservation) ([]byte, error) {
	if len(observations) >= len(s) {
		return []byte(`{"action":"finish"}`), nil
	}
	return s[len(observations)], nil
}

type staticToolRunner struct {
	responses map[string]ToolResponse
}

func (r staticToolRunner) RunTool(_ context.Context, name string, _ json.RawMessage) (ToolResponse, error) {
	response, ok := r.responses[name]
	if !ok {
		return ToolResponse{}, AgentError(CodeToolExecutionFailed, "missing test response", false)
	}
	return response, nil
}
