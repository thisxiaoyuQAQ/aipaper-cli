package writing

import (
	"testing"
	"time"
)

func TestModel_EscRequest_SetsPauseRequestedState(t *testing.T) {
	m := NewModel(Options{Width: 120, Height: 40})

	updated, cmd := m.handleKey("esc")
	m = updated.(Model)

	if !m.pauseRequested {
		t.Errorf("expected pauseRequested=true after esc")
	}
	if m.paused {
		t.Errorf("expected paused=false until runtime reaches safe boundary")
	}
	if !m.running {
		t.Errorf("expected running=true while waiting for safe boundary")
	}
	if cmd == nil {
		t.Fatalf("expected pause command")
	}
	if _, ok := cmd().(RuntimePauseRequestedMsg); !ok {
		t.Fatalf("expected RuntimePauseRequestedMsg")
	}
}

func TestModel_EscRequest_IgnoresSecondEsc(t *testing.T) {
	m := NewModel(Options{Width: 120, Height: 40})

	updated, _ := m.handleKey("esc")
	m = updated.(Model)
	updated, cmd := m.handleKey("esc")
	m = updated.(Model)

	if cmd != nil {
		t.Errorf("expected nil cmd for second esc while pause requested")
	}
	if !m.pauseRequested {
		t.Errorf("expected pauseRequested=true to persist")
	}
}

func TestModel_PausedEvent_CompletesPause(t *testing.T) {
	m := NewModel(Options{Width: 120, Height: 40})
	updated, _ := m.handleKey("esc")
	m = updated.(Model)

	m.handleRuntimeEvent(RuntimeEvent{At: time.Now(), Kind: EventPaused})

	if !m.paused {
		t.Errorf("expected paused=true after paused event")
	}
	if m.pauseRequested {
		t.Errorf("expected pauseRequested=false after paused event")
	}
	if !m.running {
		t.Errorf("expected runtime to remain alive while paused")
	}
}

func TestModel_CheckpointSaved_WithoutStopRequest(t *testing.T) {
	m := NewModel(Options{Width: 120, Height: 40})

	m.handleRuntimeEvent(RuntimeEvent{At: time.Now(), Kind: EventCheckpointSaved})

	if !m.running {
		t.Errorf("expected running=true when checkpoint saved without stop request")
	}
	if m.canceled {
		t.Errorf("expected canceled=false when checkpoint saved without stop request")
	}
}

func TestModel_EnterResumesWhenPaused(t *testing.T) {
	m := NewModel(Options{Width: 120, Height: 40})
	m.handleRuntimeEvent(RuntimeEvent{At: time.Now(), Kind: EventPaused})

	updated, cmd := m.handleKey("enter")
	m = updated.(Model)

	if cmd == nil {
		t.Fatalf("expected resume command")
	}
	if _, ok := cmd().(RuntimeResumeRequestedMsg); !ok {
		t.Fatalf("expected RuntimeResumeRequestedMsg")
	}
}

func TestModel_InstructionSubmit_AddsPendingAndCommand(t *testing.T) {
	m := NewModel(Options{Width: 120, Height: 40})
	m.input = "请更强调实验设计"

	updated, cmd := m.handleKey("enter")
	m = updated.(Model)

	if cmd == nil {
		t.Fatalf("expected instruction command")
	}
	msg, ok := cmd().(RuntimeInstructionSubmittedMsg)
	if !ok {
		t.Fatalf("expected RuntimeInstructionSubmittedMsg")
	}
	if msg.Text != "请更强调实验设计" {
		t.Fatalf("instruction text = %q", msg.Text)
	}
	if m.input != "" {
		t.Fatalf("expected input to be cleared")
	}
	if m.pendingInstructions != 1 {
		t.Fatalf("pendingInstructions = %d, want 1", m.pendingInstructions)
	}
}

func TestModel_CtrlC_DoesNotPause(t *testing.T) {
	m := NewModel(Options{Width: 120, Height: 40})
	updated, cmd := m.handleKey("ctrl+c")
	m = updated.(Model)

	if cmd != nil {
		t.Fatalf("expected writing model not to handle ctrl+c")
	}
	if m.StopRequested() || m.Stopping() || m.PauseRequested() {
		t.Fatalf("ctrl+c should be owned by RootModel exit handling")
	}
}

func TestModel_PauseAccessors(t *testing.T) {
	m := NewModel(Options{Width: 120, Height: 40})
	if m.PauseRequested() || m.Paused() {
		t.Fatalf("expected initial pause state to be false")
	}
	updated, _ := m.handleKey("esc")
	m = updated.(Model)
	if !m.PauseRequested() {
		t.Fatalf("expected PauseRequested()=true after esc")
	}
	m.handleRuntimeEvent(RuntimeEvent{At: time.Now(), Kind: EventPaused})
	if !m.Paused() {
		t.Fatalf("expected Paused()=true after paused event")
	}
}
