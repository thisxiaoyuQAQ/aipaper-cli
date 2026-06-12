package app

// Tests for the module-22 bugfix TUI runtime wiring: event pump, stop
// forwarding, start-failure handling, and the launcher event sink.

import (
	"errors"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
	writingtui "github.com/thisxiaoyuQAQ/aipaper-cli/internal/tui/writing"
)

func TestRootModelPumpsRuntimeEventsIntoWritingModel(t *testing.T) {
	rt := &WritingRuntime{msgs: make(chan tea.Msg, 8)}
	model := NewRootModel(RootOptions{
		WorkDir:       t.TempDir(),
		InitialScreen: ScreenWriting,
		RuntimeStarter: func(string, string) (*WritingRuntime, error) {
			return rt, nil
		},
	})

	updated, cmd := model.Update(writingRuntimeStartedMsg{runtime: rt})
	root := updated.(RootModel)
	if root.runtime != rt || cmd == nil {
		t.Fatalf("runtime not stored or pump not started")
	}

	rt.msgs <- writingtui.RuntimeEventMsg(writingtui.RuntimeEvent{
		At:   time.Now(),
		Kind: writingtui.EventStepStarted,
		Role: "coordinator",
		Step: "create_outline",
	})
	rt.msgs <- writingtui.RuntimeDoneMsg{Success: true}
	close(rt.msgs)

	// Pump: run the command, feed the message back, repeat until done.
	var doneSeen bool
	for i := 0; i < 5 && cmd != nil && !doneSeen; i++ {
		msg := cmd()
		if msg == nil {
			break
		}
		if _, ok := msg.(writingtui.RuntimeDoneMsg); ok {
			doneSeen = true
		}
		updated, cmd = root.Update(msg)
		root = updated.(RootModel)
	}
	if !doneSeen {
		t.Fatalf("RuntimeDoneMsg never reached the root model")
	}
	if !root.Writing.Done() {
		t.Fatalf("writing model not done after RuntimeDoneMsg")
	}
}

func TestRootModelStopRequestAbortsRuntime(t *testing.T) {
	rt := &WritingRuntime{msgs: make(chan tea.Msg, 1)}
	model := NewRootModel(RootOptions{
		WorkDir:       t.TempDir(),
		InitialScreen: ScreenWriting,
	})
	model.runtime = rt

	updated, _ := model.Update(writingtui.RuntimeStopRequestedMsg{})
	root := updated.(RootModel)
	if !rt.stopRequested.Load() {
		t.Fatalf("stop request did not reach the runtime")
	}
	_ = root
}

func TestRootModelRuntimeStartFailureStaysOnWritingScreen(t *testing.T) {
	model := NewRootModel(RootOptions{
		WorkDir:       t.TempDir(),
		InitialScreen: ScreenWriting,
	})

	updated, _ := model.Update(writingRuntimeStartedMsg{err: errors.New("provider and model are required")})
	root := updated.(RootModel)
	if root.Writing.Err() == nil {
		t.Fatalf("writing model should carry the start error")
	}
	// A failed start must not slide into the export screen on the next key.
	next, _ := root.updateWriting("enter")
	root = next.(RootModel)
	if root.CurrentScreen != ScreenWriting {
		t.Fatalf("CurrentScreen = %q, want %q", root.CurrentScreen, ScreenWriting)
	}
}

func TestLauncherSinkFiltersAndAccumulatesUsage(t *testing.T) {
	rt := &WritingRuntime{msgs: make(chan tea.Msg, 8)}
	sink := rt.makeSink()

	// Noisy lifecycle kinds are dropped.
	sink(contracts.RunEvent{Kind: "message_update", Fields: map[string]any{"delta": "x"}})
	sink(contracts.RunEvent{Kind: "turn_end"})
	// agent_end is kept for the launcher loop, not forwarded to the TUI.
	sink(contracts.RunEvent{Kind: "agent_end", Fields: map[string]any{"end_reason": "max_turns"}})
	if len(rt.msgs) != 0 {
		t.Fatalf("filtered events leaked into the channel: %d", len(rt.msgs))
	}

	// Usage folds into cumulative totals.
	sink(contracts.RunEvent{Kind: "usage_update", Fields: map[string]any{
		"agent": "writer", "input_tokens": int64(100), "output_tokens": int64(50), "model": "gpt-test",
	}})
	sink(contracts.RunEvent{Kind: "message_end", Fields: map[string]any{
		"agent": "coordinator", "input_tokens": int64(40), "output_tokens": int64(10),
	}})
	if len(rt.msgs) != 2 {
		t.Fatalf("usage events = %d, want 2", len(rt.msgs))
	}
	<-rt.msgs
	second := (<-rt.msgs).(writingtui.RuntimeEventMsg)
	if second.Kind != writingtui.EventUsageUpdate || second.Usage == nil {
		t.Fatalf("second usage event = %#v", second)
	}
	if *second.Usage.InputTokens != 140 || *second.Usage.OutputTokens != 60 || second.Usage.Model != "gpt-test" {
		t.Fatalf("cumulative usage = %#v", second.Usage)
	}

	// Step events pass through to the bridge.
	sink(contracts.RunEvent{Kind: "tool_exec_start", Fields: map[string]any{"tool": "writer_run", "agent": "coordinator"}})
	stepMsg := (<-rt.msgs).(writingtui.RuntimeEventMsg)
	if stepMsg.Kind != writingtui.EventStepStarted || stepMsg.Step != "writer_run" {
		t.Fatalf("step event = %#v", stepMsg)
	}

	if reason, _ := rt.endReason.Load().(string); reason != "" {
		// endReason stores agentcore.EndReason, not string
		t.Fatalf("endReason stored as string %q", reason)
	}
}
