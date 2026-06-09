package app

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestRootModel_ExitConfirmation_RequirementsScreen(t *testing.T) {
	m := NewRootModel(RootOptions{
		WorkDir:       ".",
		InitialScreen: ScreenRequirements,
	})

	// Simulate user pressing Ctrl+C on Requirements screen
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	m = updated.(RootModel)

	// Should enter exit confirmation mode if Requirements has changes
	// For now, we'll test the mechanism
	if m.exitConfirm && !m.shouldConfirmExit() {
		t.Errorf("exitConfirm set but shouldConfirmExit returned false")
	}

	// If exit confirmation is shown, cmd should be nil (not quit yet)
	if m.exitConfirm && cmd != nil {
		t.Errorf("expected cmd=nil when showing exit confirmation")
	}
}

func TestRootModel_ExitConfirmation_ConfirmYes(t *testing.T) {
	m := NewRootModel(RootOptions{
		WorkDir:       ".",
		InitialScreen: ScreenRequirements,
	})
	m.exitConfirm = true
	m.exitConfirmScreen = ScreenRequirements

	// User presses 'y' to confirm exit
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m = updated.(RootModel)

	// Should quit
	if cmd == nil {
		t.Errorf("expected quit command after confirming exit")
	}
}

func TestRootModel_ExitConfirmation_ConfirmNo(t *testing.T) {
	m := NewRootModel(RootOptions{
		WorkDir:       ".",
		InitialScreen: ScreenRequirements,
	})
	m.exitConfirm = true
	m.exitConfirmScreen = ScreenRequirements

	// User presses 'n' to cancel exit
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	m = updated.(RootModel)

	// Should not quit, should clear exitConfirm
	if m.exitConfirm {
		t.Errorf("expected exitConfirm=false after pressing 'n'")
	}
	if cmd != nil {
		t.Errorf("expected cmd=nil after canceling exit")
	}
}

func TestRootModel_ExitConfirmation_EscapeKey(t *testing.T) {
	m := NewRootModel(RootOptions{
		WorkDir:       ".",
		InitialScreen: ScreenRequirements,
	})
	m.exitConfirm = true
	m.exitConfirmScreen = ScreenRequirements

	// User presses 'esc' to cancel exit
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(RootModel)

	// Should clear exitConfirm
	if m.exitConfirm {
		t.Errorf("expected exitConfirm=false after pressing 'esc'")
	}
}

func TestRootModel_View_ShowsExitConfirmation(t *testing.T) {
	m := NewRootModel(RootOptions{
		WorkDir:       ".",
		InitialScreen: ScreenRequirements,
	})
	m.exitConfirm = true
	m.exitConfirmScreen = ScreenRequirements

	view := m.View()

	// Should show exit confirmation message
	if !strings.Contains(view, "Exit") {
		t.Errorf("view should contain 'Exit' when exitConfirm=true")
	}
	if !strings.Contains(view, "[Y]") && !strings.Contains(view, "Yes") {
		t.Errorf("view should contain 'Yes' option")
	}
	if !strings.Contains(view, "[N]") && !strings.Contains(view, "No") {
		t.Errorf("view should contain 'No' option")
	}
}

func TestRootModel_ScreenTransition_ClearsExitConfirm(t *testing.T) {
	m := NewRootModel(RootOptions{
		WorkDir:       ".",
		InitialScreen: ScreenRequirements,
	})
	m.exitConfirm = true
	m.exitConfirmScreen = ScreenRequirements

	// Transition to another screen (ConfigWizard doesn't require specific data)
	updated, _ := m.Update(ScreenTransitionMsg{
		Next: ScreenConfigWizard,
		Data: nil,
	})
	m = updated.(RootModel)

	// Should clear exitConfirm
	if m.exitConfirm {
		t.Errorf("expected exitConfirm=false after screen transition")
	}
}

func TestRootModel_WritingScreen_NoExitConfirm(t *testing.T) {
	m := NewRootModel(RootOptions{
		WorkDir:       ".",
		InitialScreen: ScreenWriting,
	})

	// Writing screen should not require exit confirmation (it handles Ctrl+C itself)
	if m.shouldConfirmExit() {
		t.Errorf("expected shouldConfirmExit()=false for WritingScreen")
	}
}

func TestRootModel_RecoverPromptScreen_NoExitConfirm(t *testing.T) {
	m := NewRootModel(RootOptions{
		WorkDir:       ".",
		InitialScreen: ScreenRecoverPrompt,
	})

	// RecoverPrompt should not require exit confirmation
	if m.shouldConfirmExit() {
		t.Errorf("expected shouldConfirmExit()=false for RecoverPromptScreen")
	}
}

func TestRootModel_ConfigWizardScreen_NoExitConfirm(t *testing.T) {
	m := NewRootModel(RootOptions{
		WorkDir:       ".",
		InitialScreen: ScreenConfigWizard,
	})

	// ConfigWizard should not require exit confirmation (handles its own)
	if m.shouldConfirmExit() {
		t.Errorf("expected shouldConfirmExit()=false for ConfigWizardScreen")
	}
}

func TestRootModel_shouldConfirmExit_ReturnsCorrectly(t *testing.T) {
	tests := []struct {
		screen   Screen
		expected bool
		desc     string
	}{
		{ScreenRequirements, false, "Requirements with no changes"},
		{ScreenReferences, false, "References with no changes"},
		{ScreenWriting, false, "Writing handles its own Ctrl+C"},
		{ScreenRecoverPrompt, false, "RecoverPrompt handles its own exit"},
		{ScreenConfigWizard, false, "ConfigWizard handles its own exit"},
		{ScreenMaterialsScan, false, "MaterialsScan allows quick exit"},
		{ScreenSearchProgress, false, "SearchProgress allows quick exit"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			m := NewRootModel(RootOptions{
				WorkDir:       ".",
				InitialScreen: tt.screen,
			})

			result := m.shouldConfirmExit()
			if result != tt.expected {
				t.Errorf("shouldConfirmExit() for %s: expected %v, got %v", tt.screen, tt.expected, result)
			}
		})
	}
}

func TestRootModel_currentScreenView_ReturnsCorrectView(t *testing.T) {
	screens := []Screen{
		ScreenConfigWizard,
		ScreenRecoverPrompt,
		ScreenRequirements,
		ScreenMaterialsScan,
		ScreenSearchProgress,
		ScreenReferences,
		ScreenWriting,
	}

	for _, screen := range screens {
		t.Run(string(screen), func(t *testing.T) {
			m := NewRootModel(RootOptions{
				WorkDir:       ".",
				InitialScreen: screen,
			})

			view := m.currentScreenView()
			if view == "" {
				t.Errorf("currentScreenView() returned empty string for %s", screen)
			}
		})
	}
}
