package exportsummary

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/export"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/i18n"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/store"
)

// ExportFunc runs the final export for the given store and returns its result.
// It is injectable so tests can mock export behaviour without touching disk.
type ExportFunc func(s store.Store) (export.Result, error)

// Options configures an ExportSummary model.
type Options struct {
	// WorkDir is the project working directory; the store root is
	// derived from it (output/aipaper).
	WorkDir string

	// Export overrides the export implementation. When nil, the default
	// implementation calls export.LoadInput followed by export.ExportFinal.
	Export ExportFunc

	// DocxExporter is forwarded to export.ExportFinal so tests can inject a
	// failing exporter to exercise docx degradation. nil uses the standard
	// implementation.
	DocxExporter export.DocxExporter

	// Now is forwarded to export.ExportFinal for deterministic versions in
	// tests. Zero value uses the current UTC time.
	Now time.Time

	I18N i18n.T
}

// Model represents the ExportSummary screen state.
type Model struct {
	workDir string
	store   store.Store
	export  ExportFunc

	exported bool
	result   export.Result
	err      error

	done     bool
	canceled bool
	i18n     i18n.T

	// width/height let the view wrap long output/issue lines to the window.
	width  int
	height int
}

// SetSize injects terminal dimensions for wrapping. No-op when unset.
func (m Model) SetSize(width, height int) Model {
	m.width = width
	m.height = height
	return m
}

// Width reports the last injected terminal width (0 when unset).
func (m Model) Width() int { return m.width }

// Height reports the last injected terminal height (0 when unset).
func (m Model) Height() int { return m.height }

// exportDoneMsg carries the outcome of an asynchronous export.
type exportDoneMsg struct {
	result export.Result
	err    error
}

// NewModel creates a new ExportSummary model.
func NewModel(opts Options) Model {
	workDir := opts.WorkDir
	if workDir == "" {
		workDir = "."
	}
	exportFn := opts.Export
	if exportFn == nil {
		exportFn = defaultExport(opts.DocxExporter, opts.Now)
	}
	tr := opts.I18N
	if tr.IsZero() {
		tr = i18n.New("")
	}
	return Model{
		workDir: workDir,
		store:   store.New(workDir),
		export:  exportFn,
		i18n:    tr,
	}
}

func defaultExport(docx export.DocxExporter, now time.Time) ExportFunc {
	return func(s store.Store) (export.Result, error) {
		input, err := export.LoadInput(s)
		if err != nil {
			return export.Result{}, err
		}
		return export.ExportFinal(s, input, export.Options{
			Now:          now,
			DocxExporter: docx,
		})
	}
}

// Init triggers the export.
func (m Model) Init() tea.Cmd {
	return m.runExport()
}

func (m Model) runExport() tea.Cmd {
	return func() tea.Msg {
		result, err := m.export(m.store)
		return exportDoneMsg{result: result, err: err}
	}
}

// Update handles Bubble Tea messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case exportDoneMsg:
		m.exported = true
		m.result = msg.result
		m.err = msg.err
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg.String())
	default:
		return m, nil
	}
}

// UpdateKey updates the model based on key input and returns any resulting
// command (for example, the retry export command triggered by "r").
func (m Model) UpdateKey(key string) (Model, tea.Cmd) {
	updated, cmd := m.handleKey(key)
	return updated.(Model), cmd
}

func (m Model) handleKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "r":
		// Retry export.
		m.exported = false
		m.err = nil
		m.result = export.Result{}
		return m, m.runExport()
	case "b", "esc":
		m.canceled = true
		return m, nil
	case "enter":
		if m.exported && m.err == nil {
			m.done = true
		}
		return m, nil
	}
	return m, nil
}

// ApplyExportResult records an export outcome without running the command. It
// exists so callers (such as the RootModel) can drive the export synchronously
// when transitioning into this screen.
func (m Model) ApplyExportResult(result export.Result, err error) Model {
	m.exported = true
	m.result = result
	m.err = err
	return m
}

// Run executes the export immediately and returns the updated model. It is a
// convenience for synchronous callers and tests.
func (m Model) Run() Model {
	result, err := m.export(m.store)
	return m.ApplyExportResult(result, err)
}

// Exported reports whether an export attempt has completed.
func (m Model) Exported() bool {
	return m.exported
}

// Done reports whether the user confirmed a successful export and wants to
// advance to the Done screen.
func (m Model) Done() bool {
	return m.done
}

// Err returns the export error, if any.
func (m Model) Err() error {
	return m.err
}

// Result returns the export result.
func (m Model) Result() export.Result {
	return m.result
}

// Canceled reports whether the user chose to go back / cancel.
func (m Model) Canceled() bool {
	return m.canceled
}

// Store returns the store backing this screen.
func (m Model) Store() store.Store {
	return m.store
}
