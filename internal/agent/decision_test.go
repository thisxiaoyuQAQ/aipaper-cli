package agent

import (
	"errors"
	"testing"
)

func TestParseDecisionRejectsInvalidJSON(t *testing.T) {
	_, err := ParseDecision([]byte(`{"action":`))
	if err == nil {
		t.Fatalf("ParseDecision() unexpectedly succeeded")
	}
	var agentErr Error
	if !errors.As(err, &agentErr) || agentErr.Code != CodeInvalidJSON {
		t.Fatalf("ParseDecision() error = %v", err)
	}
}

func TestParseDecisionRejectsUnknownFieldsAndMissingAction(t *testing.T) {
	tests := [][]byte{
		[]byte(`{"action":"continue","unexpected":true}`),
		[]byte(`{"reason":"missing action"}`),
	}
	for _, input := range tests {
		_, err := ParseDecision(input)
		if err == nil {
			t.Fatalf("ParseDecision(%s) unexpectedly succeeded", input)
		}
		var agentErr Error
		if !errors.As(err, &agentErr) || agentErr.Code != CodeInvalidJSON {
			t.Fatalf("ParseDecision(%s) error = %v", input, err)
		}
	}
}

func TestParseDecisionAcceptsValidDecision(t *testing.T) {
	decision, err := ParseDecision([]byte(`{"action":"continue","tool":"progress_read","reason":"need facts"}`))
	if err != nil {
		t.Fatalf("ParseDecision() error = %v", err)
	}
	if decision.Action != "continue" || decision.Tool != "progress_read" {
		t.Fatalf("decision = %#v", decision)
	}
}
