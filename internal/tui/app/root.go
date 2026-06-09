package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	runtimeapp "github.com/thisxiaoyuQAQ/aipaper-cli/internal/app"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/references"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/search"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/store"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/tui/configwizard"
	materialstui "github.com/thisxiaoyuQAQ/aipaper-cli/internal/tui/materials"
	referencestui "github.com/thisxiaoyuQAQ/aipaper-cli/internal/tui/references"
	requirementstui "github.com/thisxiaoyuQAQ/aipaper-cli/internal/tui/requirements"
	searchtui "github.com/thisxiaoyuQAQ/aipaper-cli/internal/tui/search"
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
	Probe         ProbeResult
	recover       func(string) (runtimeapp.RecoveryResult, error)

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
	m := RootModel{
		WorkDir:      opts.WorkDir,
		ConfigWizard: configwizard.NewModel(configwizard.Options{WorkDir: opts.WorkDir}),
		recover:      recoverFn,
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
		m.RecoverPrompt = NewRecoverPromptModel(m.Probe)
	}
	if screen == ScreenRequirements {
		m.Requirements = newRequirementsModel()
	}
	if screen == ScreenMaterialsScan {
		materialsModel, err := newMaterialsModel(m.WorkDir, m.ScreenData)
		if err != nil {
			m.err = err
			m.CurrentScreen = ScreenRequirements
			m.Requirements = newRequirementsModel()
		} else {
			m.Materials = materialsModel
		}
	}
	if screen == ScreenReferences {
		referencesModel, err := newReferencesModel(m.WorkDir, m.ScreenData)
		if err != nil {
			m.err = err
		} else {
			m.References = referencesModel
		}
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
	return nil
}

func (m RootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case ScreenTransitionMsg:
		if !KnownScreen(msg.Next) {
			m.err = fmt.Errorf("unknown screen %q", msg.Next)
			return m, nil
		}
		var materialsModel materialstui.Model
		var searchModel searchtui.Model
		if msg.Next == ScreenMaterialsScan {
			var err error
			materialsModel, err = newMaterialsModel(m.WorkDir, msg.Data)
			if err != nil {
				m.err = err
				return m, nil
			}
		}
		if msg.Next == ScreenSearchProgress {
			var err error
			searchModel, err = newSearchModel(m.WorkDir, msg.Data)
			if err != nil {
				m.err = err
				return m, nil
			}
		}
		var referencesModel referencestui.Model
		if msg.Next == ScreenReferences {
			var err error
			referencesModel, err = newReferencesModel(m.WorkDir, msg.Data)
			if err != nil {
				m.err = err
				return m, nil
			}
		}
		m.CurrentScreen = msg.Next
		m.ScreenData = msg.Data
		m.err = nil
		if msg.Next == ScreenConfigWizard {
			m.ConfigWizard = configwizard.NewModel(configwizard.Options{WorkDir: m.WorkDir})
		}
		if msg.Next == ScreenRecoverPrompt {
			if probe, ok := msg.Data.(ProbeResult); ok {
				m.Probe = probe
			}
			m.RecoverPrompt = NewRecoverPromptModel(m.Probe)
		}
		if msg.Next == ScreenRequirements {
			m.Requirements = newRequirementsModel()
		}
		if msg.Next == ScreenMaterialsScan {
			m.Materials = materialsModel
			return m, m.Materials.Init()
		}
		if msg.Next == ScreenSearchProgress {
			m.Search = searchModel
			return m, m.Search.Init()
		}
		if msg.Next == ScreenReferences {
			m.References = referencesModel
			return m, nil
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
	case tea.KeyMsg:
		if m.CurrentScreen == ScreenConfigWizard {
			m.ConfigWizard = m.ConfigWizard.UpdateKey(msg.String())
			if m.ConfigWizard.Done() {
				m.CurrentScreen = ScreenRequirements
				m.ScreenData = m.ConfigWizard.SavedPath()
				m.Requirements = newRequirementsModel()
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
				m.err = nil
				return m, nil
			case RecoverActionRestart:
				m.CurrentScreen = ScreenRequirements
				m.ScreenData = RecoverActionRestart
				m.Requirements = newRequirementsModel()
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
	if m.CurrentScreen == ScreenRecoverPrompt {
		view := m.RecoverPrompt.View()
		if m.err != nil {
			return view + "\nError: " + m.err.Error() + "\n"
		}
		return view
	}
	if m.CurrentScreen == ScreenRequirements {
		view := m.Requirements.View()
		if m.err != nil {
			return view + "\nError: " + m.err.Error() + "\n"
		}
		return view
	}
	if m.CurrentScreen == ScreenMaterialsScan {
		view := m.Materials.View()
		if m.err != nil && m.Materials.Err() == nil {
			return view + "\nError: " + m.err.Error() + "\n"
		}
		return view
	}
	if m.CurrentScreen == ScreenSearchProgress {
		view := m.Search.View()
		if m.err != nil && m.Search.Err() == nil {
			return view + "\nError: " + m.err.Error() + "\n"
		}
		return view
	}
	if m.CurrentScreen == ScreenReferences {
		view := m.References.View()
		if m.err != nil && m.References.Err() == nil {
			return view + "\nError: " + m.err.Error() + "\n"
		}
		return view
	}
	if m.err != nil {
		return fmt.Sprintf("aipaper-cli\n\n%s\n\nError: %s\n", m.CurrentScreen, m.err)
	}
	return fmt.Sprintf("aipaper-cli\n\n%s\n", m.CurrentScreen)
}

func (m RootModel) Err() error {
	return m.err
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
		materialsModel, err := newMaterialsModelWithCreatedDir(m.WorkDir, req, createdMaterialDir)
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
		m.CurrentScreen = ScreenSearchProgress
		m.ScreenData = m.Materials.Result()
		m.err = nil
		return m, nil
	case materialstui.ActionBack:
		m.CurrentScreen = ScreenRequirements
		m.ScreenData = materialstui.ActionBack
		m.Requirements = newRequirementsModel()
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
		m.CurrentScreen = ScreenReferences
		m.ScreenData = m.Search.Result()
		m.err = nil
		return m, nil
	case searchtui.ActionBack:
		m.CurrentScreen = ScreenMaterialsScan
		materialsModel, err := newMaterialsModel(m.WorkDir, nil)
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

func newMaterialsModel(workDir string, data any) (materialstui.Model, error) {
	return newMaterialsModelWithCreatedDir(workDir, data, false)
}

func newMaterialsModelWithCreatedDir(workDir string, data any, createdMaterialDir bool) (materialstui.Model, error) {
	req, err := requirementsForMaterials(workDir, data)
	if err != nil {
		return materialstui.Model{}, err
	}
	return materialstui.NewModel(materialstui.Options{
		WorkDir:     workDir,
		MaterialDir: req.MaterialDir,
		CreatedDir:  createdMaterialDir,
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

func newRequirementsModel() requirementstui.Model {
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
	})
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

func newSearchModel(workDir string, data any) (searchtui.Model, error) {
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
	}), nil
}

func newReferencesModel(workDir string, data any) (referencestui.Model, error) {
	s := store.New(workDir)
	candidates, err := references.LoadCandidates(s)
	if err != nil {
		return referencestui.Model{}, fmt.Errorf("load candidates failed: %w", err)
	}
	return referencestui.NewModel(candidates), nil
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
		m.err = nil
		return m, nil
	}

	return m, nil
}
