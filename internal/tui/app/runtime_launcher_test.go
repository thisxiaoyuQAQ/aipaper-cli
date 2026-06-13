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

// TestWritingRuntimeEmitsStartLog asserts the launcher sends a startup
// role_log into the event channel so a static writing screen can be
// attributed to the run goroutine (B) rather than the bridge. The runtime
// is constructed directly (no network, no goroutine) and the sink's public
// path is exercised via systemEvent + sendEvent, mirroring StartWritingRuntime.
func TestWritingRuntimeEmitsStartLog(t *testing.T) {
	rt := &WritingRuntime{msgs: make(chan tea.Msg, 8)}
	rt.sendEvent(rt.systemEvent("role_log", "启动写作运行时（model=test）", nil))

	select {
	case msg := <-rt.msgs:
		ev, ok := msg.(writingtui.RuntimeEventMsg)
		if !ok {
			t.Fatalf("expected RuntimeEventMsg, got %T", msg)
		}
		if ev.Kind != writingtui.EventRoleLog {
			t.Fatalf("kind = %q, want %q", ev.Kind, writingtui.EventRoleLog)
		}
		if ev.Message == "" {
			t.Fatal("startup role_log message is empty")
		}
	default:
		t.Fatal("no startup role_log on the channel")
	}
}

// TestWritingRuntimeHeartbeatEmitsWhileIdle drives the watchdog directly with
// a faked idle channel: it stays open long enough for one short tick, then the
// goroutine stops cleanly. A real blocked WaitForIdle looks identical here.
func TestWritingRuntimeHeartbeatEmitsWhileIdle(t *testing.T) {
	prev := heartbeatInterval
	heartbeatInterval = 10 * time.Millisecond
	t.Cleanup(func() { heartbeatInterval = prev })

	rt := &WritingRuntime{msgs: make(chan tea.Msg, 8)}
	idle := make(chan struct{})
	rt.startHeartbeat(3, idle)

	// One heartbeat should land quickly while idle stays open.
	select {
	case msg := <-rt.msgs:
		ev, ok := msg.(writingtui.RuntimeEventMsg)
		if !ok {
			t.Fatalf("expected RuntimeEventMsg, got %T", msg)
		}
		if ev.Kind != writingtui.EventRoleLog {
			t.Fatalf("kind = %q, want %q", ev.Kind, writingtui.EventRoleLog)
		}
	case <-time.After(time.Second):
		t.Fatal("heartbeat did not fire while idle")
	}

	close(idle)
	rt.heartbeatWG.Wait() // goroutine must release without Stop()

	// After idle closes no further heartbeats arrive.
	rt.msgs = make(chan tea.Msg, 1) // reset for the drain check
	select {
	case msg := <-rt.msgs:
		t.Fatalf("unexpected message after idle closed: %#v", msg)
	case <-time.After(40 * time.Millisecond):
	}
}

// TestWritingRuntimeStopHeartbeatTearsDown ensures Stop-style teardown waits
// for the watchdog so the run goroutine can close the channel safely.
func TestWritingRuntimeStopHeartbeatTearsDown(t *testing.T) {
	prev := heartbeatInterval
	heartbeatInterval = 10 * time.Millisecond
	t.Cleanup(func() { heartbeatInterval = prev })

	rt := &WritingRuntime{msgs: make(chan tea.Msg, 8)}
	idle := make(chan struct{})
	rt.startHeartbeat(0, idle)

	rt.stopHeartbeat()
	rt.heartbeatWG.Wait() // must not block

	// Stopping twice must be a no-op (safe on run-exit + Stop paths).
	rt.stopHeartbeat()
}
