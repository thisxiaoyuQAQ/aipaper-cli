package writing

import (
	"testing"
	"time"
)

func TestModel_StopRequest_SetsStoppingState(t *testing.T) {
	m := NewModel(Options{Width: 120, Height: 40})

	// Initially running
	if !m.running {
		t.Errorf("expected model to start in running state")
	}
	if m.stopping {
		t.Errorf("expected stopping=false initially")
	}
	if m.stopRequested {
		t.Errorf("expected stopRequested=false initially")
	}

	// Press Ctrl+C to request stop
	updated, _ := m.handleKey("ctrl+c")
	m = updated.(Model)

	if !m.stopping {
		t.Errorf("expected stopping=true after ctrl+c")
	}
	if !m.stopRequested {
		t.Errorf("expected stopRequested=true after ctrl+c")
	}
	if !m.running {
		t.Errorf("expected running=true while stopping")
	}
	if m.canceled {
		t.Errorf("expected canceled=false until checkpoint saved")
	}
}

func TestModel_StopRequest_IgnoresSecondCtrlC(t *testing.T) {
	m := NewModel(Options{Width: 120, Height: 40})

	// First Ctrl+C
	updated, _ := m.handleKey("ctrl+c")
	m = updated.(Model)
	if !m.stopping {
		t.Errorf("expected stopping=true after first ctrl+c")
	}

	// Second Ctrl+C should be ignored (not quit immediately)
	updated, cmd := m.handleKey("ctrl+c")
	m = updated.(Model)
	if cmd != nil {
		t.Errorf("expected nil cmd for second ctrl+c while stopping")
	}
	if !m.stopping {
		t.Errorf("expected stopping=true to persist")
	}
}

func TestModel_CheckpointSaved_CompletesStop(t *testing.T) {
	m := NewModel(Options{Width: 120, Height: 40})

	// Request stop
	updated, _ := m.handleKey("ctrl+c")
	m = updated.(Model)

	// Simulate checkpoint saved event
	ev := RuntimeEvent{
		At:   time.Now(),
		Kind: EventCheckpointSaved,
	}
	m.handleRuntimeEvent(ev)

	if m.running {
		t.Errorf("expected running=false after checkpoint saved during stop")
	}
	if !m.canceled {
		t.Errorf("expected canceled=true after checkpoint saved during stop")
	}
}

func TestModel_CheckpointSaved_WithoutStopRequest(t *testing.T) {
	m := NewModel(Options{Width: 120, Height: 40})

	// Simulate checkpoint saved event without stop request
	ev := RuntimeEvent{
		At:   time.Now(),
		Kind: EventCheckpointSaved,
	}
	m.handleRuntimeEvent(ev)

	// Should remain running
	if !m.running {
		t.Errorf("expected running=true when checkpoint saved without stop request")
	}
	if m.canceled {
		t.Errorf("expected canceled=false when checkpoint saved without stop request")
	}
}

func TestModel_Canceled_ReturnsTrue_AfterStopComplete(t *testing.T) {
	m := NewModel(Options{Width: 120, Height: 40})

	// Request stop
	updated, _ := m.handleKey("ctrl+c")
	m = updated.(Model)

	if m.Canceled() {
		t.Errorf("expected Canceled()=false before checkpoint saved")
	}

	// Simulate checkpoint saved
	ev := RuntimeEvent{
		At:   time.Now(),
		Kind: EventCheckpointSaved,
	}
	m.handleRuntimeEvent(ev)

	if !m.Canceled() {
		t.Errorf("expected Canceled()=true after stop complete")
	}
}

func TestModel_StopRequest_AddsLogEntry(t *testing.T) {
	m := NewModel(Options{Width: 120, Height: 40})

	// Request stop
	updated, _ := m.handleKey("ctrl+c")
	m = updated.(Model)

	// Should have log entry about stop request
	found := false
	for _, entry := range m.logs {
		if entry.role == "system" && contains(entry.message, "Stop requested") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected log entry about stop request")
	}
}

func TestModel_CheckpointSavedDuringStop_AddsSuccessLog(t *testing.T) {
	m := NewModel(Options{Width: 120, Height: 40})

	// Request stop
	updated, _ := m.handleKey("ctrl+c")
	m = updated.(Model)

	// Simulate checkpoint saved
	ev := RuntimeEvent{
		At:   time.Now(),
		Kind: EventCheckpointSaved,
	}
	m.handleRuntimeEvent(ev)

	// Should have log entry about progress saved
	found := false
	for _, entry := range m.logs {
		if entry.role == "system" && contains(entry.message, "Progress saved") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected log entry about progress saved")
	}
}

func TestModel_CtrlC_QuitsWhenNotRunning(t *testing.T) {
	m := NewModel(Options{Width: 120, Height: 40})
	m.running = false

	// Ctrl+C when not running should quit
	_, cmd := m.handleKey("ctrl+c")
	if cmd == nil {
		t.Errorf("expected quit command when not running")
	}
}

func TestModel_Stopping_ReturnsTrue_DuringStop(t *testing.T) {
	m := NewModel(Options{Width: 120, Height: 40})

	if m.Stopping() {
		t.Errorf("expected Stopping()=false initially")
	}

	// Request stop
	updated, _ := m.handleKey("ctrl+c")
	m = updated.(Model)

	if !m.Stopping() {
		t.Errorf("expected Stopping()=true after stop request")
	}
}

func TestModel_StopRequested_ReturnsTrue_AfterCtrlC(t *testing.T) {
	m := NewModel(Options{Width: 120, Height: 40})

	if m.StopRequested() {
		t.Errorf("expected StopRequested()=false initially")
	}

	// Request stop
	updated, _ := m.handleKey("ctrl+c")
	m = updated.(Model)

	if !m.StopRequested() {
		t.Errorf("expected StopRequested()=true after ctrl+c")
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
