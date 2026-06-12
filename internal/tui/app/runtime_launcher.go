package app

// B4/B6/B7 (BUG-20260613-01 plan B): the missing TUI -> AgentRuntime wiring.
// When RootModel enters ScreenWriting it starts a WritingRuntime: the real
// Host runtime (NewAgentRuntime with the writer/architect/editor LLM runners),
// a kickoff prompt, a continuation loop guarded by progress.json and a
// round cap (MaxTurns=12 per round would otherwise truncate the workflow),
// and an event pump that bridges contracts.RunEvent into Bubble Tea messages.
// Ctrl+C two-phase stop maps to agent.Abort(): step tools record checkpoints
// after every persisted artifact, so the latest checkpoint is always a safe
// resume point. Steering input is deliberately out of scope (recorded in the
// bug vault); recovery injects the probe's recovery prompt (B7).

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/voocel/agentcore"

	runtimeapp "github.com/thisxiaoyuQAQ/aipaper-cli/internal/app"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/config"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/store"
	writingtui "github.com/thisxiaoyuQAQ/aipaper-cli/internal/tui/writing"
)

// maxContinuationRounds bounds the Prompt/Continue loop so a confused model
// cannot spin forever. Each round allows up to the coordinator MaxTurns.
const maxContinuationRounds = 30

// WritingRuntimeStarter launches the real runtime for the writing screen.
// Injectable so tests can avoid provider config and goroutines.
type WritingRuntimeStarter func(workDir, recoveryPrompt string) (*WritingRuntime, error)

// writingRuntimeStartedMsg reports the launcher start result to RootModel.
type writingRuntimeStartedMsg struct {
	runtime *WritingRuntime
	err     error
}

// WritingRuntime owns the background agent run and the event channel pumped
// into the Bubble Tea program.
type WritingRuntime struct {
	agent *agentcore.Agent
	msgs  chan tea.Msg

	stopRequested atomic.Bool
	closeOnce     sync.Once

	// cumulative usage across coordinator and role runners
	usageMu     sync.Mutex
	totalInput  int64
	totalOutput int64
	totalCost   float64
	hasCost     bool
	model       string

	endReason atomic.Value // agentcore.EndReason of the last round
	lastErr   atomic.Value // error message of the last error event
}

// StartWritingRuntime is the production WritingRuntimeStarter.
func StartWritingRuntime(workDir, recoveryPrompt string) (*WritingRuntime, error) {
	cfg, _, err := config.Load(config.LoadOptions{WorkDir: workDir})
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	if cfg.Provider == "" || cfg.Model == "" {
		return nil, fmt.Errorf("provider and model are required to start the writing runtime; configure aipaper.json first")
	}

	rt := &WritingRuntime{msgs: make(chan tea.Msg, 512)}
	s := store.New(workDir)
	sink := rt.makeSink()
	runnerOpts := runtimeapp.RoleRunnerOptions{Config: cfg, Store: s, EventSink: sink}
	writer, err := runtimeapp.NewWriterRunner(runnerOpts)
	if err != nil {
		return nil, fmt.Errorf("build writer runner: %w", err)
	}
	architect, err := runtimeapp.NewArchitectRunner(runnerOpts)
	if err != nil {
		return nil, fmt.Errorf("build architect runner: %w", err)
	}
	editor, err := runtimeapp.NewEditorRunner(runnerOpts)
	if err != nil {
		return nil, fmt.Errorf("build editor runner: %w", err)
	}

	runtime, err := runtimeapp.NewAgentRuntime(runtimeapp.AgentRuntimeOptions{
		WorkDir:        workDir,
		Config:         cfg,
		RecoveryPrompt: recoveryPrompt,
		Writer:         writer,
		Architect:      architect,
		Editor:         editor,
		EventSink:      sink,
	})
	if err != nil {
		return nil, err
	}
	rt.agent = runtime.Agent
	rt.model = runtime.Model

	go rt.run(workDir, recoveryPrompt)
	return rt, nil
}

// NextEventCmd returns a Bubble Tea command that waits for the next runtime
// message. A closed channel yields nil, which Bubble Tea ignores.
func (rt *WritingRuntime) NextEventCmd() tea.Cmd {
	if rt == nil {
		return nil
	}
	return func() tea.Msg {
		msg, ok := <-rt.msgs
		if !ok {
			return nil
		}
		return msg
	}
}

// Stop implements the second phase of the Ctrl+C stop: abort the current run.
// Step tools already persisted artifacts + checkpoint for every completed
// step, so aborting mid-step only loses the in-flight step.
func (rt *WritingRuntime) Stop() {
	if rt == nil {
		return
	}
	rt.stopRequested.Store(true)
	if rt.agent != nil {
		rt.agent.Abort()
	}
}

func (rt *WritingRuntime) run(workDir, recoveryPrompt string) {
	defer rt.closeOnce.Do(func() { close(rt.msgs) })

	kickoff := buildKickoffPrompt(recoveryPrompt != "")
	if err := rt.agent.Prompt(kickoff); err != nil {
		rt.msgs <- writingtui.RuntimeDoneMsg{Error: fmt.Errorf("start coordinator: %w", err)}
		return
	}

	for round := 0; ; round++ {
		rt.agent.WaitForIdle()

		if rt.stopRequested.Load() {
			// Two-phase stop: report the safe state; the writing model marks
			// itself canceled on checkpoint_saved while stopping.
			rt.sendEvent(contracts.RunEvent{
				At:      time.Now().UTC(),
				Kind:    "checkpoint_saved",
				Message: "Stopped. Progress is saved at the last checkpoint; continue from the recovery prompt next time.",
				Fields:  map[string]any{"agent": "system"},
			})
			return
		}

		progress, ok, err := runtimeapp.LoadProgress(workDir)
		if err == nil && ok && progress.Status == "completed" {
			rt.msgs <- writingtui.RuntimeDoneMsg{Success: true}
			return
		}

		reason, _ := rt.endReason.Load().(agentcore.EndReason)
		switch reason {
		case agentcore.EndReasonAborted:
			rt.sendEvent(contracts.RunEvent{
				At:      time.Now().UTC(),
				Kind:    "checkpoint_saved",
				Message: "Run aborted. Progress is saved at the last checkpoint.",
				Fields:  map[string]any{"agent": "system"},
			})
			return
		case agentcore.EndReasonError:
			msg, _ := rt.lastErr.Load().(string)
			if msg == "" {
				msg = "coordinator run failed"
			}
			rt.msgs <- writingtui.RuntimeDoneMsg{Error: fmt.Errorf("%s", msg)}
			return
		}

		if round >= maxContinuationRounds {
			rt.msgs <- writingtui.RuntimeDoneMsg{Error: fmt.Errorf("writing did not complete within %d coordinator rounds; progress is saved at the last checkpoint", maxContinuationRounds)}
			return
		}

		// EndReasonStop with unfinished progress, or EndReasonMaxTurns: nudge
		// the Coordinator to re-read progress and keep going in the same
		// conversation context.
		if err := rt.agent.Continue(); err != nil {
			if err := rt.agent.Prompt(continuationPrompt); err != nil {
				rt.msgs <- writingtui.RuntimeDoneMsg{Error: fmt.Errorf("continue coordinator: %w", err)}
				return
			}
		}
	}
}

// makeSink converts contracts.RunEvent into TUI messages. It filters the noisy
// agentcore message lifecycle kinds, folds usage into cumulative totals, and
// keeps agent_end/error for the launcher loop instead of the TUI (a per-round
// agent_end must not mark the writing screen as done).
func (rt *WritingRuntime) makeSink() func(contracts.RunEvent) {
	return func(ev contracts.RunEvent) {
		switch ev.Kind {
		case "agent_start", "turn_start", "turn_end", "message_start", "message_update", "tool_exec_update", "retry":
			return
		case "agent_end":
			if reason, ok := ev.Fields["end_reason"].(string); ok {
				rt.endReason.Store(agentcore.EndReason(reason))
			}
			return
		case "error":
			if ev.Message != "" {
				rt.lastErr.Store(ev.Message)
			}
			rt.sendEvent(contracts.RunEvent{
				At:      ev.At,
				Kind:    "role_log",
				Message: "error: " + ev.Message,
				Fields:  map[string]any{"agent": "system", "is_error": true},
			})
			return
		case "message_end", "usage_update":
			if cumulative, ok := rt.accumulateUsage(ev); ok {
				rt.sendEvent(cumulative)
			}
			return
		}
		rt.sendEvent(ev)
	}
}

// accumulateUsage folds per-call usage events from the coordinator and the
// role runners into one cumulative snapshot for the TUI metrics row.
func (rt *WritingRuntime) accumulateUsage(ev contracts.RunEvent) (contracts.RunEvent, bool) {
	input, okIn := usageField(ev.Fields, "input_tokens")
	output, okOut := usageField(ev.Fields, "output_tokens")
	if !okIn && !okOut {
		return contracts.RunEvent{}, false
	}
	rt.usageMu.Lock()
	defer rt.usageMu.Unlock()
	rt.totalInput += input
	rt.totalOutput += output
	if cost, ok := ev.Fields["cost_usd"].(float64); ok {
		rt.totalCost += cost
		rt.hasCost = true
	}
	if model, ok := ev.Fields["model"].(string); ok && model != "" {
		rt.model = model
	}
	fields := map[string]any{
		"agent":         "system",
		"input_tokens":  rt.totalInput,
		"output_tokens": rt.totalOutput,
		"model":         rt.model,
	}
	if rt.hasCost {
		fields["cost_usd"] = rt.totalCost
	}
	if context, ok := usageField(ev.Fields, "context_tokens"); ok {
		fields["context_tokens"] = context
	}
	return contracts.RunEvent{At: ev.At, Kind: "usage_update", Fields: fields}, true
}

func usageField(fields map[string]any, key string) (int64, bool) {
	switch v := fields[key].(type) {
	case int64:
		return v, true
	case int:
		return int64(v), true
	case float64:
		return int64(v), true
	default:
		return 0, false
	}
}

func (rt *WritingRuntime) sendEvent(ev contracts.RunEvent) {
	msg := writingtui.RuntimeEventMsg(writingtui.BridgeRunEvent(ev))
	select {
	case rt.msgs <- msg:
	default:
		// The TUI pump fell behind; dropping a display event is preferable to
		// blocking the agent loop.
	}
}

func buildKickoffPrompt(resuming bool) string {
	if resuming {
		return "Resume the aipaper writing workflow. Follow the recovery context in your system prompt: read progress_read and checkpoint_validate_latest first, do not repeat completed steps, and continue from next_expected until every chapter is committed or needs_human_review."
	}
	return "Start the aipaper writing workflow for this run. Read requirements_read, progress_read, and references_confirmed_read first, then follow the standard workflow from your system prompt (architect_run outline -> evidence_extraction -> section_quality_plan, then per chapter writer_run -> extract_chapter_claims -> editor_run verify -> editor_run review -> rewrite or commit) until every chapter is committed or needs_human_review. Then finish with a short summary."
}

const continuationPrompt = "Continue the aipaper writing workflow: read progress_read, find the first unfinished step, and proceed. Do not repeat completed steps."
