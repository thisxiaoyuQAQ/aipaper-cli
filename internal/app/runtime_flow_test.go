package app

// B8 layer-one acceptance for BUG-20260613-01: a scripted Coordinator
// ChatModel drives the REAL agentcore loop through NewAgentRuntime with the
// real role runners (each backed by its own scripted model). This exercises
// the exact production wiring — tool registration, role tools, guarded
// writes, checkpoints, progress completion — without any network access.

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/voocel/agentcore"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/quality"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/store"
)

func TestAgentRuntimeFullFlowWithScriptedCoordinator(t *testing.T) {
	s, chenKey, _ := setupRunnerStore(t)
	workDir := storeWorkDir(s)

	// Role models
	writerModel := &scriptedChatModel{replies: []string{writerReply(chenKey)}}
	verdictReply, _ := json.Marshal(map[string]any{
		"verdicts": []map[string]any{{
			"claim_id": "claim_001", "support": "supported", "risk_level": "low",
			"verifier_note": "matches the abstract-level evidence",
		}},
	})
	reviewReply, _ := json.Marshal(map[string]any{
		"scores": map[string]int{
			"overall": 88, "citation_consistency": 95, "structure_logic": 86, "coverage": 85, "readability": 90,
		},
		"summary":              "Accepted.",
		"unsupported_claims":   []string{},
		"required_fixes":       []string{},
		"optional_fixes":       []string{},
		"rewrite_instructions": []any{},
	})
	editorModel := &scriptedChatModel{replies: []string{string(verdictReply), string(reviewReply)}}

	var events []contracts.RunEvent
	var eventsMu sync.Mutex
	sink := func(ev contracts.RunEvent) {
		eventsMu.Lock()
		events = append(events, ev)
		eventsMu.Unlock()
	}

	writer, err := NewWriterRunner(RoleRunnerOptions{Config: runnerTestConfig(), Store: s, Model: writerModel, EventSink: sink})
	if err != nil {
		t.Fatalf("NewWriterRunner() error = %v", err)
	}
	editor, err := NewEditorRunner(RoleRunnerOptions{Config: runnerTestConfig(), Store: s, Model: editorModel, EventSink: sink})
	if err != nil {
		t.Fatalf("NewEditorRunner() error = %v", err)
	}
	// Outline / evidence / plan already exist in the fixture store, so the
	// scripted Coordinator starts from write_chapter; the architect runner is
	// covered by its own chain test.
	architectModel := &scriptedChatModel{}
	architect, err := NewArchitectRunner(RoleRunnerOptions{Config: runnerTestConfig(), Store: s, Model: architectModel, EventSink: sink})
	if err != nil {
		t.Fatalf("NewArchitectRunner() error = %v", err)
	}

	coordinator := newToolScriptModel(t,
		scriptedCall("progress_read", `{}`),
		scriptedCall("writer_run", `{"chapter_id":"ch01"}`),
		scriptedCall("extract_chapter_claims", `{"chapter_id":"ch01","draft_version":1}`),
		scriptedCall("editor_run", `{"task":"verify","chapter_id":"ch01"}`),
		scriptedCall("editor_run", `{"task":"review","chapter_id":"ch01"}`),
		scriptedCall("editor_run", `{"task":"commit","chapter_id":"ch01"}`),
	)

	runtime, err := NewAgentRuntime(AgentRuntimeOptions{
		WorkDir:   workDir,
		Config:    runnerTestConfig(),
		Model:     coordinator,
		Writer:    writer,
		Architect: architect,
		Editor:    editor,
		EventSink: sink,
	})
	if err != nil {
		t.Fatalf("NewAgentRuntime() error = %v", err)
	}

	if err := runtime.Agent.Prompt("Start the aipaper writing workflow."); err != nil {
		t.Fatalf("Prompt() error = %v", err)
	}
	runtime.Agent.WaitForIdle()
	if stateErr := runtime.Agent.State().Error; stateErr != "" {
		t.Fatalf("agent error = %v", stateErr)
	}

	// The full chain persisted everything and completed the writing phase.
	if _, err := os.Stat(s.Path("drafts", "ch01", "accepted.md")); err != nil {
		t.Fatalf("accepted.md missing: %v", err)
	}
	graph, err := quality.LoadClaimGraph(s)
	if err != nil || len(graph.Claims) != 1 || graph.Claims[0].Support != quality.SupportSupported {
		t.Fatalf("claim graph = %#v err = %v", graph, err)
	}
	var progress contracts.Progress
	if err := store.ReadJSON(s.ProgressPath(), &progress); err != nil {
		t.Fatalf("read progress: %v", err)
	}
	if progress.Status != "completed" || progress.Phase != "writing_completed" {
		t.Fatalf("progress = %#v", progress)
	}

	eventsMu.Lock()
	defer eventsMu.Unlock()
	kinds := map[string]int{}
	for _, ev := range events {
		kinds[ev.Kind]++
	}
	if kinds["tool_exec_start"] < 6 || kinds["checkpoint_saved"] < 3 || kinds["agent_end"] != 1 {
		t.Fatalf("event kinds = %#v", kinds)
	}
}

// toolScriptModel is a ChatModel whose replies are scripted tool calls.
type toolScriptModel struct {
	t     *testing.T
	mu    sync.Mutex
	steps []agentcore.ToolCall
	idx   int
}

func scriptedCall(name, args string) agentcore.ToolCall {
	return agentcore.ToolCall{Name: name, Args: json.RawMessage(args)}
}

func newToolScriptModel(t *testing.T, steps ...agentcore.ToolCall) *toolScriptModel {
	return &toolScriptModel{t: t, steps: steps}
}

func (m *toolScriptModel) Generate(_ context.Context, _ []agentcore.Message, _ []agentcore.ToolSpec, _ ...agentcore.CallOption) (*agentcore.LLMResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.idx >= len(m.steps) {
		msg := agentcore.Message{
			Role:       agentcore.RoleAssistant,
			Content:    []agentcore.ContentBlock{agentcore.TextBlock("All chapters are committed. Workflow complete.")},
			StopReason: agentcore.StopReasonStop,
			Usage:      &agentcore.Usage{Input: 10, Output: 5, TotalTokens: 15},
			Timestamp:  time.Now(),
		}
		return &agentcore.LLMResponse{Message: msg}, nil
	}
	call := m.steps[m.idx]
	call.ID = fmt.Sprintf("call_%03d", m.idx+1)
	m.idx++
	msg := agentcore.Message{
		Role:       agentcore.RoleAssistant,
		Content:    []agentcore.ContentBlock{agentcore.ToolCallBlock(call)},
		StopReason: agentcore.StopReasonToolUse,
		Usage:      &agentcore.Usage{Input: 10, Output: 5, TotalTokens: 15},
		Timestamp:  time.Now(),
	}
	return &agentcore.LLMResponse{Message: msg}, nil
}

func (m *toolScriptModel) GenerateStream(ctx context.Context, messages []agentcore.Message, tools []agentcore.ToolSpec, opts ...agentcore.CallOption) (<-chan agentcore.StreamEvent, error) {
	resp, err := m.Generate(ctx, messages, tools, opts...)
	if err != nil {
		return nil, err
	}
	ch := make(chan agentcore.StreamEvent, 1)
	ch <- agentcore.StreamEvent{Type: agentcore.StreamEventDone, Message: resp.Message}
	close(ch)
	return ch, nil
}

func (m *toolScriptModel) SupportsTools() bool { return true }

// storeWorkDir recovers the workDir from the store root (root is
// <workDir>/output/aipaper).
func storeWorkDir(s store.Store) string {
	return filepath.Dir(filepath.Dir(s.Root()))
}
