package app

import (
	"fmt"
	"strings"

	runtimeapp "github.com/thisxiaoyuQAQ/aipaper-cli/internal/app"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/i18n"
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
	i18n           i18n.T
}

type WritingResumeData struct {
	Recovery       runtimeapp.RecoveryResult
	RecoveryPrompt string
}

func NewRecoverPromptModel(probe ProbeResult, trs ...i18n.T) RecoverPromptModel {
	tr := i18n.New("")
	if len(trs) > 0 && !trs[0].IsZero() {
		tr = trs[0]
	}
	return RecoverPromptModel{probe: probe, i18n: tr}
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
	b.WriteString(m.i18n.Text(i18n.RecoverTitle))
	b.WriteString("\n\n")
	if m.probe.CheckpointStep > 0 {
		fmt.Fprintf(&b, m.i18n.Text(i18n.RecoverCheckpoint), m.probe.CheckpointStep)
		if m.probe.CheckpointPhase != "" {
			fmt.Fprintf(&b, " (%s)", m.probe.CheckpointPhase)
		}
		b.WriteByte('\n')
	}
	if m.probe.CheckpointNextExpected != "" {
		fmt.Fprintf(&b, m.i18n.Text(i18n.RecoverNextExpected)+"\n", m.probe.CheckpointNextExpected)
	}
	if m.probe.ProgressStatus != "" {
		fmt.Fprintf(&b, m.i18n.Text(i18n.RecoverProgress)+"\n", m.probe.ProgressStatus)
	}
	if m.probe.QualityMode != "" {
		modeDesc := m.qualityModeDescription(m.probe.QualityMode)
		fmt.Fprintf(&b, m.i18n.Text(i18n.RecoverQualityMode)+"\n", m.probe.QualityMode, modeDesc)
	}
	if len(m.probe.CheckpointErrors) > 0 {
		b.WriteString("\n")
		b.WriteString(m.i18n.Text(i18n.RecoverCheckpointWarnings))
		b.WriteString("\n")
		for _, err := range m.probe.CheckpointErrors {
			fmt.Fprintf(&b, "- %s\n", err)
		}
	}
	if !m.probe.HasQualityArtifacts && m.probe.QualityMode != "" && m.probe.QualityMode != "fast" {
		b.WriteString("\n")
		b.WriteString(m.i18n.Text(i18n.RecoverCompatibilityMode))
		b.WriteString("\n")
	}
	if m.confirmRestart {
		b.WriteString("\n")
		b.WriteString(m.i18n.Text(i18n.RecoverRestartConfirm))
		b.WriteString("\n")
		return b.String()
	}
	b.WriteString("\n")
	b.WriteString(m.i18n.Text(i18n.RecoverFooter))
	b.WriteString("\n")
	return b.String()
}

func (m RecoverPromptModel) qualityModeDescription(mode string) string {
	switch mode {
	case "fast":
		return m.i18n.Text(i18n.RecoverQualityFast)
	case "enhanced":
		return m.i18n.Text(i18n.RecoverQualityEnhanced)
	case "strict":
		return m.i18n.Text(i18n.RecoverQualityStrict)
	default:
		return m.i18n.Text(i18n.RecoverQualityDefault)
	}
}

func (m RecoverPromptModel) Action() RecoverPromptAction {
	return m.action
}
