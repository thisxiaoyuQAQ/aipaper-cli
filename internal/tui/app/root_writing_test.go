package app

import (
	"testing"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/tui/writing"
)

func TestRootModel_WritingScreen_Integration(t *testing.T) {
	m := NewRootModel(RootOptions{
		WorkDir:       "/tmp/test",
		InitialScreen: ScreenWriting,
	})

	if m.CurrentScreen != ScreenWriting {
		t.Errorf("expected CurrentScreen to be ScreenWriting, got %q", m.CurrentScreen)
	}

	// Writing model should be initialized
	if m.Writing.Done() {
		t.Error("expected Writing model to not be done initially")
	}
}

func TestNewWritingModel(t *testing.T) {
	workDir := "/tmp/test"

	// Test with nil data
	model := newWritingModel(workDir, nil)
	if model.Done() {
		t.Error("expected model to not be done initially")
	}

	// Test with WritingResumeData
	resumeData := WritingResumeData{
		RecoveryPrompt: "Resume from step X",
	}
	model = newWritingModel(workDir, resumeData)
	if model.Done() {
		t.Error("expected model to not be done initially")
	}
}

func TestRootModel_UpdateWriting_Done(t *testing.T) {
	m := NewRootModel(RootOptions{
		WorkDir:       "/tmp/test",
		InitialScreen: ScreenWriting,
	})

	// Simulate writing completion
	m.Writing = writing.NewModel(writing.Options{
		WorkDir: "/tmp/test",
		Width:   120,
		Height:  40,
	})

	// Manually mark as done for test
	doneMsg := writing.RuntimeDoneMsg{Success: true}
	updated, _ := m.Writing.Update(doneMsg)
	m.Writing = updated.(writing.Model)

	// Update should handle done state
	updated2, _ := m.updateWriting("")
	updatedModel := updated2.(RootModel)

	if updatedModel.Writing.Err() != nil {
		t.Errorf("unexpected error: %v", updatedModel.Writing.Err())
	}
}
