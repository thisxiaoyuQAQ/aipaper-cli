package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/voocel/agentcore"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/store"
)

func TestWriterToolRefusesWithoutConfirmedReferences(t *testing.T) {
	s := store.NewAt(t.TempDir())
	calls := 0
	tool := NewWriterTool(s, WriterRunnerFunc(func(context.Context, json.RawMessage) (any, error) {
		calls++
		return map[string]any{"called": true}, nil
	}))

	response := executeTool(t, tool, `{"chapter_id":"ch01"}`)
	if response.OK {
		t.Fatalf("writer tool unexpectedly OK: %#v", response)
	}
	if response.Error == nil || response.Error.Code != CodeNoneConfirmed {
		t.Fatalf("writer error = %#v", response.Error)
	}
	if calls != 0 {
		t.Fatalf("writer runner was called %d times", calls)
	}
}

func TestWriterToolRunsWithConfirmedReferences(t *testing.T) {
	s := store.NewAt(t.TempDir())
	writeConfirmed(t, s)
	calls := 0
	tool := NewWriterTool(s, WriterRunnerFunc(func(_ context.Context, args json.RawMessage) (any, error) {
		calls++
		return map[string]any{"args": string(args)}, nil
	}))

	response := executeTool(t, tool, `{"chapter_id":"ch01"}`)
	if !response.OK {
		t.Fatalf("writer tool failed: %#v", response)
	}
	if calls != 1 {
		t.Fatalf("writer runner calls = %d", calls)
	}
}

func TestConfirmedReferencesToolTreatsMissingFileAsZero(t *testing.T) {
	response := executeTool(t, NewConfirmedReferencesTool(store.NewAt(t.TempDir())), `{}`)
	if !response.OK {
		t.Fatalf("confirmed references tool failed: %#v", response)
	}
	data := response.Data.(map[string]any)
	if data["count"].(float64) != 0 {
		t.Fatalf("count = %#v", data["count"])
	}
}

func TestProjectEventDoesNotCopyToolArgs(t *testing.T) {
	ev := agentcore.Event{
		Type:    agentcore.EventToolExecStart,
		Tool:    "writer_run",
		Args:    json.RawMessage(`{"api_key":"secret"}`),
		IsError: false,
	}
	projected := ProjectEvent(ev, time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC))
	data, err := json.Marshal(projected)
	if err != nil {
		t.Fatalf("Marshal(projected) error = %v", err)
	}
	if strings.Contains(string(data), "secret") || strings.Contains(string(data), "api_key") {
		t.Fatalf("projected event leaked args: %s", data)
	}
	if projected.Fields["tool"] != "writer_run" {
		t.Fatalf("fields = %#v", projected.Fields)
	}
}

func executeTool(t *testing.T, tool agentcore.Tool, args string) ToolResponse {
	t.Helper()
	raw, err := tool.Execute(context.Background(), json.RawMessage(args))
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	var response ToolResponse
	if err := json.Unmarshal(raw, &response); err != nil {
		t.Fatalf("Unmarshal(%s) error = %v", raw, err)
	}
	if response.Data != nil {
		normalized, err := json.Marshal(response.Data)
		if err != nil {
			t.Fatalf("Marshal(data) error = %v", err)
		}
		var data map[string]any
		if err := json.Unmarshal(normalized, &data); err == nil {
			response.Data = data
		}
	}
	return response
}

func writeConfirmed(t *testing.T, s store.Store) {
	t.Helper()
	confirmed := contracts.ConfirmedReferences{Items: []contracts.ConfirmedReference{{
		Key:         "smith2024Rag",
		Title:       "RAG",
		Authors:     []string{"Smith"},
		ConfirmedAt: time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC),
	}}}
	if _, err := store.WriteJSON(s.Path("references", "confirmed.json"), confirmed, store.Overwrite); err != nil {
		t.Fatalf("WriteJSON(confirmed) error = %v", err)
	}
}
