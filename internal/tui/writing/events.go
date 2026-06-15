package writing

import (
	"time"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
)

// RuntimeEventKind defines the type of runtime event for TUI consumption.
type RuntimeEventKind string

const (
	EventStepStarted        RuntimeEventKind = "step_started"
	EventStepDone           RuntimeEventKind = "step_done"
	EventStepFailed         RuntimeEventKind = "step_failed"
	EventRoleLog            RuntimeEventKind = "role_log"
	EventContentDelta       RuntimeEventKind = "content_delta"
	EventUsageUpdate        RuntimeEventKind = "usage_update"
	EventChapterStatus      RuntimeEventKind = "chapter_status"
	EventQualityReview      RuntimeEventKind = "quality_review"
	EventCheckpointSaved    RuntimeEventKind = "checkpoint_saved"
	EventExportArtifact     RuntimeEventKind = "export_artifact"
	EventRuntimeDone        RuntimeEventKind = "runtime_done"
	EventRuntimeError       RuntimeEventKind = "runtime_error"
	EventPauseRequested     RuntimeEventKind = "pause_requested"
	EventPaused             RuntimeEventKind = "paused"
	EventResumed            RuntimeEventKind = "resumed"
	EventInstructionQueued  RuntimeEventKind = "instruction_queued"
	EventInstructionApplied RuntimeEventKind = "instruction_applied"
)

// RuntimeEvent is the internal TUI event type, bridging agentcore/litellm events.
type RuntimeEvent struct {
	At        time.Time        `json:"at"`
	Kind      RuntimeEventKind `json:"kind"`
	Role      string           `json:"role,omitempty"`
	Step      string           `json:"step,omitempty"`
	ChapterID string           `json:"chapter_id,omitempty"`
	Message   string           `json:"message,omitempty"`
	Delta     string           `json:"delta,omitempty"`
	Usage     *UsageSnapshot   `json:"usage,omitempty"`
	Fields    map[string]any   `json:"fields,omitempty"`
}

// UsageSnapshot captures token usage and cost information.
type UsageSnapshot struct {
	InputTokens         *int64   `json:"input_tokens,omitempty"`
	OutputTokens        *int64   `json:"output_tokens,omitempty"`
	CacheReadTokens     *int64   `json:"cache_read_tokens,omitempty"`
	CacheCreationTokens *int64   `json:"cache_creation_tokens,omitempty"`
	ContextTokens       *int64   `json:"context_tokens,omitempty"`
	MaxContextTokens    *int64   `json:"max_context_tokens,omitempty"`
	CostUSD             *float64 `json:"cost_usd,omitempty"`
	Model               string   `json:"model,omitempty"`
}

// ChapterStatus represents the state of a chapter in the writing process.
type ChapterStatus string

const (
	ChapterPending       ChapterStatus = "pending"
	ChapterWriting       ChapterStatus = "writing"
	ChapterReviewing     ChapterStatus = "reviewing"
	ChapterVerifying     ChapterStatus = "verifying"
	ChapterRewriting     ChapterStatus = "rewriting"
	ChapterDone          ChapterStatus = "done"
	ChapterNeedsReview   ChapterStatus = "needs_human_review"
	ChapterNeedsRevision ChapterStatus = "needs_revision"
)

// ChapterState holds the current state of a chapter.
type ChapterState struct {
	ID             string        `json:"id"`
	Status         ChapterStatus `json:"status"`
	DraftVersion   int           `json:"draft_version"`
	RevisionRounds int           `json:"revision_rounds"`
	Score          *int          `json:"score,omitempty"`
	CitationScore  *int          `json:"citation_score,omitempty"`
	WordCount      int           `json:"word_count"`
}

// BridgeRunEvent converts contracts.RunEvent to RuntimeEvent for TUI consumption.
// This isolates TUI from agentcore/litellm event details.
func BridgeRunEvent(ev contracts.RunEvent) RuntimeEvent {
	re := RuntimeEvent{
		At:      ev.At,
		Message: ev.Message,
		Fields:  make(map[string]any),
	}

	// Copy fields if present
	if ev.Fields != nil {
		for k, v := range ev.Fields {
			re.Fields[k] = v
		}
	}

	// Map event kinds
	switch ev.Kind {
	case "tool_exec_start":
		re.Kind = EventStepStarted
		if tool, ok := ev.Fields["tool"].(string); ok {
			re.Step = tool
		}
	case "tool_exec_end":
		if isError, ok := ev.Fields["is_error"].(bool); ok && isError {
			re.Kind = EventStepFailed
		} else {
			re.Kind = EventStepDone
		}
		if tool, ok := ev.Fields["tool"].(string); ok {
			re.Step = tool
		}
	case "agent_end":
		re.Kind = EventRuntimeDone
	case "error":
		re.Kind = EventRuntimeError
	// Module-22 bugfix boundary extension: host-side runner events carry the
	// writing event kinds directly (streaming deltas, usage, chapter status,
	// checkpoint markers).
	case string(EventContentDelta):
		re.Kind = EventContentDelta
		if delta, ok := ev.Fields["delta"].(string); ok {
			re.Delta = delta
		}
	case string(EventUsageUpdate), "message_end":
		re.Kind = EventUsageUpdate
		re.Usage = usageFromFields(ev.Fields)
	case string(EventChapterStatus):
		re.Kind = EventChapterStatus
	case string(EventCheckpointSaved):
		re.Kind = EventCheckpointSaved
	case string(EventQualityReview):
		re.Kind = EventQualityReview
	case string(EventExportArtifact):
		re.Kind = EventExportArtifact
	case string(EventPauseRequested):
		re.Kind = EventPauseRequested
	case string(EventPaused):
		re.Kind = EventPaused
	case string(EventResumed):
		re.Kind = EventResumed
	case string(EventInstructionQueued):
		re.Kind = EventInstructionQueued
	case string(EventInstructionApplied):
		re.Kind = EventInstructionApplied
	default:
		re.Kind = EventRoleLog
	}

	// Extract role and chapter id
	if ev.Fields != nil {
		if agent, ok := ev.Fields["agent"].(string); ok {
			re.Role = agent
		}
		if chapterID, ok := ev.Fields["chapter_id"].(string); ok {
			re.ChapterID = chapterID
		}
	}

	return re
}

// usageFromFields builds a UsageSnapshot from projected run-event fields.
func usageFromFields(fields map[string]any) *UsageSnapshot {
	if fields == nil {
		return nil
	}
	usage := &UsageSnapshot{}
	found := false
	if v, ok := fieldInt64(fields, "input_tokens"); ok {
		usage.InputTokens = &v
		found = true
	}
	if v, ok := fieldInt64(fields, "output_tokens"); ok {
		usage.OutputTokens = &v
		found = true
	}
	if v, ok := fieldInt64(fields, "context_tokens"); ok {
		usage.ContextTokens = &v
		found = true
	}
	if v, ok := fieldInt64(fields, "max_context_tokens"); ok {
		usage.MaxContextTokens = &v
		found = true
	}
	if v, ok := fields["cost_usd"].(float64); ok {
		usage.CostUSD = &v
		found = true
	}
	if v, ok := fields["model"].(string); ok && v != "" {
		usage.Model = v
		found = true
	}
	if !found {
		return nil
	}
	return usage
}

func fieldInt64(fields map[string]any, key string) (int64, bool) {
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

// RuntimeEventMsg is a Bubble Tea message carrying a RuntimeEvent.
type RuntimeEventMsg RuntimeEvent

// RuntimeDoneMsg signals runtime completion.
type RuntimeDoneMsg struct {
	Success bool
	Error   error
}

// RuntimeStopRequestedMsg signals that the user requested a stop.
type RuntimeStopRequestedMsg struct{}

// RuntimePauseRequestedMsg signals that the user requested a safe-point pause.
type RuntimePauseRequestedMsg struct{}

// RuntimeResumeRequestedMsg signals that the user requested generation resume.
type RuntimeResumeRequestedMsg struct{}

// RuntimeInstructionSubmittedMsg carries a user instruction to apply at the
// next safe runtime boundary.
type RuntimeInstructionSubmittedMsg struct {
	Text string
}
