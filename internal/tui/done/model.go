package done

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/export"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/i18n"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/store"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("42"))

	pathStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("250"))

	hintStyle = lipgloss.NewStyle().
			Faint(true)
)

// Options configures a Done model.
type Options struct {
	// WorkDir is the project working directory; the store root is derived
	// from it (output/aipaper).
	WorkDir string

	// Result is the export result produced by ExportSummary.
	Result export.Result

	I18N i18n.T
}

// Model represents the completion screen state.
type Model struct {
	store  store.Store
	result export.Result
	quit   bool
	i18n   i18n.T
}

// NewModel creates a new Done model.
func NewModel(opts Options) Model {
	workDir := opts.WorkDir
	if workDir == "" {
		workDir = "."
	}
	tr := opts.I18N
	if tr.IsZero() {
		tr = i18n.New("")
	}
	return Model{
		store:  store.New(workDir),
		result: opts.Result,
		i18n:   tr,
	}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update handles Bubble Tea messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg.String())
	default:
		return m, nil
	}
}

// UpdateKey updates the model based on key input.
func (m Model) UpdateKey(key string) Model {
	updated, _ := m.handleKey(key)
	return updated.(Model)
}

func (m Model) handleKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "q", "ctrl+c", "enter":
		m.quit = true
		return m, tea.Quit
	}
	return m, nil
}

// Quit reports whether the user requested to exit.
func (m Model) Quit() bool {
	return m.quit
}

// View renders the completion screen.
func (m Model) View() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render(m.i18n.Text(i18n.DoneTitle)))
	b.WriteString("\n\n")

	b.WriteString(m.i18n.Text(i18n.DoneOutputDir))
	b.WriteString(": ")
	b.WriteString(pathStyle.Render(m.store.Path("final")))
	b.WriteString("\n\n")

	b.WriteString(m.i18n.Text(i18n.DoneOutputs))
	b.WriteString(":\n")
	if len(m.result.Outputs) == 0 {
		b.WriteString(hintStyle.Render(m.i18n.Text(i18n.CommonNone)))
		b.WriteString("\n")
	} else {
		for _, out := range m.result.Outputs {
			b.WriteString("  - ")
			b.WriteString(pathStyle.Render(out.Path))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(m.i18n.Text(i18n.DoneNextSteps))
	b.WriteString(":\n")
	b.WriteString(hintStyle.Render(m.i18n.Text(i18n.DoneRecoverHint)))
	b.WriteString("\n")
	b.WriteString(hintStyle.Render(m.i18n.Text(i18n.DoneStatusHint)))
	b.WriteString("\n")
	b.WriteString(hintStyle.Render(m.i18n.Text(i18n.DoneConfigHint)))
	b.WriteString("\n\n")

	b.WriteString(hintStyle.Render(m.i18n.Text(i18n.DoneExitHint)))
	b.WriteString("\n")
	return b.String()
}
