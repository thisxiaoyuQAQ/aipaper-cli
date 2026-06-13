package writing

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// staleThreshold is how long the writing screen may go without any runtime
// event before the footer shows a "no updates" hint. It complements the
// launcher heartbeat: the heartbeat keeps the screen alive when the agent loop
// blocks; this hint backstops the cases the heartbeat misses.
const staleThreshold = 20 * time.Second

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

	// Logs (middle-top region)
	logs          []logEntry
	maxLogs       int
	autoScroll    bool
	logScrollPos  int

	// Content (middle-bottom region)
	currentChapterID string
	contentBuffer    string
	contentLines     []string
	maxContentLines  int
	contentScrollPos int

	// Chapter progress (right region)
	chapters        map[string]*ChapterState
	chapterOrder    []string
	citationScore   int

	// Terminal dimensions
	width  int
	height int

	// Runtime state
	running       bool
	done          bool
	err           error
	canceled      bool
	stopping      bool
	stopRequested bool
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
}

// NewModel creates a new WritingProgress model.
func NewModel(opts Options) Model {
	if opts.Width <= 0 {
		opts.Width = 120
	}
	if opts.Height <= 0 {
		opts.Height = 40
	}

	return Model{
		phase:           "正在启动写作运行时…",
		currentStep:     "",
		totalProgress:   0.0,
		wordCount:       0,
		model:           "--",
		contextUsed:     0,
		contextMax:      0,
		totalTokens:     0,
		totalCost:       0.0,
		startTime:       time.Now(),
		elapsed:         0,
		lastActivity:    time.Now(),
		logs:            []logEntry{},
		maxLogs:         100,
		autoScroll:      true,
		logScrollPos:    0,
		currentChapterID: "",
		contentBuffer:   "",
		contentLines:    []string{},
		maxContentLines: 1000,
		contentScrollPos: 0,
		chapters:        make(map[string]*ChapterState),
		chapterOrder:    []string{},
		citationScore:   0,
		width:           opts.Width,
		height:          opts.Height,
		running:         true,
		done:            false,
		err:             nil,
		canceled:        false,
		stopping:        false,
		stopRequested:   false,
	}
}

// Init initializes the model.
func (m Model) Init() tea.Cmd {
	// Return a command to start the runtime
	// In real implementation, this would launch AgentRuntime
	return nil
}

// Update handles Bubble Tea messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case RuntimeEventMsg:
		m.handleRuntimeEvent(RuntimeEvent(msg))
		return m, nil

	case RuntimeDoneMsg:
		m.running = false
		m.done = true
		if msg.Error != nil {
			m.err = msg.Error
			m.addLog(time.Now(), "system", fmt.Sprintf("Runtime error: %v", msg.Error), true)
		} else {
			m.addLog(time.Now(), "system", "Writing completed successfully", false)
		}
		return m, nil

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
	case "ctrl+c":
		if m.running && !m.stopping {
			// Request graceful stop
			m.stopping = true
			m.stopRequested = true
			m.addLog(time.Now(), "system", "Stop requested, waiting for safe checkpoint...", false)
			// In real implementation, send stop signal to runtime
			// Return a command that will trigger runtime stop
			return m, func() tea.Msg {
				return RuntimeStopRequestedMsg{}
			}
		}
		if !m.running {
			return m, tea.Quit
		}
		return m, nil

	case "q":
		if !m.running {
			return m, tea.Quit
		}

	case "up":
		if !m.autoScroll && m.logScrollPos > 0 {
			m.logScrollPos--
		}

	case "down":
		if !m.autoScroll {
			maxScroll := len(m.logs) - m.visibleLogLines()
			if maxScroll > 0 && m.logScrollPos < maxScroll {
				m.logScrollPos++
			}
		}

	case " ":
		// Toggle auto-scroll
		m.autoScroll = !m.autoScroll
	}

	return m, nil
}

func (m *Model) handleRuntimeEvent(ev RuntimeEvent) {
	m.elapsed = time.Since(m.startTime)
	m.lastActivity = time.Now()

	switch ev.Kind {
	case EventStepStarted:
		m.currentStep = ev.Step
		m.phase = formatPhase(ev.Role, ev.Step)
		m.addLog(ev.At, ev.Role, fmt.Sprintf("Started: %s", ev.Step), false)

	case EventStepDone:
		m.addLog(ev.At, ev.Role, fmt.Sprintf("Completed: %s", ev.Step), false)

	case EventStepFailed:
		m.addLog(ev.At, ev.Role, fmt.Sprintf("Failed: %s - %s", ev.Step, ev.Message), true)

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
		m.addLog(ev.At, "editor", fmt.Sprintf("Review: %s", ev.Message), false)

	case EventCheckpointSaved:
		m.addLog(ev.At, "system", "Checkpoint saved", false)
		// If stop was requested and checkpoint is saved, mark as canceled
		if m.stopRequested && m.stopping {
			m.running = false
			m.canceled = true
			m.addLog(ev.At, "system", "Progress saved. You can continue next time.", false)
		}

	case EventExportArtifact:
		m.addLog(ev.At, "export", ev.Message, false)

	case EventRuntimeDone:
		m.running = false
		m.done = true
		m.addLog(ev.At, "system", "Writing completed", false)

	case EventRuntimeError:
		m.running = false
		m.err = fmt.Errorf("%s", ev.Message)
		m.addLog(ev.At, "system", fmt.Sprintf("Error: %s", ev.Message), true)
	}

	// Update auto-scroll position
	if m.autoScroll {
		maxScroll := len(m.logs) - m.visibleLogLines()
		if maxScroll > 0 {
			m.logScrollPos = maxScroll
		}
	}
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
		// Chapter switch
		m.currentChapterID = chapterID
		m.contentBuffer = ""
		m.contentLines = []string{}
	}

	m.contentBuffer += delta
	// Split into lines for display
	// Simple implementation: split on newline
	lines := splitLines(m.contentBuffer)
	m.contentLines = lines
	if len(m.contentLines) > m.maxContentLines {
		m.contentLines = m.contentLines[len(m.contentLines)-m.maxContentLines:]
	}

	// Update word count (rough estimate)
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

	// Update status from event fields
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

	// Calculate total progress
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
	// Reserve space for metrics and content
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
	// Simple word count: split on whitespace
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
