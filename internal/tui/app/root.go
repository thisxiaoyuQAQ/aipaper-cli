package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	runtimeapp "github.com/thisxiaoyuQAQ/aipaper-cli/internal/app"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/config"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/export"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/i18n"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/references"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/search"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/store"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/tui/configwizard"
	donetui "github.com/thisxiaoyuQAQ/aipaper-cli/internal/tui/done"
	exportsummarytui "github.com/thisxiaoyuQAQ/aipaper-cli/internal/tui/exportsummary"
	materialstui "github.com/thisxiaoyuQAQ/aipaper-cli/internal/tui/materials"
	referencestui "github.com/thisxiaoyuQAQ/aipaper-cli/internal/tui/references"
	requirementstui "github.com/thisxiaoyuQAQ/aipaper-cli/internal/tui/requirements"
	searchtui "github.com/thisxiaoyuQAQ/aipaper-cli/internal/tui/search"
	writingtui "github.com/thisxiaoyuQAQ/aipaper-cli/internal/tui/writing"
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
	StateProbe    StateProbeFunc
	Recover       func(string) (runtimeapp.RecoveryResult, error)
	I18N          i18n.T
	// RuntimeStarter launches the real writing runtime when the writing
	// screen starts; nil uses StartWritingRuntime (module-22 bugfix wiring).
	RuntimeStarter WritingRuntimeStarter
}

type RootModel struct {
	WorkDir       string
	CurrentScreen Screen
	ScreenData    any
	ConfigWizard  configwizard.Model
	RecoverPrompt RecoverPromptModel
	Requirements  requirementstui.Model
	Materials     materialstui.Model
	Search        searchtui.Model
	References    referencestui.Model
	Writing       writingtui.Model
	ExportSummary exportsummarytui.Model
	Done          donetui.Model
	Probe         ProbeResult
	i18n          i18n.T
	recover       func(string) (runtimeapp.RecoveryResult, error)

	// Real writing runtime (module-22 bugfix wiring)
	runtimeStarter WritingRuntimeStarter
	runtime        *WritingRuntime

	// Runtime commands issued before the runtime starter returns.
	pendingRuntimePause        bool
	pendingRuntimeInstructions []string

	// Exit confirmation state
	exitConfirm       bool
	exitConfirmScreen Screen

	// Terminal dimensions from the last tea.WindowSizeMsg, forwarded to the
	// active screen so list-style views can wrap and scroll to fit.
	width  int
	height int

	err error
}

func NewRootModel(opts RootOptions) RootModel {
	if opts.WorkDir == "" {
		opts.WorkDir = "."
	}
	recoverFn := opts.Recover
	if recoverFn == nil {
		recoverFn = runtimeapp.Recover
	}
	tr := opts.I18N
	if tr.IsZero() {
		cfg, _, err := config.Load(config.LoadOptions{WorkDir: opts.WorkDir})
		if err == nil {
			tr = i18n.New(cfg.UILanguage)
		} else {
			tr = i18n.New("")
		}
	}
	m := RootModel{
		WorkDir:        opts.WorkDir,
		ConfigWizard:   configwizard.NewModel(configwizard.Options{WorkDir: opts.WorkDir, I18N: tr}),
		i18n:           tr,
		recover:        recoverFn,
		runtimeStarter: opts.RuntimeStarter,
	}

	screen := opts.InitialScreen
	if screen == "" {
		probeFn := opts.StateProbe
		if probeFn == nil {
			probeFn = StateProbe
		}
		probe, err := probeFn(opts.WorkDir)
		m.Probe = probe
		m.ScreenData = probe
		if err != nil {
			m.err = fmt.Errorf("state probe failed: %w", err)
			screen = ScreenConfigWizard
		} else {
			screen = probe.SuggestedScreen
		}
	}
	if screen == "" || !KnownScreen(screen) {
		screen = ScreenConfigWizard
	}
	m.CurrentScreen = screen
	if screen == ScreenRecoverPrompt {
		m.RecoverPrompt = NewRecoverPromptModel(m.Probe, m.i18n)
	}
	if screen == ScreenRequirements {
		m.Requirements = newRequirementsModel(m.i18n)
	}
	if screen == ScreenMaterialsScan {
		materialsModel, err := newMaterialsModel(m.WorkDir, m.ScreenData, m.i18n)
		if err != nil {
			m.err = err
			m.CurrentScreen = ScreenRequirements
			m.Requirements = newRequirementsModel(m.i18n)
		} else {
			m.Materials = materialsModel
		}
	}
	if screen == ScreenSearchProgress {
		searchModel, err := newSearchModel(m.WorkDir, m.ScreenData, m.i18n)
		if err != nil {
			m.err = err
		} else {
			m.Search = searchModel
		}
	}
	if screen == ScreenReferences {
		referencesModel, err := newReferencesModel(m.WorkDir, m.ScreenData, m.i18n)
		if err != nil {
			m.err = err
		} else {
			m.References = referencesModel
		}
	}
	if screen == ScreenWriting {
		writingModel := newWritingModel(m.WorkDir, m.ScreenData, m.i18n)
		m.Writing = writingModel
	}
	if screen == ScreenExportSummary {
		m.ExportSummary = newExportSummaryModel(m.WorkDir, m.i18n)
	}
	if screen == ScreenDone {
		m.Done = newDoneModel(m.WorkDir, m.ScreenData, m.i18n)
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
	if m.CurrentScreen == ScreenMaterialsScan {
		if m.err != nil {
			return nil
		}
		return m.Materials.Init()
	}
	if m.CurrentScreen == ScreenSearchProgress {
		if m.err != nil {
			return nil
		}
		return m.Search.Init()
	}
	if m.CurrentScreen == ScreenWriting {
		if m.err != nil {
			return nil
		}
		return tea.Batch(m.Writing.Init(), m.startWritingRuntimeCmd())
	}
	if m.CurrentScreen == ScreenExportSummary {
		return m.ExportSummary.Init()
	}
	return nil
}

func (m RootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m = m.applySizeToActiveScreen()
		return m, nil
	case ScreenTransitionMsg:
		if !KnownScreen(msg.Next) {
			m.err = fmt.Errorf("unknown screen %q", msg.Next)
			return m, nil
		}
		var materialsModel materialstui.Model
		var searchModel searchtui.Model
		if msg.Next == ScreenMaterialsScan {
			var err error
			materialsModel, err = newMaterialsModel(m.WorkDir, msg.Data, m.i18n)
			if err != nil {
				m.err = err
				return m, nil
			}
		}
		if msg.Next == ScreenSearchProgress {
			var err error
			searchModel, err = newSearchModel(m.WorkDir, msg.Data, m.i18n)
			if err != nil {
				m.err = err
				return m, nil
			}
		}
		var referencesModel referencestui.Model
		if msg.Next == ScreenReferences {
			var err error
			referencesModel, err = newReferencesModel(m.WorkDir, msg.Data, m.i18n)
			if err != nil {
				m.err = err
				return m, nil
			}
		}
		var writingModel writingtui.Model
		if msg.Next == ScreenWriting {
			writingModel = newWritingModel(m.WorkDir, msg.Data, m.i18n)
		}
		m.CurrentScreen = msg.Next
		m.ScreenData = msg.Data
		m.err = nil
		m.exitConfirm = false
		if msg.Next == ScreenConfigWizard {
			m.ConfigWizard = configwizard.NewModel(configwizard.Options{WorkDir: m.WorkDir, I18N: m.i18n})
		}
		if msg.Next == ScreenRecoverPrompt {
			if probe, ok := msg.Data.(ProbeResult); ok {
				m.Probe = probe
			}
			m.RecoverPrompt = NewRecoverPromptModel(m.Probe, m.i18n)
		}
		if msg.Next == ScreenRequirements {
			m.Requirements = newRequirementsModel(m.i18n)
		}
		if msg.Next == ScreenMaterialsScan {
			m.Materials = materialsModel
			return m.applySizeToActiveScreen(), m.Materials.Init()
		}
		if msg.Next == ScreenSearchProgress {
			m.Search = searchModel
			return m.applySizeToActiveScreen(), m.Search.Init()
		}
		if msg.Next == ScreenReferences {
			m.References = referencesModel
			return m.applySizeToActiveScreen(), nil
		}
		if msg.Next == ScreenWriting {
			m.Writing = writingModel
			return m.applySizeToActiveScreen(), tea.Batch(m.Writing.Init(), m.startWritingRuntimeCmd())
		}
		if msg.Next == ScreenExportSummary {
			m.ExportSummary = newExportSummaryModel(m.WorkDir, m.i18n)
			return m.applySizeToActiveScreen(), m.ExportSummary.Init()
		}
		if msg.Next == ScreenDone {
			m.Done = newDoneModel(m.WorkDir, msg.Data, m.i18n)
			return m.applySizeToActiveScreen(), nil
		}
	case materialstui.ScanFinishedMsg:
		if m.CurrentScreen == ScreenMaterialsScan {
			m.Materials, _ = m.Materials.Update(msg)
			m.err = m.Materials.Err()
			return m, nil
		}
	case searchtui.SearchFinishedMsg:
		if m.CurrentScreen == ScreenSearchProgress {
			m.Search, _ = m.Search.Update(msg)
			m.err = m.Search.Err()
			return m, nil
		}
	case writingRuntimeStartedMsg:
		// Module-22 bugfix wiring: the real runtime started (or failed to).
		if msg.err != nil {
			m.pendingRuntimePause = false
			m.pendingRuntimeInstructions = nil
			updated, _ := m.Writing.Update(writingtui.RuntimeDoneMsg{Error: msg.err})
			if writingModel, ok := updated.(writingtui.Model); ok {
				m.Writing = writingModel
			}
			m.err = msg.err
			return m, nil
		}
		m.runtime = msg.runtime
		if m.pendingRuntimePause {
			m.runtime.RequestPause()
			m.pendingRuntimePause = false
		}
		for _, text := range m.pendingRuntimeInstructions {
			m.runtime.SubmitInstruction(text)
		}
		m.pendingRuntimeInstructions = nil
		return m, m.runtime.NextEventCmd()
	case writingtui.RuntimeEventMsg:
		if m.CurrentScreen == ScreenWriting {
			updated, _ := m.Writing.Update(msg)
			if writingModel, ok := updated.(writingtui.Model); ok {
				m.Writing = writingModel
			}
			if m.Writing.Canceled() {
				return m, tea.Quit
			}
		}
		if m.runtime == nil {
			return m, nil
		}
		return m, m.runtime.NextEventCmd()
	case writingtui.RuntimeDoneMsg:
		if m.CurrentScreen == ScreenWriting {
			updated, _ := m.Writing.Update(msg)
			if writingModel, ok := updated.(writingtui.Model); ok {
				m.Writing = writingModel
			}
			if m.Writing.Done() && m.Writing.Err() == nil {
				m.CurrentScreen = ScreenExportSummary
				m.ScreenData = nil
				m.ExportSummary = newExportSummaryModel(m.WorkDir, m.i18n)
				m.err = nil
				return m, m.ExportSummary.Init()
			}
			m.err = m.Writing.Err()
		}
		return m, nil
	case writingtui.RuntimeStopRequestedMsg:
		// Backward compatibility for older tests/messages: abort the real runtime.
		if m.CurrentScreen == ScreenWriting {
			if m.runtime != nil {
				m.runtime.Stop()
			}
			return m, nil
		}
	case writingtui.RuntimePauseRequestedMsg:
		if m.CurrentScreen == ScreenWriting {
			if m.runtime != nil {
				m.runtime.RequestPause()
			} else {
				m.pendingRuntimePause = true
			}
		}
		return m, nil
	case writingtui.RuntimeResumeRequestedMsg:
		if m.CurrentScreen == ScreenWriting {
			if m.runtime != nil {
				m.runtime.Resume()
			} else {
				m.pendingRuntimePause = false
			}
		}
		return m, nil
	case writingtui.RuntimeInstructionSubmittedMsg:
		if m.CurrentScreen == ScreenWriting {
			if m.runtime != nil {
				m.runtime.SubmitInstruction(msg.Text)
			} else {
				m.pendingRuntimeInstructions = append(m.pendingRuntimeInstructions, msg.Text)
			}
		}
		return m, nil
	case tea.KeyMsg:
		// Handle exit confirmation first
		if m.exitConfirm {
			key := strings.ToLower(strings.TrimSpace(msg.String()))
			switch key {
			case "y":
				if m.exitConfirmScreen == ScreenWriting && m.runtime != nil {
					m.runtime.Stop()
					m.exitConfirm = false
					return m, m.runtime.NextEventCmd()
				}
				return m, tea.Quit
			case "n", "esc":
				m.exitConfirm = false
				return m, nil
			}
			return m, nil
		}

		if m.CurrentScreen == ScreenConfigWizard {
			m.ConfigWizard = m.ConfigWizard.UpdateKey(msg.String())
			if m.ConfigWizard.Done() {
				if savedCfg, _, err := config.Load(config.LoadOptions{WorkDir: m.WorkDir}); err == nil {
					m.i18n = i18n.New(savedCfg.UILanguage)
				}
				m.CurrentScreen = ScreenRequirements
				m.ScreenData = m.ConfigWizard.SavedPath()
				m.Requirements = newRequirementsModel(m.i18n)
				m.err = nil
				return m, nil
			}
			if m.ConfigWizard.Canceled() {
				return m, tea.Quit
			}
			return m, nil
		}
		if m.CurrentScreen == ScreenRecoverPrompt {
			m.RecoverPrompt = m.RecoverPrompt.UpdateKey(msg.String())
			switch m.RecoverPrompt.Action() {
			case RecoverActionContinue:
				result, err := m.recover(m.WorkDir)
				if err != nil {
					m.err = fmt.Errorf("recover failed: %w", err)
					return m, nil
				}
				if !result.OK {
					m.err = fmt.Errorf("recover failed: %s", strings.Join(result.Errors, "; "))
					return m, nil
				}
				m.CurrentScreen = ScreenWriting
				m.ScreenData = WritingResumeData{Recovery: result, RecoveryPrompt: result.RecoveryPrompt}
				m.Writing = newWritingModel(m.WorkDir, m.ScreenData, m.i18n)
				m.err = nil
				return m, tea.Batch(m.Writing.Init(), m.startWritingRuntimeCmd())
			case RecoverActionRestart:
				m.CurrentScreen = ScreenRequirements
				m.ScreenData = RecoverActionRestart
				m.Requirements = newRequirementsModel(m.i18n)
				m.err = nil
				return m, nil
			case RecoverActionExit:
				return m, tea.Quit
			}
			return m, nil
		}
		if m.CurrentScreen == ScreenRequirements {
			return m.updateRequirements(msg.String())
		}
		if m.CurrentScreen == ScreenMaterialsScan {
			return m.updateMaterials(msg.String())
		}
		if m.CurrentScreen == ScreenSearchProgress {
			return m.updateSearch(msg.String())
		}
		if m.CurrentScreen == ScreenReferences {
			return m.updateReferences(msg.String())
		}
		if m.CurrentScreen == ScreenWriting {
			if msg.String() == "ctrl+c" || msg.String() == "q" {
				m.exitConfirm = true
				m.exitConfirmScreen = m.CurrentScreen
				return m, nil
			}
			return m.updateWriting(msg)
		}
		if m.CurrentScreen == ScreenExportSummary {
			return m.updateExportSummary(msg.String())
		}
		if m.CurrentScreen == ScreenDone {
			return m.updateDone(msg.String())
		}
		// Global Ctrl+C handling with confirmation for certain screens
		switch msg.String() {
		case "ctrl+c":
			// Screens that need exit confirmation
			if m.shouldConfirmExit() {
				m.exitConfirm = true
				m.exitConfirmScreen = m.CurrentScreen
				return m, nil
			}
			return m, tea.Quit
		case "q":
			if m.shouldConfirmExit() {
				m.exitConfirm = true
				m.exitConfirmScreen = m.CurrentScreen
				return m, nil
			}
			return m, tea.Quit
		}
	case tea.MouseMsg:
		if m.CurrentScreen == ScreenWriting {
			return m.updateWriting(msg)
		}
	default:
		if m.CurrentScreen == ScreenExportSummary {
			updated, cmd := m.ExportSummary.Update(msg)
			if exportModel, ok := updated.(exportsummarytui.Model); ok {
				m.ExportSummary = exportModel
				m.err = m.ExportSummary.Err()
			}
			return m, cmd
		}
	}
	return m, nil
}

func (m RootModel) View() string {
	// Show exit confirmation if active
	if m.exitConfirm {
		view := m.currentScreenView()
		return view + "\n\n" + m.i18n.Text(i18n.RootExitConfirm) + "\n"
	}

	if m.CurrentScreen == ScreenConfigWizard {
		return m.ConfigWizard.View()
	}
	if m.CurrentScreen == ScreenRecoverPrompt {
		view := m.RecoverPrompt.View()
		if m.err != nil {
			return view + "\n" + m.i18n.Text(i18n.CommonErrorPrefix) + ": " + m.err.Error() + "\n"
		}
		return view
	}
	if m.CurrentScreen == ScreenRequirements {
		view := m.Requirements.View()
		if m.err != nil {
			return view + "\n" + m.i18n.Text(i18n.CommonErrorPrefix) + ": " + m.err.Error() + "\n"
		}
		return view
	}
	if m.CurrentScreen == ScreenMaterialsScan {
		view := m.Materials.View()
		if m.err != nil && m.Materials.Err() == nil {
			return view + "\n" + m.i18n.Text(i18n.CommonErrorPrefix) + ": " + m.err.Error() + "\n"
		}
		return view
	}
	if m.CurrentScreen == ScreenSearchProgress {
		view := m.Search.View()
		if m.err != nil && m.Search.Err() == nil {
			return view + "\n" + m.i18n.Text(i18n.CommonErrorPrefix) + ": " + m.err.Error() + "\n"
		}
		return view
	}
	if m.CurrentScreen == ScreenReferences {
		view := m.References.View()
		if m.err != nil && m.References.Err() == nil {
			return view + "\n" + m.i18n.Text(i18n.CommonErrorPrefix) + ": " + m.err.Error() + "\n"
		}
		return view
	}
	if m.CurrentScreen == ScreenWriting {
		return m.Writing.View()
	}
	if m.CurrentScreen == ScreenExportSummary {
		return m.ExportSummary.View()
	}
	if m.CurrentScreen == ScreenDone {
		return m.Done.View()
	}
	if m.err != nil {
		return fmt.Sprintf("aipaper-cli\n\n%s\n\n%s: %s\n", m.CurrentScreen, m.i18n.Text(i18n.CommonErrorPrefix), m.err)
	}
	return fmt.Sprintf("aipaper-cli\n\n%s\n", m.CurrentScreen)
}

func (m RootModel) Err() error {
	return m.err
}

// applySizeToActiveScreen forwards the last known terminal dimensions to the
// currently active screen so list-style views can wrap and scroll to fit the
// window. Screens without a SetSize method are left untouched. No-op when the
// dimensions are unknown (zero), preserving the unrestricted layout.
func (m RootModel) applySizeToActiveScreen() RootModel {
	if m.width <= 0 || m.height <= 0 {
		return m
	}
	switch m.CurrentScreen {
	case ScreenReferences:
		m.References = m.References.SetSize(m.width, m.height)
	case ScreenMaterialsScan:
		m.Materials = m.Materials.SetSize(m.width, m.height)
	case ScreenSearchProgress:
		m.Search = m.Search.SetSize(m.width, m.height)
	case ScreenExportSummary:
		m.ExportSummary = m.ExportSummary.SetSize(m.width, m.height)
	case ScreenDone:
		m.Done = m.Done.SetSize(m.width, m.height)
	}
	return m
}

func (m RootModel) updateRequirements(key string) (tea.Model, tea.Cmd) {
	if strings.ToLower(strings.TrimSpace(key)) == "enter" {
		req, createdMaterialDir, err := m.collectRequirements()
		if err != nil {
			m.err = err
			return m, nil
		}
		if _, err := store.WriteJSON(store.New(m.WorkDir).RequirementsPath(), req, store.Overwrite); err != nil {
			m.err = fmt.Errorf("write requirements failed: %w", err)
			return m, nil
		}
		m.CurrentScreen = ScreenMaterialsScan
		m.ScreenData = req
		materialsModel, err := newMaterialsModelWithCreatedDir(m.WorkDir, req, createdMaterialDir, m.i18n)
		if err != nil {
			m.err = err
			return m, nil
		}
		m.Materials = materialsModel
		m.err = nil
		return m, m.Materials.Init()
	}

	m.Requirements = m.Requirements.UpdateKey(key)
	if m.Requirements.Canceled() {
		return m, tea.Quit
	}
	m.err = nil
	return m, nil
}

func (m RootModel) updateMaterials(key string) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.Materials, cmd = m.Materials.UpdateKey(key)
	m.err = m.Materials.Err()
	switch m.Materials.Action() {
	case materialstui.ActionContinue, materialstui.ActionSkip:
		result := m.Materials.Result()
		searchModel, err := newSearchModel(m.WorkDir, result, m.i18n)
		if err != nil {
			m.err = err
			return m, nil
		}
		m.CurrentScreen = ScreenSearchProgress
		m.ScreenData = result
		m.Search = searchModel
		m.err = nil
		return m, m.Search.Init()
	case materialstui.ActionBack:
		m.CurrentScreen = ScreenRequirements
		m.ScreenData = materialstui.ActionBack
		m.Requirements = newRequirementsModel(m.i18n)
		m.err = nil
		return m, nil
	case materialstui.ActionCancel:
		return m, tea.Quit
	default:
		return m, cmd
	}
}

func (m RootModel) updateSearch(key string) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.Search, cmd = m.Search.UpdateKey(key)
	m.err = m.Search.Err()
	switch m.Search.Action() {
	case searchtui.ActionContinue, searchtui.ActionSkip:
		referencesModel, err := newReferencesModel(m.WorkDir, m.Search.Result(), m.i18n)
		if err != nil {
			m.err = err
			return m, nil
		}
		m.CurrentScreen = ScreenReferences
		m.ScreenData = m.Search.Result()
		m.References = referencesModel
		m.err = nil
		return m, nil
	case searchtui.ActionBack:
		m.CurrentScreen = ScreenMaterialsScan
		materialsModel, err := newMaterialsModel(m.WorkDir, nil, m.i18n)
		if err != nil {
			m.err = err
			return m, nil
		}
		m.Materials = materialsModel
		m.err = nil
		return m, m.Materials.Init()
	case searchtui.ActionCancel:
		return m, tea.Quit
	default:
		return m, cmd
	}
}

func (m RootModel) collectRequirements() (contracts.Requirements, bool, error) {
	materialDir := strings.TrimSpace(m.Requirements.FieldValue(requirementstui.FieldMaterialDir))
	createdMaterialDir := false
	if materialDir != "" {
		resolvedMaterialDir := resolveWorkDirPath(m.WorkDir, materialDir)
		info, err := os.Stat(resolvedMaterialDir)
		if err != nil {
			if !os.IsNotExist(err) {
				return contracts.Requirements{}, false, fmt.Errorf("stat material dir failed: %w", err)
			}
			if err := os.MkdirAll(resolvedMaterialDir, 0o755); err != nil {
				return contracts.Requirements{}, false, fmt.Errorf("create material dir failed: %w", err)
			}
			createdMaterialDir = true
		} else if !info.IsDir() {
			return contracts.Requirements{}, false, fmt.Errorf("material dir is not a directory")
		}
	}

	form := m.Requirements
	if materialDir != "" && !filepath.IsAbs(materialDir) {
		form = form.SetField(requirementstui.FieldMaterialDir, resolveWorkDirPath(m.WorkDir, materialDir))
	}
	req, err := form.Requirements()
	if err != nil {
		return contracts.Requirements{}, false, err
	}
	if materialDir != "" {
		req.MaterialDir = materialDir
	}
	normalizeRequirementSlices(&req)
	return req, createdMaterialDir, nil
}

func newMaterialsModel(workDir string, data any, trs ...i18n.T) (materialstui.Model, error) {
	return newMaterialsModelWithCreatedDir(workDir, data, false, trs...)
}

func newMaterialsModelWithCreatedDir(workDir string, data any, createdMaterialDir bool, trs ...i18n.T) (materialstui.Model, error) {
	tr := i18n.New("")
	if len(trs) > 0 && !trs[0].IsZero() {
		tr = trs[0]
	}
	req, err := requirementsForMaterials(workDir, data)
	if err != nil {
		return materialstui.Model{}, err
	}
	return materialstui.NewModel(materialstui.Options{
		WorkDir:     workDir,
		MaterialDir: req.MaterialDir,
		CreatedDir:  createdMaterialDir,
		I18N:        tr,
	}), nil
}

func requirementsForMaterials(workDir string, data any) (contracts.Requirements, error) {
	switch value := data.(type) {
	case contracts.Requirements:
		return value, nil
	case *contracts.Requirements:
		if value != nil {
			return *value, nil
		}
	}
	var req contracts.Requirements
	if err := store.ReadJSON(store.New(workDir).RequirementsPath(), &req); err != nil {
		return contracts.Requirements{}, fmt.Errorf("load requirements for materials scan failed: %w", err)
	}
	return req, nil
}

func newRequirementsModel(tr i18n.T) requirementstui.Model {
	return requirementstui.NewModel(contracts.Requirements{
		Language:          "zh-CN",
		CitationStyle:     "gbt7714",
		TargetWords:       8000,
		MaterialDir:       "./materials",
		AllowOnlineSearch: true,
		SearchProviders: []string{
			search.SemanticScholarProviderName,
			search.CrossrefProviderName,
		},
		ResearchQuestions:  []string{},
		ChapterPreferences: []string{},
		Constraints:        []string{},
	}, requirementstui.Options{I18N: tr})
}

func resolveWorkDirPath(workDir, path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	if workDir == "" {
		workDir = "."
	}
	return filepath.Clean(filepath.Join(workDir, path))
}

func normalizeRequirementSlices(req *contracts.Requirements) {
	if req.ResearchQuestions == nil {
		req.ResearchQuestions = []string{}
	}
	if req.ChapterPreferences == nil {
		req.ChapterPreferences = []string{}
	}
	if req.Constraints == nil {
		req.Constraints = []string{}
	}
}

func newSearchModel(workDir string, data any, trs ...i18n.T) (searchtui.Model, error) {
	tr := i18n.New("")
	if len(trs) > 0 && !trs[0].IsZero() {
		tr = trs[0]
	}
	materialsResult, ok := data.(materialstui.ScanResult)
	if !ok {
		return searchtui.Model{}, fmt.Errorf("invalid data for search screen: expected materialstui.ScanResult")
	}
	s := store.New(workDir)
	var req contracts.Requirements
	if err := store.ReadJSON(s.RequirementsPath(), &req); err != nil {
		return searchtui.Model{}, fmt.Errorf("load requirements for search failed: %w", err)
	}
	return searchtui.NewModel(searchtui.Options{
		WorkDir:         workDir,
		Store:           s,
		Requirements:    req,
		MaterialsResult: materialsResult,
		I18N:            tr,
	}), nil
}

func newReferencesModel(workDir string, _ any, trs ...i18n.T) (referencestui.Model, error) {
	tr := i18n.New("")
	if len(trs) > 0 && !trs[0].IsZero() {
		tr = trs[0]
	}
	s := store.New(workDir)
	candidates, err := references.LoadCandidates(s)
	if err != nil {
		return referencestui.Model{}, fmt.Errorf("load candidates failed: %w", err)
	}
	return referencestui.NewModel(candidates, referencestui.Options{I18N: tr}), nil
}

func (m RootModel) updateReferences(key string) (tea.Model, tea.Cmd) {
	m.References = m.References.UpdateKey(key)
	m.err = m.References.Err()

	if m.References.Canceled() {
		// 用户取消，提供返回路径
		candidates := m.References.VisibleCandidates()
		if len(candidates) == 0 {
			// 候选为空时提供返回 SearchProgress、MaterialsScan 或退出
			// 这里简化为退出，实际可以添加更复杂的逻辑
			return m, tea.Quit
		}
		return m, tea.Quit
	}

	if m.References.Done() {
		// 用户确认，调用 ConfirmCandidates
		s := store.New(m.WorkDir)
		candidates, err := references.LoadCandidates(s)
		if err != nil {
			m.err = fmt.Errorf("reload candidates failed: %w", err)
			return m, nil
		}

		decision := m.References.Decision(time.Now().UTC())
		result, err := references.ConfirmCandidates(s, candidates, decision)
		if err != nil {
			if references.IsNoneConfirmed(err) {
				// 未确认任何文献，阻塞进入 WritingProgress
				m.err = err
				return m, nil
			}
			m.err = fmt.Errorf("confirm candidates failed: %w", err)
			return m, nil
		}

		// 成功确认，转场到 WritingProgress
		m.CurrentScreen = ScreenWriting
		m.ScreenData = result
		m.Writing = newWritingModel(m.WorkDir, result, m.i18n)
		m.err = nil
		return m, tea.Batch(m.Writing.Init(), m.startWritingRuntimeCmd())
	}

	return m, nil
}

func newWritingModel(workDir string, data any, trs ...i18n.T) writingtui.Model {
	tr := i18n.New("")
	if len(trs) > 0 && !trs[0].IsZero() {
		tr = trs[0]
	}
	opts := writingtui.Options{
		WorkDir: workDir,
		Width:   120,
		Height:  40,
		I18N:    tr,
	}

	// Check if this is a recovery scenario
	if resumeData, ok := data.(WritingResumeData); ok {
		opts.RecoveryPrompt = resumeData.RecoveryPrompt
	}

	return writingtui.NewModel(opts)
}

// startWritingRuntimeCmd launches the real writing runtime in the background.
// The recovery prompt comes from WritingResumeData when the screen entered via
// the recovery path (B7).
func (m RootModel) startWritingRuntimeCmd() tea.Cmd {
	starter := m.runtimeStarter
	if starter == nil {
		starter = StartWritingRuntime
	}
	workDir := m.WorkDir
	recoveryPrompt := ""
	if resumeData, ok := m.ScreenData.(WritingResumeData); ok {
		recoveryPrompt = resumeData.RecoveryPrompt
	}
	return func() tea.Msg {
		rt, err := starter(workDir, recoveryPrompt)
		return writingRuntimeStartedMsg{runtime: rt, err: err}
	}
}

func (m RootModel) updateWriting(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg != nil {
		updated, cmd := m.Writing.Update(msg)
		if writingModel, ok := updated.(writingtui.Model); ok {
			m.Writing = writingModel
		}
		if cmd != nil {
			m.err = m.Writing.Err()
			return m, cmd
		}
	}
	m.err = m.Writing.Err()

	if m.Writing.Done() && m.Writing.Err() == nil {
		// Writing completed, transition to ExportSummary and trigger export.
		m.CurrentScreen = ScreenExportSummary
		m.ScreenData = nil
		m.ExportSummary = newExportSummaryModel(m.WorkDir, m.i18n)
		m.err = nil
		return m, m.ExportSummary.Init()
	}

	if m.Writing.Canceled() {
		return m, tea.Quit
	}

	return m, nil
}

func newExportSummaryModel(workDir string, tr i18n.T) exportsummarytui.Model {
	return exportsummarytui.NewModel(exportsummarytui.Options{WorkDir: workDir, I18N: tr})
}

func newDoneModel(workDir string, data any, tr i18n.T) donetui.Model {
	opts := donetui.Options{WorkDir: workDir, I18N: tr}
	if result, ok := data.(export.Result); ok {
		opts.Result = result
	}
	return donetui.NewModel(opts)
}

func (m RootModel) updateExportSummary(key string) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.ExportSummary, cmd = m.ExportSummary.UpdateKey(key)
	m.err = m.ExportSummary.Err()

	if m.ExportSummary.Canceled() {
		return m, tea.Quit
	}

	if m.ExportSummary.Done() {
		m.CurrentScreen = ScreenDone
		m.ScreenData = m.ExportSummary.Result()
		m.Done = newDoneModel(m.WorkDir, m.ExportSummary.Result(), m.i18n)
		m.err = nil
		return m, nil
	}

	return m, cmd
}

func (m RootModel) updateDone(key string) (tea.Model, tea.Cmd) {
	m.Done = m.Done.UpdateKey(key)
	if m.Done.Quit() {
		return m, tea.Quit
	}
	return m, nil
}

// shouldConfirmExit returns true if current screen should show exit confirmation.
func (m RootModel) shouldConfirmExit() bool {
	switch m.CurrentScreen {
	case ScreenRequirements:
		// For now, always allow quick exit from requirements
		// In future, could check if form has unsaved changes
		return false
	case ScreenConfigWizard:
		// Config wizard handles its own cancellation
		return false
	case ScreenWriting:
		return true
	case ScreenRecoverPrompt:
		// Recovery prompt handles its own exit
		return false
	case ScreenMaterialsScan, ScreenSearchProgress:
		// Background tasks - allow quick exit
		return false
	case ScreenReferences:
		// For now, allow quick exit from references
		// In future, could check if user has made selections
		return false
	default:
		return false
	}
}

// currentScreenView returns the view for the current screen without exit confirmation.
func (m RootModel) currentScreenView() string {
	if m.CurrentScreen == ScreenConfigWizard {
		return m.ConfigWizard.View()
	}
	if m.CurrentScreen == ScreenRecoverPrompt {
		return m.RecoverPrompt.View()
	}
	if m.CurrentScreen == ScreenRequirements {
		return m.Requirements.View()
	}
	if m.CurrentScreen == ScreenMaterialsScan {
		return m.Materials.View()
	}
	if m.CurrentScreen == ScreenSearchProgress {
		return m.Search.View()
	}
	if m.CurrentScreen == ScreenReferences {
		return m.References.View()
	}
	if m.CurrentScreen == ScreenWriting {
		return m.Writing.View()
	}
	if m.CurrentScreen == ScreenExportSummary {
		return m.ExportSummary.View()
	}
	if m.CurrentScreen == ScreenDone {
		return m.Done.View()
	}
	return fmt.Sprintf("aipaper-cli\n\n%s\n", m.CurrentScreen)
}
