package app

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/checkpoint"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/store"
)

type RecoveryResult struct {
	OK             bool     `json:"ok"`
	Store          string   `json:"store"`
	ResumedFrom    int      `json:"resumed_from,omitempty"`
	Phase          string   `json:"phase,omitempty"`
	ProgressPhase  string   `json:"progress_phase,omitempty"`
	ProgressStatus string   `json:"progress_status,omitempty"`
	CurrentChapter string   `json:"current_chapter,omitempty"`
	NextExpected   string   `json:"next_expected,omitempty"`
	Checked        []string `json:"checked,omitempty"`
	Errors         []string `json:"errors,omitempty"`
	RecoveryPrompt string   `json:"recovery_prompt,omitempty"`
	RunUpdated     bool     `json:"run_updated,omitempty"`
}

func Recover(workDir string) (RecoveryResult, error) {
	return recoverAt(workDir, time.Now().UTC())
}

func recoverAt(workDir string, now time.Time) (RecoveryResult, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	s := store.New(workDir)
	result := RecoveryResult{Store: s.Root()}

	progress, ok, err := LoadProgress(workDir)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("read progress.json: %v", err))
		return result, nil
	}
	if !ok {
		result.Errors = append(result.Errors, "progress.json does not exist")
		return result, nil
	}
	result.ProgressPhase = progress.Phase
	result.ProgressStatus = progress.Status
	result.CurrentChapter = progress.CurrentChapter

	validation := checkpoint.ValidateLatest(s)
	result.OK = validation.OK
	result.ResumedFrom = validation.Step
	result.Phase = validation.Phase
	result.NextExpected = validation.Next
	result.Checked = validation.Checked
	result.Errors = append(result.Errors, validation.Errors...)
	if !validation.OK {
		return result, nil
	}

	cp, exists, err := checkpoint.LoadLatest(s)
	if err != nil {
		return result, err
	}
	if !exists {
		result.OK = false
		result.Errors = append(result.Errors, "latest checkpoint does not exist")
		return result, nil
	}

	run, ok, err := loadRun(s)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("read run.json: %v", err))
		result.OK = false
		return result, nil
	}
	if !ok {
		result.Errors = append(result.Errors, "run.json does not exist")
		result.OK = false
		return result, nil
	}
	run.ResumedFrom = &cp.Step
	run.Events = append(run.Events, contracts.RunEvent{
		At:      now,
		Kind:    "recover",
		Message: fmt.Sprintf("resumed from step %d", cp.Step),
		Fields: map[string]any{
			"phase":         cp.Phase,
			"next_expected": cp.NextExpected,
			"checked":       validation.Checked,
		},
	})
	if _, err := store.WriteJSON(s.RunPath(), run, store.Overwrite); err != nil {
		return result, err
	}

	result.RunUpdated = true
	result.RecoveryPrompt = BuildRecoveryPrompt(cp, progress, validation)
	return result, nil
}

func BuildRecoveryPrompt(cp checkpoint.Checkpoint, progress contracts.Progress, validation checkpoint.Validation) string {
	var b strings.Builder
	b.WriteString("Recovery context for Coordinator:\n")
	fmt.Fprintf(&b, "- resumed_from_step: %d\n", cp.Step)
	fmt.Fprintf(&b, "- checkpoint_phase: %s\n", cp.Phase)
	if cp.NextExpected != "" {
		fmt.Fprintf(&b, "- next_expected: %s\n", cp.NextExpected)
	}
	if progress.Phase != "" {
		fmt.Fprintf(&b, "- progress_phase: %s\n", progress.Phase)
	}
	if progress.Status != "" {
		fmt.Fprintf(&b, "- progress_status: %s\n", progress.Status)
	}
	if progress.CurrentChapter != "" {
		fmt.Fprintf(&b, "- current_chapter: %s\n", progress.CurrentChapter)
	}
	b.WriteString("- checked_artifacts:\n")
	if len(validation.Checked) == 0 {
		b.WriteString("  - none\n")
	} else {
		for _, checked := range validation.Checked {
			kind := artifactKind(cp.Outputs, checked)
			if kind == "" {
				fmt.Fprintf(&b, "  - %s\n", checked)
				continue
			}
			fmt.Fprintf(&b, "  - %s: %s\n", kind, checked)
		}
	}
	b.WriteString("- do_not_repeat:\n")
	fmt.Fprintf(&b, "  - do not rerun completed step %d unless the operator explicitly requests it\n", cp.Step)
	b.WriteString("  - do not overwrite, regenerate, or truncate checked artifacts\n")
	if cp.NextExpected != "" {
		fmt.Fprintf(&b, "  - continue from next_expected=%s using Store artifacts as source of truth\n", cp.NextExpected)
	} else {
		b.WriteString("  - infer the next action from progress.json and checked Store artifacts\n")
	}
	return b.String()
}

func artifactKind(outputs []checkpoint.OutputArtifact, path string) string {
	for _, out := range outputs {
		if out.Path == path {
			return out.Kind
		}
	}
	return ""
}

func loadRun(s store.Store) (contracts.Run, bool, error) {
	var run contracts.Run
	err := store.ReadJSON(s.RunPath(), &run)
	if errors.Is(err, os.ErrNotExist) {
		return contracts.Run{}, false, nil
	}
	if err != nil {
		return contracts.Run{}, false, err
	}
	return run, true, nil
}
