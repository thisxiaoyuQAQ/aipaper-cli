package agent

import (
	"fmt"
	"time"

	"github.com/voocel/agentcore"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
)

func ProjectEvent(ev agentcore.Event, at time.Time) contracts.RunEvent {
	if at.IsZero() {
		at = time.Now().UTC()
	}
	fields := map[string]any{"agent": RoleCoordinator}
	if ev.Tool != "" {
		fields["tool"] = ev.Tool
	}
	if ev.ToolID != "" {
		fields["tool_id"] = ev.ToolID
	}
	if ev.IsError {
		fields["is_error"] = true
	}
	if ev.Summary != nil {
		fields["turn_count"] = ev.Summary.TurnCount
		fields["tool_calls"] = ev.Summary.ToolCalls
		fields["tool_errors"] = ev.Summary.ToolErrors
		fields["end_reason"] = string(ev.Summary.EndReason)
	}
	if ev.Progress != nil {
		fields["progress_kind"] = string(ev.Progress.Kind)
		fields["progress_summary"] = ev.Progress.Summary
		if ev.Progress.Tool != "" {
			fields["tool"] = ev.Progress.Tool
		}
		if ev.Progress.Agent != "" {
			fields["agent"] = ev.Progress.Agent
		}
	}
	message := eventMessage(ev)
	if len(fields) == 1 {
		fields = nil
	}
	return contracts.RunEvent{
		At:      at.UTC(),
		Kind:    string(ev.Type),
		Message: message,
		Fields:  fields,
	}
}

func eventMessage(ev agentcore.Event) string {
	switch ev.Type {
	case agentcore.EventToolExecStart:
		return fmt.Sprintf("tool started: %s", ev.Tool)
	case agentcore.EventToolExecEnd:
		if ev.IsError {
			return fmt.Sprintf("tool failed: %s", ev.Tool)
		}
		return fmt.Sprintf("tool finished: %s", ev.Tool)
	case agentcore.EventError:
		if ev.Err != nil {
			return ev.Err.Error()
		}
		return "agent error"
	case agentcore.EventAgentEnd:
		if ev.Summary != nil {
			return fmt.Sprintf("agent ended: %s", ev.Summary.EndReason)
		}
	}
	return ""
}
