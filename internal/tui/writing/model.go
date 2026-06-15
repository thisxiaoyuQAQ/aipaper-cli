package writing

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/i18n"

	tea "github.com/charmbracelet/bubbletea"
)

// staleThreshold is how long the writing screen may go without any runtime
// event before the footer shows a "no updates" hint. It complements the
// launcher heartbeat: the heartbeat keeps the screen alive when the agent loop
// blocks; this hint backstops the cases the heartbeat misses.
const staleThreshold = 20 * time.Second

type paneID int

const (
	paneMetrics paneID = iota
	paneLogs
	paneContent
	paneProgress
	paneCount
)

func (p paneID) String() string {
	switch p {
	case paneMetrics:
		return "metrics"
	case paneLogs:
		return "logs"
	case paneContent:
		return "content"
	case paneProgress:
		return "progress"
	default:
		return "unknown"
	}
}

// Model represents the WritingProgress screen state.
type Model struct {
	// Metrics
	phase         string
	currentStep   string
	totalProgress float64
	wordCount     int
	model         string
	contextUsed   int64
	contextMax    int64
	totalTokens   int64
	totalCost     float64
	startTime     time.Time
	elapsed       time.Duration
	lastActivity  time.Time // updated on every runtime event; drives the stale hint

	// Pane focus and scroll positions.
	focusedPane       paneID
	metricsScrollPos  int
	progressScrollPos int

	// Logs (middle-top region)
	logs         []logEntry
	maxLogs      int
	autoScroll   bool
	logScrollPos int

	// Content (middle-bottom region)
	currentChapterID string
	contentBuffer    string
	contentLines     []string
	maxContentLines  int
	contentScrollPos int

	// Chapter progress (right region)
	chapters      map[string]*ChapterState
	chapterOrder  []string
	citationScore int

	// Bottom intervention input.
	input               string
	pendingInstructions int

	// Terminal dimensions
	width  int
	height int

	// Runtime state
	running        bool
	done           bool
	err            error
	canceled       bool
	stopping       bool
	stopRequested  bool
	pauseRequested bool
	paused         bool
	i18n           i18n.T
}

type logEntry struct {
	at      time.Time
	role    string
	message string
	isError bool
}

// Options for creating a new WritingProgress model.
type Options struct {
	WorkDir        string
	RecoveryPrompt string
	Width          int
	Height         int
	I18N           i18n.T
}

// NewModel creates a new WritingProgress model.
func NewModel(opts Options) Model {
	if opts.Width <= 0 {
		opts.Width = 120
	}
	if opts.Height <= 0 {
		opts.Height = 40
	}

	tr := opts.I18N
	if tr.IsZero() {
		tr = i18n.New("")
	}
	return Model{
		phase:            tr.Text(i18n.WritingStatusStarting),
		currentStep:      "",
		totalProgress:    0.0,
		wordCount:        0,
		model:            "--",
		contextUsed:      0,
		contextMax:       0,
		totalTokens:      0,
		totalCost:        0.0,
		startTime:        time.Now(),
		elapsed:          0,
		lastActivity:     time.Now(),
		focusedPane:      paneLogs,
		logs:             []logEntry{},
		maxLogs:          100,
		autoScroll:       true,
		logScrollPos:     0,
		currentChapterID: "",
		contentBuffer:    "",
		contentLines:     []string{},
		maxContentLines:  1000,
		contentScrollPos: 0,
		chapters:         make(map[string]*ChapterState),
		chapterOrder:     []string{},
		citationScore:    0,
		width:            opts.Width,
		height:           opts.Height,
		running:          true,
		done:             false,
		err:              nil,
		canceled:         false,
		stopping:         false,
		stopRequested:    false,
		pauseRequested:   false,
		paused:           false,
		i18n:             tr,
	}
}

// Init initializes the model.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update handles Bubble Tea messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.clampAllScrolls()
		return m, nil

	case RuntimeEventMsg:
		m.handleRuntimeEvent(RuntimeEvent(msg))
		return m, nil

	case RuntimeDoneMsg:
		m.running = false
		m.paused = false
		m.pauseRequested = false
		m.done = true
		if msg.Error != nil {
			m.err = msg.Error
			m.addLog(time.Now(), "system", fmt.Sprintf(m.i18n.Text(i18n.WritingLogRuntimeError), msg.Error), true)
		} else {
			m.addLog(time.Now(), "system", m.i18n.Text(i18n.WritingLogCompletedOK), false)
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleKeyMsg(msg)

	case tea.MouseMsg:
		return m.handleMouse(msg)

	default:
		return m, nil
	}
}

// UpdateKey updates the model based on key input. It is kept for older tests and
// screens; RootModel uses Update so commands are not lost.
func (m Model) UpdateKey(key string) Model {
	updated, _ := m.handleKey(key)
	return updated.(Model)
}

func (m Model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	return m.handleKey(msg.String())
}

func (m Model) handleKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "ctrl+c":
		// Ctrl+C is owned by RootModel now: exit/exit-confirm, not pause.
		return m, nil
	case "q":
		if !m.running || m.done || m.paused {
			return m, tea.Quit
		}
	case "left":
		m.moveFocus(-1)
	case "right":
		m.moveFocus(1)
	case "up":
		m.scrollFocused(-1)
	case "down":
		m.scrollFocused(1)
	case "pgup":
		m.scrollFocused(-5)
	case "pgdown":
		m.scrollFocused(5)
	case "esc":
		if m.running && !m.paused && !m.pauseRequested && !m.done {
			m.pauseRequested = true
			m.phase = m.i18n.Text(i18n.WritingPhasePauseWait)
			m.addLog(time.Now(), "system", m.i18n.Text(i18n.WritingLogPauseRequested), false)
			return m, func() tea.Msg { return RuntimePauseRequestedMsg{} }
		}
	case "enter":
		text := strings.TrimSpace(m.input)
		if text != "" {
			m.input = ""
			m.pendingInstructions++
			m.addLog(time.Now(), "user", m.i18n.Text(i18n.WritingLogInstructionSubmitted), false)
			return m, func() tea.Msg { return RuntimeInstructionSubmittedMsg{Text: text} }
		}
		if m.paused {
			m.pauseRequested = false
			m.addLog(time.Now(), "system", m.i18n.Text(i18n.WritingLogResume), false)
			return m, func() tea.Msg { return RuntimeResumeRequestedMsg{} }
		}
	case "backspace":
		m.deleteInputRune()
	case " ":
		if m.input == "" {
			m.autoScroll = !m.autoScroll
		} else {
			m.input += " "
		}
	default:
		if isPrintableKey(key) {
			m.input += key
		}
	}

	m.clampAllScrolls()
	return m, nil
}

func (m Model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.MouseWheelUp:
		m.scrollFocused(-1)
	case tea.MouseWheelDown:
		m.scrollFocused(1)
	}
	return m, nil
}

func (m *Model) moveFocus(delta int) {
	idx := int(m.focusedPane) + delta
	for idx < 0 {
		idx += int(paneCount)
	}
	m.focusedPane = paneID(idx % int(paneCount))
}

func (m *Model) scrollFocused(delta int) {
	switch m.focusedPane {
	case paneMetrics:
		m.metricsScrollPos += delta
	case paneLogs:
		m.autoScroll = false
		m.logScrollPos += delta
	case paneContent:
		m.contentScrollPos += delta
	case paneProgress:
		m.progressScrollPos += delta
	}
	m.clampAllScrolls()
}

func (m *Model) clampAllScrolls() {
	if m.metricsScrollPos < 0 {
		m.metricsScrollPos = 0
	}
	if m.logScrollPos < 0 {
		m.logScrollPos = 0
	}
	if m.contentScrollPos < 0 {
		m.contentScrollPos = 0
	}
	if m.progressScrollPos < 0 {
		m.progressScrollPos = 0
	}
}

func (m *Model) deleteInputRune() {
	if m.input == "" {
		return
	}
	_, size := utf8.DecodeLastRuneInString(m.input)
	if size <= 0 {
		m.input = ""
		return
	}
	m.input = m.input[:len(m.input)-size]
}

func isPrintableKey(key string) bool {
	if key == "" {
		return false
	}
	if strings.HasPrefix(key, "ctrl+") || strings.HasPrefix(key, "alt+") {
		return false
	}
	switch key {
	case "tab", "shift+tab", "capslock", "esc", "enter", "backspace", "delete", "insert", "home", "end", "up", "down", "left", "right", "pgup", "pgdown":
		return false
	}
	return utf8.RuneCountInString(key) == 1
}

func (m *Model) handleRuntimeEvent(ev RuntimeEvent) {
	m.elapsed = time.Since(m.startTime)
	m.lastActivity = time.Now()

	switch ev.Kind {
	case EventStepStarted:
		m.currentStep = ev.Step
		m.phase = formatPhase(ev.Role, ev.Step)
		m.addLog(ev.At, ev.Role, fmt.Sprintf(m.i18n.Text(i18n.WritingLogStarted), ev.Step), false)

	case EventStepDone:
		m.addLog(ev.At, ev.Role, fmt.Sprintf(m.i18n.Text(i18n.WritingLogCompleted), ev.Step), false)

	case EventStepFailed:
		m.addLog(ev.At, ev.Role, fmt.Sprintf(m.i18n.Text(i18n.WritingLogFailed), ev.Step, ev.Message), true)

	case EventRoleLog:
		m.addLog(ev.At, ev.Role, ev.Message, false)

	case EventContentDelta:
		if ev.Delta != "" {
			m.appendContent(ev.ChapterID, ev.Delta)
		}

	case EventUsageUpdate:
		if ev.Usage != nil {
			m.updateUsage(ev.Usage)
		}

	case EventChapterStatus:
		m.updateChapterStatus(ev)

	case EventQualityReview:
		m.addLog(ev.At, "editor", fmt.Sprintf(m.i18n.Text(i18n.WritingLogReview), ev.Message), false)

	case EventCheckpointSaved:
		m.addLog(ev.At, "system", m.i18n.Text(i18n.WritingLogCheckpoint), false)
		if isStoppedEvent(ev) {
			m.running = false
			m.paused = false
			m.pauseRequested = false
			m.canceled = true
			m.addLog(ev.At, "system", m.i18n.Text(i18n.WritingLogProgressSaved), false)
		}

	case EventExportArtifact:
		m.addLog(ev.At, "export", ev.Message, false)

	case EventPauseRequested:
		m.pauseRequested = true
		m.phase = m.i18n.Text(i18n.WritingPhasePauseWait)
		m.addLog(ev.At, "system", nonEmpty(ev.Message, m.i18n.Text(i18n.WritingLogPauseRequested)), false)

	case EventPaused:
		m.pauseRequested = false
		m.paused = true
		m.running = true
		m.phase = m.i18n.Text(i18n.WritingStatusPaused)
		m.addLog(ev.At, "system", nonEmpty(ev.Message, m.i18n.Text(i18n.WritingStatusPaused)), false)

	case EventResumed:
		m.pauseRequested = false
		m.paused = false
		m.running = true
		m.phase = m.i18n.Text(i18n.WritingPhaseRunning)
		m.addLog(ev.At, "system", nonEmpty(ev.Message, m.i18n.Text(i18n.WritingLogResume)), false)

	case EventInstructionQueued:
		m.pendingInstructions = pendingFromFields(ev.Fields, m.pendingInstructions)
		m.addLog(ev.At, "system", nonEmpty(ev.Message, "Instruction queued"), false)

	case EventInstructionApplied:
		m.pendingInstructions = pendingFromFields(ev.Fields, max(0, m.pendingInstructions-1))
		m.addLog(ev.At, "system", nonEmpty(ev.Message, "Instruction applied"), false)

	case EventRuntimeDone:
		m.running = false
		m.paused = false
		m.done = true
		m.addLog(ev.At, "system", m.i18n.Text(i18n.WritingLogRuntimeDone), false)

	case EventRuntimeError:
		m.running = false
		m.paused = false
		m.err = fmt.Errorf("%s", ev.Message)
		m.addLog(ev.At, "system", fmt.Sprintf(m.i18n.Text(i18n.WritingLogError), ev.Message), true)
	}

	if m.autoScroll {
		maxScroll := len(m.logs) - m.visibleLogLines()
		if maxScroll > 0 {
			m.logScrollPos = maxScroll
		}
	}
	m.clampAllScrolls()
}

func isStoppedEvent(ev RuntimeEvent) bool {
	if ev.Fields == nil {
		return false
	}
	stopped, ok := ev.Fields["stop_requested"].(bool)
	return ok && stopped
}

func pendingFromFields(fields map[string]any, fallback int) int {
	if fields == nil {
		return fallback
	}
	switch v := fields["pending_instructions"].(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	default:
		return fallback
	}
}

func nonEmpty(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func (m *Model) addLog(at time.Time, role, message string, isError bool) {
	m.logs = append(m.logs, logEntry{
		at:      at,
		role:    role,
		message: message,
		isError: isError,
	})
	if len(m.logs) > m.maxLogs {
		m.logs = m.logs[len(m.logs)-m.maxLogs:]
	}
}

func (m *Model) appendContent(chapterID, delta string) {
	if chapterID != "" && chapterID != m.currentChapterID {
		m.currentChapterID = chapterID
		m.contentBuffer = ""
		m.contentLines = []string{}
		m.contentScrollPos = 0
	}

	m.contentBuffer += delta
	m.contentLines = splitLines(m.contentBuffer)
	if len(m.contentLines) > m.maxContentLines {
		m.contentLines = m.contentLines[len(m.contentLines)-m.maxContentLines:]
	}

	m.wordCount = estimateWordCount(m.contentBuffer)
}

func (m *Model) updateUsage(usage *UsageSnapshot) {
	if usage.InputTokens != nil {
		m.totalTokens = *usage.InputTokens
	}
	if usage.OutputTokens != nil {
		m.totalTokens += *usage.OutputTokens
	}
	if usage.ContextTokens != nil {
		m.contextUsed = *usage.ContextTokens
	}
	if usage.MaxContextTokens != nil {
		m.contextMax = *usage.MaxContextTokens
	}
	if usage.CostUSD != nil {
		m.totalCost = *usage.CostUSD
	}
	if usage.Model != "" {
		m.model = usage.Model
	}
}

func (m *Model) updateChapterStatus(ev RuntimeEvent) {
	if ev.ChapterID == "" {
		return
	}

	state, exists := m.chapters[ev.ChapterID]
	if !exists {
		state = &ChapterState{
			ID:             ev.ChapterID,
			Status:         ChapterPending,
			DraftVersion:   0,
			RevisionRounds: 0,
			WordCount:      0,
		}
		m.chapters[ev.ChapterID] = state
		m.chapterOrder = append(m.chapterOrder, ev.ChapterID)
	}

	if statusStr, ok := ev.Fields["status"].(string); ok {
		state.Status = ChapterStatus(statusStr)
	}
	if version, ok := ev.Fields["draft_version"].(int); ok {
		state.DraftVersion = version
	}
	if score, ok := ev.Fields["score"].(int); ok {
		state.Score = &score
	}
	if citScore, ok := ev.Fields["citation_score"].(int); ok {
		state.CitationScore = &citScore
	}
	if wc, ok := ev.Fields["word_count"].(int); ok {
		state.WordCount = wc
	}

	m.calculateProgress()
}

func (m *Model) calculateProgress() {
	if len(m.chapters) == 0 {
		m.totalProgress = 0.0
		return
	}

	completed := 0
	for _, state := range m.chapters {
		if state.Status == ChapterDone || state.Status == ChapterNeedsReview {
			completed++
		}
	}
	m.totalProgress = float64(completed) / float64(len(m.chapters)) * 100.0
}

func (m Model) visibleLogLines() int {
	return max(5, m.height/3)
}

// Done returns true if the model is finished.
func (m Model) Done() bool {
	return m.done && !m.running
}

// Err returns any error.
func (m Model) Err() error {
	return m.err
}

// Canceled returns true if user requested stop.
func (m Model) Canceled() bool {
	return m.canceled
}

// Stopping returns true if stop is in progress.
func (m Model) Stopping() bool {
	return m.stopping
}

// StopRequested returns true if user requested stop.
func (m Model) StopRequested() bool {
	return m.stopRequested
}

func (m Model) Paused() bool {
	return m.paused
}

func (m Model) PauseRequested() bool {
	return m.pauseRequested
}

// Helper functions

func formatPhase(role, step string) string {
	if role != "" && step != "" {
		return fmt.Sprintf("%s: %s", role, step)
	}
	if step != "" {
		return step
	}
	if role != "" {
		return role
	}
	return "Running"
}

func splitLines(content string) []string {
	if content == "" {
		return []string{}
	}
	lines := []string{}
	current := ""
	for _, ch := range content {
		if ch == '\n' {
			lines = append(lines, current)
			current = ""
		} else {
			current += string(ch)
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}

func estimateWordCount(content string) int {
	if content == "" {
		return 0
	}
	words := 0
	inWord := false
	for _, ch := range content {
		if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' {
			if inWord {
				words++
				inWord = false
			}
		} else {
			inWord = true
		}
	}
	if inWord {
		words++
	}
	return words
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
