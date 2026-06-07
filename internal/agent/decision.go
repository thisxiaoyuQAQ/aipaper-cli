package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

type Decision struct {
	Action string         `json:"action"`
	Agent  string         `json:"agent,omitempty"`
	Tool   string         `json:"tool,omitempty"`
	Reason string         `json:"reason,omitempty"`
	Facts  map[string]any `json:"facts,omitempty"`
}

func ParseDecision(data []byte) (Decision, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var decision Decision
	if err := dec.Decode(&decision); err != nil {
		return Decision{}, AgentError(CodeInvalidJSON, fmt.Sprintf("parse coordinator decision: %v", err), false)
	}
	var extra any
	if err := dec.Decode(&extra); err == nil {
		return Decision{}, AgentError(CodeInvalidJSON, "parse coordinator decision: trailing JSON value", false)
	} else if err != nil && err != io.EOF {
		return Decision{}, AgentError(CodeInvalidJSON, fmt.Sprintf("parse coordinator decision: %v", err), false)
	}
	if decision.Action == "" {
		return Decision{}, AgentError(CodeInvalidJSON, "parse coordinator decision: action is required", false)
	}
	return decision, nil
}
