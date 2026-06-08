package app

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/tui/configwizard"
)

type Screen string

const (
	ScreenConfigWizard   Screen = "config_wizard"
	ScreenRecoverPrompt  Screen = "recover_prompt"
	ScreenRequirements   Screen = "requirements"
	ScreenMaterialsScan  Screen = "materials_scan"
	ScreenSearchProgress Screen = "search_progress"
	ScreenReferences     Screen = "references"
	ScreenWriting        Screen = "writing_progress"
	ScreenExportSummary  Screen = "export_summary"
	ScreenDone           Screen = "done"
)

type ScreenTransitionMsg struct {
	Next Screen
	Data any
}

type RootOptions struct {
	WorkDir       string
	InitialScreen Screen
}

type RootModel struct {
	WorkDir       string
	CurrentScreen Screen
	ScreenData    any
	ConfigWizard  configwizard.Model

	err error
}

func NewRootModel(opts RootOptions) RootModel {
	screen := opts.InitialScreen
	if screen == "" {
		screen = ScreenConfigWizard
	}
	if !KnownScreen(screen) {
		screen = ScreenConfigWizard
	}
	if opts.WorkDir == "" {
		opts.WorkDir = "."
	}
	m := RootModel{
		WorkDir:       opts.WorkDir,
		CurrentScreen: screen,
		ConfigWizard:  configwizard.NewModel(configwizard.Options{WorkDir: opts.WorkDir}),
	}
	return m
}

func KnownScreen(screen Screen) bool {
	switch screen {
	case ScreenConfigWizard,
		ScreenRecoverPrompt,
		ScreenRequirements,
		ScreenMaterialsScan,
		ScreenSearchProgress,
		ScreenReferences,
		ScreenWriting,
		ScreenExportSummary,
		ScreenDone:
		return true
	default:
		return false
	}
}

func (m RootModel) Init() tea.Cmd {
	return nil
}

func (m RootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case ScreenTransitionMsg:
		if !KnownScreen(msg.Next) {
			m.err = fmt.Errorf("unknown screen %q", msg.Next)
			return m, nil
		}
		m.CurrentScreen = msg.Next
		m.ScreenData = msg.Data
		m.err = nil
		if msg.Next == ScreenConfigWizard {
			m.ConfigWizard = configwizard.NewModel(configwizard.Options{WorkDir: m.WorkDir})
		}
	case tea.KeyMsg:
		if m.CurrentScreen == ScreenConfigWizard {
			m.ConfigWizard = m.ConfigWizard.UpdateKey(msg.String())
			if m.ConfigWizard.Done() {
				m.CurrentScreen = ScreenRequirements
				m.ScreenData = m.ConfigWizard.SavedPath()
				m.err = nil
				return m, nil
			}
			if m.ConfigWizard.Canceled() {
				return m, tea.Quit
			}
			return m, nil
		}
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m RootModel) View() string {
	if m.CurrentScreen == ScreenConfigWizard {
		return m.ConfigWizard.View()
	}
	if m.err != nil {
		return fmt.Sprintf("aipaper-cli\n\n%s\n\nError: %s\n", m.CurrentScreen, m.err)
	}
	return fmt.Sprintf("aipaper-cli\n\n%s\n", m.CurrentScreen)
}

func (m RootModel) Err() error {
	return m.err
}
