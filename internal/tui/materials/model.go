package materials

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/i18n"
	domainmaterials "github.com/thisxiaoyuQAQ/aipaper-cli/internal/materials"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/store"
)

type Status string

const (
	StatusScanning  Status = "scanning"
	StatusEmpty     Status = "empty"
	StatusComplete  Status = "complete"
	StatusAllFailed Status = "all_failed"
	StatusError     Status = "error"
)

type Action string

const (
	ActionNone     Action = ""
	ActionContinue Action = "continue"
	ActionSkip     Action = "skip"
	ActionBack     Action = "back"
	ActionCancel   Action = "cancel"
)

type ScanFunc func(materialDir string, s store.Store) (domainmaterials.Result, error)

type Options struct {
	WorkDir     string
	MaterialDir string
	Store       store.Store
	Scan        ScanFunc
	CreatedDir  bool
	I18N        i18n.T
}

type Model struct {
	workDir     string
	materialDir string
	displayDir  string
	store       store.Store
	scan        ScanFunc

	status     Status
	stats      ScanStats
	manifest   contracts.MaterialManifest
	candidates []contracts.ReferenceCandidate
	outputs    []string

	createdDir bool
	precreated bool
	details    bool
	action     Action
	err        error
	i18n       i18n.T

	// width/height let the view wrap long manifest lines and fit the panel to
	// the window. Zero falls back to the unrestricted layout.
	width  int
	height int
}

// SetSize injects terminal dimensions for wrapping/fitting. No-op when unset.
func (m Model) SetSize(width, height int) Model {
	m.width = width
	m.height = height
	return m
}

// Width reports the last injected terminal width (0 when unset).
func (m Model) Width() int { return m.width }

// Height reports the last injected terminal height (0 when unset).
func (m Model) Height() int { return m.height }

type ScanStats struct {
	Total      int
	Parsed     int
	Failed     int
	Skipped    int
	Degraded   int
	Candidates int
}

type ScanResult struct {
	MaterialDir string
	Manifest    contracts.MaterialManifest
	Candidates  []contracts.ReferenceCandidate
	Outputs     []string
	Stats       ScanStats
	Skipped     bool
}

type ScanFinishedMsg struct {
	MaterialDir string
	CreatedDir  bool
	Result      domainmaterials.Result
	Err         error
}

func NewModel(opts Options) Model {
	workDir := opts.WorkDir
	if workDir == "" {
		workDir = "."
	}
	materialDir, displayDir := resolveMaterialDir(workDir, opts.MaterialDir)
	scan := opts.Scan
	if scan == nil {
		scan = domainmaterials.ProcessDir
	}
	s := opts.Store
	if s.Root() == "" {
		s = store.New(workDir)
	}
	tr := opts.I18N
	if tr.IsZero() {
		tr = i18n.New("")
	}
	return Model{
		workDir:     workDir,
		materialDir: materialDir,
		displayDir:  displayDir,
		store:       s,
		scan:        scan,
		status:      StatusScanning,
		createdDir:  opts.CreatedDir,
		precreated:  opts.CreatedDir,
		i18n:        tr,
	}
}

func (m Model) Init() tea.Cmd {
	return m.scanCmd()
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case ScanFinishedMsg:
		m.applyScanFinished(msg)
		return m, nil
	default:
		return m, nil
	}
}

func (m Model) UpdateKey(key string) (Model, tea.Cmd) {
	m.action = ActionNone
	key = normalizeKey(key)
	if key == "q" || key == "ctrl+c" || key == "esc" {
		m.action = ActionCancel
		return m, nil
	}
	if m.status == StatusScanning {
		return m, nil
	}
	switch key {
	case "d":
		m.details = !m.details
	case "b":
		m.action = ActionBack
	case "r":
		m.startScan()
		return m, m.scanCmd()
	case "s":
		if m.status == StatusEmpty || m.status == StatusAllFailed {
			m.action = ActionSkip
		}
	case "enter":
		switch m.status {
		case StatusComplete:
			m.action = ActionContinue
		case StatusEmpty, StatusAllFailed, StatusError:
			m.startScan()
			return m, m.scanCmd()
		}
	}
	return m, nil
}

func (m Model) Status() Status {
	return m.status
}

func (m Model) Stats() ScanStats {
	return m.stats
}

func (m Model) Manifest() contracts.MaterialManifest {
	return m.manifest
}

func (m Model) Candidates() []contracts.ReferenceCandidate {
	return append([]contracts.ReferenceCandidate(nil), m.candidates...)
}

func (m Model) CreatedDir() bool {
	return m.createdDir
}

func (m Model) Details() bool {
	return m.details
}

func (m Model) Action() Action {
	return m.action
}

func (m Model) Err() error {
	return m.err
}

func (m Model) Result() ScanResult {
	return ScanResult{
		MaterialDir: m.materialDir,
		Manifest:    m.manifest,
		Candidates:  append([]contracts.ReferenceCandidate(nil), m.candidates...),
		Outputs:     append([]string(nil), m.outputs...),
		Stats:       m.stats,
		Skipped:     m.action == ActionSkip,
	}
}

func (m *Model) startScan() {
	m.status = StatusScanning
	m.stats = ScanStats{}
	m.manifest = contracts.MaterialManifest{}
	m.candidates = nil
	m.outputs = nil
	m.createdDir = false
	m.precreated = false
	m.err = nil
}

func (m *Model) applyScanFinished(msg ScanFinishedMsg) {
	m.action = ActionNone
	m.createdDir = msg.CreatedDir || m.precreated
	m.precreated = false
	if msg.MaterialDir != "" {
		m.materialDir = msg.MaterialDir
	}
	if msg.Err != nil {
		m.status = StatusError
		m.err = msg.Err
		return
	}
	m.err = nil
	if msg.CreatedDir {
		m.status = StatusEmpty
		m.stats = ScanStats{}
		m.manifest = contracts.MaterialManifest{Items: []contracts.MaterialItem{}}
		m.candidates = nil
		m.outputs = nil
		return
	}

	m.manifest = msg.Result.Manifest
	m.candidates = append([]contracts.ReferenceCandidate(nil), msg.Result.Candidates...)
	m.outputs = append([]string(nil), msg.Result.Outputs...)
	m.stats = summarize(msg.Result)
	if len(m.manifest.Items) == 0 {
		m.status = StatusEmpty
		return
	}
	if m.stats.Parsed == 0 {
		m.status = StatusAllFailed
		return
	}
	m.status = StatusComplete
}

func (m Model) scanCmd() tea.Cmd {
	materialDir := m.materialDir
	s := m.store
	scan := m.scan
	return func() tea.Msg {
		info, err := os.Stat(materialDir)
		if err != nil {
			if os.IsNotExist(err) {
				if err := os.MkdirAll(materialDir, 0o755); err != nil {
					return ScanFinishedMsg{
						MaterialDir: materialDir,
						Err:         fmt.Errorf("%s", fmt.Sprintf(m.i18n.Text(i18n.MaterialsErrCreateDir), err)),
					}
				}
				return ScanFinishedMsg{MaterialDir: materialDir, CreatedDir: true}
			}
			return ScanFinishedMsg{MaterialDir: materialDir, Err: err}
		}
		if !info.IsDir() {
			return ScanFinishedMsg{
				MaterialDir: materialDir,
				Err:         fmt.Errorf("%s", m.i18n.Text(i18n.MaterialsErrPathNotDir)),
			}
		}
		result, err := scan(materialDir, s)
		return ScanFinishedMsg{MaterialDir: materialDir, Result: result, Err: err}
	}
}

func summarize(result domainmaterials.Result) ScanStats {
	stats := ScanStats{
		Total:      len(result.Manifest.Items),
		Candidates: len(result.Candidates),
	}
	for _, item := range result.Manifest.Items {
		switch item.Status {
		case domainmaterials.StatusParsed:
			stats.Parsed++
		case domainmaterials.StatusFailed:
			stats.Failed++
		case domainmaterials.StatusSkipped:
			stats.Skipped++
		}
		if item.Degraded {
			stats.Degraded++
		}
	}
	return stats
}

func resolveMaterialDir(workDir, materialDir string) (string, string) {
	display := strings.TrimSpace(materialDir)
	if display == "" {
		display = "./materials"
	}
	if filepath.IsAbs(display) {
		return filepath.Clean(display), display
	}
	if workDir == "" {
		workDir = "."
	}
	return filepath.Clean(filepath.Join(workDir, display)), display
}

func normalizeKey(key string) string {
	if key == " " {
		return "space"
	}
	return strings.ToLower(strings.TrimSpace(key))
}
