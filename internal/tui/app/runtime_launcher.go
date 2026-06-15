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
	"strings"
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

// heartbeatInterval is how often the watchdog emits a "still waiting" event
// while the agent loop is blocked on a single WaitForIdle. It keeps the TUI
// visibly alive even when the real LLM call hangs with no streaming deltas.
// A var (not const) so tests can shorten the tick.
var heartbeatInterval = 15 * time.Second

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
	closed        atomic.Bool
	closeOnce     sync.Once

	// diag records startup-chain nodes to output/aipaper/runtime.log so a
	// real-run hang can be located after the fact. Nil = no logging.
	diag *diagLogger

	// heartbeatWG tracks the watchdog goroutine so Stop/run-exit can wait for
	// it to release; heartbeatStop closes the watchdog.
	heartbeatMu   sync.Mutex
	heartbeatStop chan struct{}
	heartbeatWG   sync.WaitGroup

	// cumulative usage across coordinator and role runners
	usageMu     sync.Mutex
	totalInput  int64
	totalOutput int64
	totalCost   float64
	hasCost     bool
	model       string

	// activity keeps the latest meaningful runtime context for heartbeat logs.
	activityMu sync.Mutex
	activity   runtimeActivity

	endReason atomic.Value // agentcore.EndReason of the last round
	lastErr   atomic.Value // error message of the last error event

	controlMu           sync.Mutex
	pauseRequested      bool
	paused              bool
	pendingInstructions []string
	resumeCh            chan struct{}
}

type runtimeActivity struct {
	at        time.Time
	kind      string
	agent     string
	step      string
	chapterID string
	status    string
	phase     string
	message   string
	model     string
	isError   bool
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

	rt := &WritingRuntime{msgs: make(chan tea.Msg, 512), diag: newFileDiagLogger(workDir), resumeCh: make(chan struct{})}
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

	// Start signal BEFORE the goroutine: if the writing screen stays static
	// after this, the run goroutine itself failed to launch (panic/blocked),
	// which is otherwise invisible. resuming flags the recovery path.
	rt.diag.logf("start writing runtime model=%s resuming=%v", rt.model, recoveryPrompt != "")
	rt.sendEvent(rt.systemEvent("role_log", fmt.Sprintf("启动写作运行时（model=%s）", rt.model), nil))

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
	rt.signalResume()
	if rt.agent != nil {
		rt.agent.Abort()
	}
}

func (rt *WritingRuntime) RequestPause() {
	if rt == nil {
		return
	}
	rt.controlMu.Lock()
	if !rt.paused {
		rt.pauseRequested = true
	}
	rt.controlMu.Unlock()
	rt.sendEvent(rt.systemEvent(string(writingtui.EventPauseRequested), "已请求在最近安全点暂停", nil))
}

func (rt *WritingRuntime) Resume() {
	if rt == nil {
		return
	}
	rt.controlMu.Lock()
	wasPaused := rt.paused || rt.pauseRequested
	rt.paused = false
	rt.pauseRequested = false
	rt.controlMu.Unlock()
	if wasPaused {
		rt.signalResume()
		rt.sendEvent(rt.systemEvent(string(writingtui.EventResumed), "继续生成", nil))
	}
}

func (rt *WritingRuntime) SubmitInstruction(text string) {
	if rt == nil {
		return
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	rt.controlMu.Lock()
	rt.pendingInstructions = append(rt.pendingInstructions, text)
	pending := len(rt.pendingInstructions)
	rt.controlMu.Unlock()
	rt.sendEvent(rt.systemEvent(string(writingtui.EventInstructionQueued), "人工指令已排队，将在最近安全边界生效", map[string]any{"pending_instructions": pending}))
}

func (rt *WritingRuntime) signalResume() {
	rt.controlMu.Lock()
	old := rt.resumeCh
	rt.resumeCh = make(chan struct{})
	rt.controlMu.Unlock()
	if old == nil {
		return
	}
	select {
	case <-old:
	default:
		close(old)
	}
}

func (rt *WritingRuntime) run(workDir, recoveryPrompt string) {
	defer rt.closeOnce.Do(func() {
		rt.closed.Store(true)
		close(rt.msgs)
	})
	defer rt.stopHeartbeat()

	kickoff := buildKickoffPrompt(recoveryPrompt != "")
	if err := rt.agent.Prompt(kickoff); err != nil {
		rt.diag.logf("coordinator prompt failed: %v", err)
		rt.msgs <- writingtui.RuntimeDoneMsg{Error: fmt.Errorf("start coordinator: %w", err)}
		return
	}
	// Prompt returned nil: the run goroutine and the coordinator turn are
	// launched. A hang hereafter means the real LLM call blocked with no
	// streaming deltas — the watchdog below makes that visible.
	rt.diag.logf("coordinator started")
	rt.sendEvent(rt.systemEvent("role_log", "coordinator 已启动，开始写作流程", nil))

	for round := 0; ; round++ {
		rt.waitForIdleWithHeartbeat(round)

		if rt.stopRequested.Load() {
			// Two-phase stop: report the safe state; the writing model marks
			// itself canceled on checkpoint_saved while stopping.
			rt.diag.logf("stop requested at round=%d", round)
			rt.sendStopCheckpoint("Stopped. Progress is saved at the last checkpoint; continue from the recovery prompt next time.")
			return
		}

		if !rt.pauseAtSafePoint(round) {
			if rt.stopRequested.Load() {
				rt.diag.logf("stop requested while paused at round=%d", round)
				rt.sendStopCheckpoint("Stopped. Progress is saved at the last checkpoint; continue from the recovery prompt next time.")
			}
			return
		}

		if rt.stopRequested.Load() {
			rt.diag.logf("stop requested after pause boundary at round=%d", round)
			rt.sendStopCheckpoint("Stopped. Progress is saved at the last checkpoint; continue from the recovery prompt next time.")
			return
		}

		progress, ok, err := runtimeapp.LoadProgress(workDir)
		if err == nil && ok && progress.Status == "completed" {
			rt.diag.logf("progress completed at round=%d", round)
			rt.msgs <- writingtui.RuntimeDoneMsg{Success: true}
			return
		}

		reason, _ := rt.endReason.Load().(agentcore.EndReason)
		switch reason {
		case agentcore.EndReasonAborted:
			rt.diag.logf("run aborted at round=%d", round)
			rt.sendStopCheckpoint("Run aborted. Progress is saved at the last checkpoint.")
			return
		case agentcore.EndReasonError:
			msg, _ := rt.lastErr.Load().(string)
			if msg == "" {
				msg = "coordinator run failed"
			}
			rt.diag.logf("run error at round=%d: %s", round, msg)
			rt.msgs <- writingtui.RuntimeDoneMsg{Error: fmt.Errorf("%s", msg)}
			return
		}

		if round >= maxContinuationRounds {
			rt.diag.logf("round cap reached (%d)", maxContinuationRounds)
			rt.msgs <- writingtui.RuntimeDoneMsg{Error: fmt.Errorf("writing did not complete within %d coordinator rounds; progress is saved at the last checkpoint", maxContinuationRounds)}
			return
		}

		// EndReasonStop with unfinished progress, or EndReasonMaxTurns: nudge
		// the Coordinator to re-read progress and keep going in the same
		// conversation context. Queued human instructions are applied at this
		// boundary by using a fresh prompt instead of a bare Continue.
		instructions := rt.drainInstructions()
		if len(instructions) > 0 {
			prompt := buildContinuationPrompt(instructions)
			rt.sendEvent(rt.systemEvent(string(writingtui.EventInstructionApplied), "人工指令已注入后续生成", map[string]any{"pending_instructions": 0}))
			if err := rt.agent.Prompt(prompt); err != nil {
				rt.diag.logf("instruction continuation failed at round=%d: %v", round, err)
				rt.msgs <- writingtui.RuntimeDoneMsg{Error: fmt.Errorf("continue coordinator with instruction: %w", err)}
				return
			}
			continue
		}
		if err := rt.agent.Continue(); err != nil {
			if err := rt.agent.Prompt(continuationPrompt); err != nil {
				rt.diag.logf("continue failed at round=%d: %v", round, err)
				rt.msgs <- writingtui.RuntimeDoneMsg{Error: fmt.Errorf("continue coordinator: %w", err)}
				return
			}
		}
	}
}

func (rt *WritingRuntime) sendStopCheckpoint(message string) {
	rt.sendEvent(contracts.RunEvent{
		At:      time.Now().UTC(),
		Kind:    "checkpoint_saved",
		Message: message,
		Fields:  map[string]any{"agent": "system", "stop_requested": true},
	})
}

func (rt *WritingRuntime) pauseAtSafePoint(round int) bool {
	rt.controlMu.Lock()
	if !rt.pauseRequested {
		rt.controlMu.Unlock()
		return true
	}
	rt.pauseRequested = false
	rt.paused = true
	resumeCh := rt.resumeCh
	rt.controlMu.Unlock()

	rt.diag.logf("pause at safe boundary round=%d", round)
	rt.sendEvent(rt.systemEvent(string(writingtui.EventPaused), "已在安全边界暂停；将从最新 checkpoint 继续", nil))

	select {
	case <-resumeCh:
		return !rt.stopRequested.Load()
	}
}

func (rt *WritingRuntime) drainInstructions() []string {
	rt.controlMu.Lock()
	defer rt.controlMu.Unlock()
	if len(rt.pendingInstructions) == 0 {
		return nil
	}
	items := append([]string(nil), rt.pendingInstructions...)
	rt.pendingInstructions = nil
	return items
}

func buildContinuationPrompt(instructions []string) string {
	if len(instructions) == 0 {
		return continuationPrompt
	}
	var b strings.Builder
	b.WriteString(continuationPrompt)
	b.WriteString("\n\nUser additional instructions queued during this run:\n")
	for _, instruction := range instructions {
		instruction = strings.TrimSpace(instruction)
		if instruction != "" {
			b.WriteString("- ")
			b.WriteString(instruction)
			b.WriteString("\n")
		}
	}
	b.WriteString("Apply these instructions to future planning, writing, and review decisions where they do not conflict with confirmed references, checkpoint recovery, or quality gates.")
	return b.String()
}

// waitForIdleWithHeartbeat blocks until the current agent turn finishes while a
// watchdog emits periodic "still waiting" role_log events. If the real LLM
// call hangs with no streaming deltas, the TUI still shows the round and how
// long it has been blocked instead of freezing on a static screen.
func (rt *WritingRuntime) waitForIdleWithHeartbeat(round int) {
	idle := make(chan struct{})
	rt.startHeartbeat(round, idle)
	defer close(idle)
	rt.agent.WaitForIdle()
}

// startHeartbeat launches a watchdog goroutine that ticks every heartbeatInterval
// while the agent is blocked on WaitForIdle. It stops when idle is closed or the
// runtime shuts down. startHeartbeat/stopHeartbeat are paired and re-entrant per
// round via heartbeatMu.
func (rt *WritingRuntime) startHeartbeat(round int, idle <-chan struct{}) {
	rt.heartbeatMu.Lock()
	defer rt.heartbeatMu.Unlock()
	// Replace any prior ticker channel (defensive; rounds are sequential).
	if rt.heartbeatStop != nil {
		close(rt.heartbeatStop)
		rt.heartbeatWG.Wait()
	}
	stop := make(chan struct{})
	rt.heartbeatStop = stop
	started := time.Now().UTC()
	rt.heartbeatWG.Add(1)
	go func() {
		defer rt.heartbeatWG.Done()
		ticker := time.NewTicker(heartbeatInterval)
		defer ticker.Stop()
		for {
			select {
			case <-idle:
				return
			case <-stop:
				return
			case <-ticker.C:
				now := time.Now().UTC()
				elapsed := now.Sub(started).Round(time.Second)
				message, fields, details := rt.heartbeatPayload(round, elapsed, now)
				rt.diag.logf("heartbeat round=%d elapsed=%s wait_target=agent_idle phase=%q chapter=%q last_event_kind=%q last_event_agent=%q last_event=%q last_event_age=%s model=%q",
					round, elapsed, details.phase, details.chapter, details.lastEventKind, details.lastEventAgent, details.lastEvent, details.lastEventAge, details.model)
				rt.sendEvent(rt.systemEvent("role_log", message, fields))
			}
		}
	}()
}

type heartbeatDetails struct {
	phase          string
	chapter        string
	lastEvent      string
	lastEventKind  string
	lastEventAgent string
	lastEventAge   string
	model          string
}

func (rt *WritingRuntime) heartbeatPayload(round int, elapsed time.Duration, now time.Time) (string, map[string]any, heartbeatDetails) {
	activity := rt.activitySnapshot()
	details := heartbeatDetails{
		phase:          activity.phaseLabel(),
		chapter:        firstNonEmpty(activity.chapterID, "未确定"),
		lastEvent:      activity.eventLabel(),
		lastEventKind:  firstNonEmpty(activity.kind, "none"),
		lastEventAgent: firstNonEmpty(activity.agent, "unknown"),
		lastEventAge:   "未知",
		model:          firstNonEmpty(activity.model, rt.currentModel(), "未配置"),
	}
	if !activity.at.IsZero() {
		age := now.Sub(activity.at)
		if age < 0 {
			age = 0
		}
		details.lastEventAge = age.Round(time.Second).String()
	}

	message := fmt.Sprintf("写作等待中：等待 coordinator/LLM 当前轮完成 | round=%d | 已等待=%s | 阶段=%s | 章节=%s | 最近事件=%s | 距最近事件=%s | model=%s",
		round, elapsed, details.phase, details.chapter, details.lastEvent, details.lastEventAge, details.model)
	fields := map[string]any{
		"heartbeat":        true,
		"wait_target":      "agent_idle",
		"round":            round,
		"elapsed":          elapsed.String(),
		"phase":            details.phase,
		"chapter_id":       details.chapter,
		"last_event":       details.lastEvent,
		"last_event_kind":  details.lastEventKind,
		"last_event_agent": details.lastEventAgent,
		"last_event_age":   details.lastEventAge,
		"model":            details.model,
	}
	return message, fields, details
}

func (rt *WritingRuntime) recordActivity(ev contracts.RunEvent) {
	if isHeartbeatEvent(ev) {
		return
	}
	activity := runtimeActivity{
		at:      ev.At,
		kind:    ev.Kind,
		message: strings.TrimSpace(ev.Message),
		model:   rt.currentModel(),
	}
	if activity.at.IsZero() {
		activity.at = time.Now().UTC()
	}
	if ev.Fields != nil {
		activity.agent = fieldString(ev.Fields, "agent")
		activity.step = firstNonEmpty(fieldString(ev.Fields, "tool"), fieldString(ev.Fields, "step"))
		activity.chapterID = fieldString(ev.Fields, "chapter_id")
		activity.status = fieldString(ev.Fields, "status")
		activity.phase = firstNonEmpty(fieldString(ev.Fields, "phase"), fieldString(ev.Fields, "progress_kind"))
		activity.message = firstNonEmpty(activity.message, fieldString(ev.Fields, "progress_summary"))
		activity.model = firstNonEmpty(fieldString(ev.Fields, "model"), activity.model)
		activity.isError, _ = ev.Fields["is_error"].(bool)
	}
	if activity.agent == "" {
		activity.agent = defaultAgentForKind(activity.kind)
	}
	if !activity.hasSignal() {
		return
	}
	rt.activityMu.Lock()
	rt.activity = activity
	rt.activityMu.Unlock()
}

func (rt *WritingRuntime) activitySnapshot() runtimeActivity {
	rt.activityMu.Lock()
	defer rt.activityMu.Unlock()
	return rt.activity
}

func (rt *WritingRuntime) currentModel() string {
	rt.usageMu.Lock()
	defer rt.usageMu.Unlock()
	return rt.model
}

func (a runtimeActivity) hasSignal() bool {
	return a.kind != "" || a.message != "" || a.step != "" || a.chapterID != "" || a.status != "" || a.phase != ""
}

func (a runtimeActivity) phaseLabel() string {
	if a.at.IsZero() {
		return "启动/规划"
	}
	if a.phase != "" {
		return a.phase
	}
	if a.step != "" {
		return a.step
	}
	if a.status != "" {
		return a.status
	}
	switch a.kind {
	case string(writingtui.EventChapterStatus):
		return "章节状态更新"
	case string(writingtui.EventCheckpointSaved):
		return "保存 checkpoint"
	case string(writingtui.EventUsageUpdate), "message_end":
		return "用量统计"
	case "message_start":
		return "等待模型响应"
	case "agent_start", "turn_start":
		return "coordinator 推理"
	case string(writingtui.EventRoleLog):
		return "运行日志"
	}
	return firstNonEmpty(a.kind, "启动/规划")
}

func (a runtimeActivity) eventLabel() string {
	if a.at.IsZero() {
		return "尚未收到运行事件"
	}
	agent := firstNonEmpty(a.agent, "unknown")
	switch a.kind {
	case "tool_exec_start":
		if a.step != "" {
			return fmt.Sprintf("%s 启动工具 %s", agent, a.step)
		}
	case "tool_exec_end":
		if a.step != "" {
			if a.isError || strings.Contains(strings.ToLower(a.message), "failed") {
				return fmt.Sprintf("%s 工具失败 %s", agent, a.step)
			}
			return fmt.Sprintf("%s 完成工具 %s", agent, a.step)
		}
	case "message_start":
		return fmt.Sprintf("%s 已向模型发送请求，等待模型响应", agent)
	case "agent_start":
		return fmt.Sprintf("%s 开始运行", agent)
	case "turn_start":
		return fmt.Sprintf("%s 开始新一轮推理", agent)
	case "agent_end":
		return fmt.Sprintf("%s 当前轮结束", agent)
	case string(writingtui.EventChapterStatus):
		if a.chapterID != "" && a.status != "" {
			return fmt.Sprintf("%s 章节 %s 状态 %s", agent, a.chapterID, a.status)
		}
	case string(writingtui.EventContentDelta):
		if a.chapterID != "" {
			return fmt.Sprintf("%s 正在输出章节 %s 正文", agent, a.chapterID)
		}
		return fmt.Sprintf("%s 正在输出正文", agent)
	case string(writingtui.EventCheckpointSaved):
		return fmt.Sprintf("%s 保存 checkpoint", agent)
	case string(writingtui.EventUsageUpdate), "message_end":
		return fmt.Sprintf("%s 更新 token 用量", agent)
	}
	if a.message != "" {
		if agent != "" && agent != "system" && !strings.HasPrefix(a.message, agent+" ") {
			return agent + " " + a.message
		}
		return a.message
	}
	if a.step != "" {
		return fmt.Sprintf("%s 处理 %s", agent, a.step)
	}
	return firstNonEmpty(a.kind, "未知事件")
}

func isHeartbeatEvent(ev contracts.RunEvent) bool {
	if ev.Fields == nil {
		return false
	}
	heartbeat, _ := ev.Fields["heartbeat"].(bool)
	return heartbeat
}

func defaultAgentForKind(kind string) string {
	switch kind {
	case "agent_start", "agent_end", "turn_start", "turn_end", "message_start", "message_update", "message_end", "tool_exec_start", "tool_exec_end", "tool_exec_update", "retry":
		return "coordinator"
	default:
		return "system"
	}
}

func fieldString(fields map[string]any, key string) string {
	if fields == nil {
		return ""
	}
	value, _ := fields[key].(string)
	return strings.TrimSpace(value)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func shouldRecordFilteredActivity(ev contracts.RunEvent) bool {
	switch ev.Kind {
	case "agent_start", "agent_end", "turn_start", "message_start", "retry":
		return true
	case "tool_exec_update":
		return fieldString(ev.Fields, "progress_summary") != "" || fieldString(ev.Fields, "tool") != ""
	default:
		return false
	}
}

// stopHeartbeat tears down any active watchdog. Called on run exit.
func (rt *WritingRuntime) stopHeartbeat() {
	rt.heartbeatMu.Lock()
	stop := rt.heartbeatStop
	rt.heartbeatStop = nil
	rt.heartbeatMu.Unlock()
	if stop != nil {
		close(stop)
		rt.heartbeatWG.Wait()
	}
}

// systemEvent builds a contracts.RunEvent with the system agent tag and a
// fresh timestamp, merging optional extra fields.
func (rt *WritingRuntime) systemEvent(kind, message string, extra map[string]any) contracts.RunEvent {
	fields := map[string]any{"agent": "system"}
	for k, v := range extra {
		fields[k] = v
	}
	return contracts.RunEvent{At: time.Now().UTC(), Kind: kind, Message: message, Fields: fields}
}

// makeSink converts contracts.RunEvent into TUI messages. It filters the noisy
// agentcore message lifecycle kinds, folds usage into cumulative totals, and
// keeps agent_end/error for the launcher loop instead of the TUI (a per-round
// agent_end must not mark the writing screen as done).
func (rt *WritingRuntime) makeSink() func(contracts.RunEvent) {
	return func(ev contracts.RunEvent) {
		switch ev.Kind {
		case "agent_start", "turn_start", "turn_end", "message_start", "message_update", "tool_exec_update", "retry":
			if shouldRecordFilteredActivity(ev) {
				rt.recordActivity(ev)
			}
			return
		case "agent_end":
			rt.recordActivity(ev)
			if reason, ok := ev.Fields["end_reason"].(string); ok {
				rt.endReason.Store(agentcore.EndReason(reason))
			}
			return
		case "error":
			if ev.Message != "" {
				rt.lastErr.Store(ev.Message)
			}
			rt.recordActivity(ev)
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
	if rt.closed.Load() {
		return
	}
	rt.recordActivity(ev)
	msg := writingtui.RuntimeEventMsg(writingtui.BridgeRunEvent(ev))
	defer func() { _ = recover() }()
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
