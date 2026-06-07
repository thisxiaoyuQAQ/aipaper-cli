package agent

import (
	"errors"
	"fmt"
)

const (
	CodeInvalidJSON         = "AGENT_INVALID_JSON"
	CodeNoneConfirmed       = "REFERENCE_NONE_CONFIRMED"
	CodeToolExecutionFailed = "AGENT_TOOL_FAILED"
)

type Error struct {
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	Retryable bool           `json:"retryable"`
	Details   map[string]any `json:"details,omitempty"`
}

func (e Error) Error() string {
	if e.Code == "" {
		return e.Message
	}
	if e.Message == "" {
		return e.Code
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func AgentError(code, message string, retryable bool) Error {
	return Error{Code: code, Message: message, Retryable: retryable}
}

func AsError(err error) (Error, bool) {
	var agentErr Error
	if errors.As(err, &agentErr) {
		return agentErr, true
	}
	return Error{}, false
}
