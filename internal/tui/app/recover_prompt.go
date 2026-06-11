package app

import (
	"fmt"
	"strings"

	runtimeapp "github.com/thisxiaoyuQAQ/aipaper-cli/internal/app"
)

type RecoverPromptAction string

const (
	RecoverActionNone     RecoverPromptAction = ""
	RecoverActionContinue RecoverPromptAction = "continue"
	RecoverActionRestart  RecoverPromptAction = "restart"
	RecoverActionExit     RecoverPromptAction = "exit"
)

type RecoverPromptModel struct {
	probe          ProbeResult
	action         RecoverPromptAction
	confirmRestart bool
}

type WritingResumeData struct {
	Recovery       runtimeapp.RecoveryResult
	RecoveryPrompt string
}

func NewRecoverPromptModel(probe ProbeResult) RecoverPromptModel {
	return RecoverPromptModel{probe: probe}
}

func (m RecoverPromptModel) UpdateKey(key string) RecoverPromptModel {
	key = strings.ToLower(strings.TrimSpace(key))
	if m.confirmRestart {
		switch key {
		case "y":
			m.action = RecoverActionRestart
		case "n", "esc":
			m.confirmRestart = false
			m.action = RecoverActionNone
		case "q", "ctrl+c":
			m.action = RecoverActionExit
		default:
			m.action = RecoverActionNone
		}
		return m
	}
	switch key {
	case "enter", "c":
		m.action = RecoverActionContinue
	case "r":
		m.confirmRestart = true
		m.action = RecoverActionNone
	case "n", "esc":
		m.confirmRestart = false
		m.action = RecoverActionNone
	case "q", "ctrl+c":
		m.action = RecoverActionExit
	}
	return m
}

func (m RecoverPromptModel) View() string {
	var b strings.Builder
	b.WriteString("Recover saved progress\n\n")
	if m.probe.CheckpointStep > 0 {
		fmt.Fprintf(&b, "Checkpoint: step %d", m.probe.CheckpointStep)
		if m.probe.CheckpointPhase != "" {
			fmt.Fprintf(&b, " (%s)", m.probe.CheckpointPhase)
		}
		b.WriteByte('\n')
	}
	if m.probe.CheckpointNextExpected != "" {
		fmt.Fprintf(&b, "Next expected: %s\n", m.probe.CheckpointNextExpected)
	}
	if m.probe.ProgressStatus != "" {
		fmt.Fprintf(&b, "Progress: %s\n", m.probe.ProgressStatus)
	}
	if m.probe.QualityMode != "" {
		modeDesc := qualityModeDescription(m.probe.QualityMode)
		fmt.Fprintf(&b, "Quality mode: %s (%s)\n", m.probe.QualityMode, modeDesc)
	}
	if len(m.probe.CheckpointErrors) > 0 {
		b.WriteString("\nCheckpoint warnings:\n")
		for _, err := range m.probe.CheckpointErrors {
			fmt.Fprintf(&b, "- %s\n", err)
		}
	}
	if !m.probe.HasQualityArtifacts && m.probe.QualityMode != "" && m.probe.QualityMode != "fast" {
		b.WriteString("\n⚠ Compatibility mode: Quality artifacts missing, will continue with warnings-only.\n")
	}
	if m.confirmRestart {
		b.WriteString("\nRestart keeps existing output files intact. Press y to confirm, n to cancel.\n")
		return b.String()
	}
	b.WriteString("\nEnter/c: continue   r: restart flow   q: exit\n")
	return b.String()
}

func qualityModeDescription(mode string) string {
	switch mode {
	case "fast":
		return "quick draft"
	case "enhanced":
		return "quality balanced"
	case "strict":
		return "strict evidence"
	default:
		return "default"
	}
}

func (m RecoverPromptModel) Action() RecoverPromptAction {
	return m.action
}
