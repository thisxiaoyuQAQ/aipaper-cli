package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/config"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/tui/configwizard"
)

func TestRootModelTransitionsScreen(t *testing.T) {
	model := NewRootModel(RootOptions{})

	updated, cmd := model.Update(ScreenTransitionMsg{
		Next: ScreenRequirements,
		Data: "payload",
	})
	if cmd != nil {
		t.Fatalf("cmd = %v, want nil", cmd)
	}
	root, ok := updated.(RootModel)
	if !ok {
		t.Fatalf("updated model type = %T, want RootModel", updated)
	}
	if root.CurrentScreen != ScreenRequirements {
		t.Fatalf("CurrentScreen = %q, want %q", root.CurrentScreen, ScreenRequirements)
	}
	if root.ScreenData != "payload" {
		t.Fatalf("ScreenData = %#v, want payload", root.ScreenData)
	}
	if root.Err() != nil {
		t.Fatalf("Err() = %v, want nil", root.Err())
	}
}

func TestRootModelRejectsUnknownScreen(t *testing.T) {
	model := NewRootModel(RootOptions{InitialScreen: ScreenRequirements})

	updated, _ := model.Update(ScreenTransitionMsg{Next: Screen("unknown")})
	root := updated.(RootModel)
	if root.CurrentScreen != ScreenRequirements {
		t.Fatalf("CurrentScreen = %q, want %q", root.CurrentScreen, ScreenRequirements)
	}
	if root.Err() == nil {
		t.Fatalf("Err() = nil, want unknown screen error")
	}
}

func TestRootModelQuitKeyReturnsQuitCommand(t *testing.T) {
	model := NewRootModel(RootOptions{})

	_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatalf("cmd = nil, want tea.Quit")
	}
	if msg := cmd(); msg != (tea.QuitMsg{}) {
		t.Fatalf("cmd() = %#v, want tea.QuitMsg", msg)
	}
}

func TestRootModelConfigWizardSaveTransitionsToRequirements(t *testing.T) {
	model := NewRootModel(RootOptions{WorkDir: "work"})
	model.ConfigWizard = configwizard.NewModel(configwizard.Options{
		WorkDir: "work",
		Save: func(workDir string, cfg config.Config) (string, error) {
			if workDir != "work" {
				t.Fatalf("workDir = %q, want work", workDir)
			}
			if cfg.Provider != "default" || cfg.Model == "" {
				t.Fatalf("saved config = %#v", cfg)
			}
			return "work/aipaper.json", nil
		},
	})

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updated, _ = updated.(RootModel).Update(tea.KeyMsg{Type: tea.KeyEnter})
	updated, cmd := updated.(RootModel).Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Fatalf("cmd = %v, want nil", cmd)
	}
	root := updated.(RootModel)
	if root.CurrentScreen != ScreenRequirements {
		t.Fatalf("CurrentScreen = %q, want %q", root.CurrentScreen, ScreenRequirements)
	}
	if root.ScreenData != "work/aipaper.json" {
		t.Fatalf("ScreenData = %#v, want saved path", root.ScreenData)
	}
}
