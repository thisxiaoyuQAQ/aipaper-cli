package done

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/export"
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
}

// Model represents the completion screen state.
type Model struct {
	store  store.Store
	result export.Result
	quit   bool
}

// NewModel creates a new Done model.
func NewModel(opts Options) Model {
	workDir := opts.WorkDir
	if workDir == "" {
		workDir = "."
	}
	return Model{
		store:  store.New(workDir),
		result: opts.Result,
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

	b.WriteString(titleStyle.Render("✓ 全部完成"))
	b.WriteString("\n\n")

	b.WriteString("输出目录: ")
	b.WriteString(pathStyle.Render(m.store.Path("final")))
	b.WriteString("\n\n")

	b.WriteString("生成文件:\n")
	if len(m.result.Outputs) == 0 {
		b.WriteString(hintStyle.Render("  (无)"))
		b.WriteString("\n")
	} else {
		for _, out := range m.result.Outputs {
			b.WriteString("  - ")
			b.WriteString(pathStyle.Render(out.Path))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString("下一步:\n")
	b.WriteString(hintStyle.Render("  - 使用 recover 子命令可在中断后恢复写作进度"))
	b.WriteString("\n")
	b.WriteString(hintStyle.Render("  - 使用 status 子命令可查看当前 run 状态"))
	b.WriteString("\n")
	b.WriteString(hintStyle.Render("  - 使用 config 子命令可调整配置"))
	b.WriteString("\n\n")

	b.WriteString(hintStyle.Render("[enter]/[q]/[ctrl+c] 退出"))
	b.WriteString("\n")
	return b.String()
}
