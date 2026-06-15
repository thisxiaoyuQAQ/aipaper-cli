package app

import (
	"strings"
	"testing"
)

func TestRecoverPromptModel_UpdateKey_Continue(t *testing.T) {
	probe := ProbeResult{
		CheckpointStep:  42,
		CheckpointPhase: "review_chapter",
		CheckpointValid: true,
	}
	m := NewRecoverPromptModel(probe)

	// Press 'c' to 继续
	m = m.UpdateKey("c")
	if m.Action() != RecoverActionContinue {
		t.Errorf("expected action=%q, got %q", RecoverActionContinue, m.Action())
	}
}

func TestRecoverPromptModel_UpdateKey_ContinueWithEnter(t *testing.T) {
	probe := ProbeResult{
		CheckpointStep:  42,
		CheckpointPhase: "review_chapter",
		CheckpointValid: true,
	}
	m := NewRecoverPromptModel(probe)

	// Press Enter to 继续
	m = m.UpdateKey("enter")
	if m.Action() != RecoverActionContinue {
		t.Errorf("expected action=%q, got %q", RecoverActionContinue, m.Action())
	}
}

func TestRecoverPromptModel_UpdateKey_重新开始RequiresConfirmation(t *testing.T) {
	probe := ProbeResult{
		CheckpointStep:  42,
		CheckpointPhase: "review_chapter",
		CheckpointValid: true,
	}
	m := NewRecoverPromptModel(probe)

	// Press 'r' to request 重新开始
	m = m.UpdateKey("r")
	if m.Action() != RecoverActionNone {
		t.Errorf("expected action=%q after 重新开始 request, got %q", RecoverActionNone, m.Action())
	}

	// View should show confirmation prompt
	view := m.View()
	if !strings.Contains(view, "重新开始") {
		t.Errorf("expected view to show 重新开始 confirmation prompt")
	}

	// Now in confirmation mode, press 'y' to confirm
	m = m.UpdateKey("y")
	if m.Action() != RecoverActionRestart {
		t.Errorf("expected action=%q after confirmation, got %q", RecoverActionRestart, m.Action())
	}
}

func TestRecoverPromptModel_UpdateKey_重新开始CancelConfirmation(t *testing.T) {
	probe := ProbeResult{
		CheckpointStep:  42,
		CheckpointPhase: "review_chapter",
		CheckpointValid: true,
	}
	m := NewRecoverPromptModel(probe)

	// Press 'r' to request 重新开始
	m = m.UpdateKey("r")

	// View should show confirmation prompt
	view := m.View()
	if !strings.Contains(view, "重新开始") {
		t.Errorf("expected view to show 重新开始 confirmation prompt")
	}

	// Now in confirmation mode, press 'n' to cancel
	m = m.UpdateKey("n")
	if m.Action() != RecoverActionNone {
		t.Errorf("expected action=%q after cancel, got %q", RecoverActionNone, m.Action())
	}

	// View should not show confirmation anymore
	view = m.View()
	if !strings.Contains(view, "继续") {
		t.Errorf("expected view to show normal options after cancel")
	}
}

func TestRecoverPromptModel_UpdateKey_Exit(t *testing.T) {
	probe := ProbeResult{
		CheckpointStep:  42,
		CheckpointPhase: "review_chapter",
		CheckpointValid: true,
	}
	m := NewRecoverPromptModel(probe)

	// Press 'q' to exit
	m = m.UpdateKey("q")
	if m.Action() != RecoverActionExit {
		t.Errorf("expected action=%q, got %q", RecoverActionExit, m.Action())
	}
}

func TestRecoverPromptModel_UpdateKey_ExitWithCtrlC(t *testing.T) {
	probe := ProbeResult{
		CheckpointStep:  42,
		CheckpointPhase: "review_chapter",
		CheckpointValid: true,
	}
	m := NewRecoverPromptModel(probe)

	// Press Ctrl+C to exit
	m = m.UpdateKey("ctrl+c")
	if m.Action() != RecoverActionExit {
		t.Errorf("expected action=%q, got %q", RecoverActionExit, m.Action())
	}
}

func TestRecoverPromptModel_View_ShowsCheckpointInfo(t *testing.T) {
	probe := ProbeResult{
		CheckpointStep:         42,
		CheckpointPhase:        "review_chapter",
		CheckpointNextExpected: "commit_chapter",
		ProgressStatus:         "in_progress",
		CheckpointValid:        true,
	}
	m := NewRecoverPromptModel(probe)

	view := m.View()

	// Should show checkpoint step
	if !strings.Contains(view, "42") {
		t.Errorf("view should contain checkpoint step 42")
	}

	// Should show phase
	if !strings.Contains(view, "review_chapter") {
		t.Errorf("view should contain phase 'review_chapter'")
	}

	// Should show next expected
	if !strings.Contains(view, "commit_chapter") {
		t.Errorf("view should contain next expected 'commit_chapter'")
	}

	// Should show progress status
	if !strings.Contains(view, "in_progress") {
		t.Errorf("view should contain progress status 'in_progress'")
	}

	// Should show action options
	if !strings.Contains(view, "继续") {
		t.Errorf("view should contain '继续' option")
	}
	if !strings.Contains(view, "重新开始") {
		t.Errorf("view should contain '重新开始' option")
	}
}

func TestRecoverPromptModel_View_ShowsConfirmationPrompt(t *testing.T) {
	probe := ProbeResult{
		CheckpointStep:  42,
		CheckpointPhase: "review_chapter",
		CheckpointValid: true,
	}
	m := NewRecoverPromptModel(probe)

	// Enter 重新开始 confirmation mode
	m = m.UpdateKey("r")

	view := m.View()

	// Should show confirmation prompt
	if !strings.Contains(view, "重新开始") {
		t.Errorf("view should contain '重新开始' in confirmation mode")
	}
	if !strings.Contains(view, "保留") && !strings.Contains(view, "已有输出文件") {
		t.Errorf("view should mention that files are kept 已有输出文件")
	}
}

func TestRecoverPromptModel_View_ShowsErrors(t *testing.T) {
	probe := ProbeResult{
		CheckpointStep:   42,
		CheckpointPhase:  "review_chapter",
		CheckpointValid:  true,
		CheckpointErrors: []string{"hash mismatch for drafts/ch03/review.json", "file not found: outline.json"},
	}
	m := NewRecoverPromptModel(probe)

	view := m.View()

	// Should show checkpoint errors
	if !strings.Contains(view, "hash mismatch") {
		t.Errorf("view should contain checkpoint error about hash mismatch")
	}
	if !strings.Contains(view, "file not found") {
		t.Errorf("view should contain checkpoint error about file not found")
	}
}
