package writing

import (
	"testing"
	"time"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
)

func TestNewModel(t *testing.T) {
	m := NewModel(Options{
		WorkDir: "/tmp/test",
		Width:   120,
		Height:  40,
	})

	if m.width != 120 {
		t.Errorf("expected width 120, got %d", m.width)
	}
	if m.height != 40 {
		t.Errorf("expected height 40, got %d", m.height)
	}
	if !m.running {
		t.Error("expected running to be true")
	}
	if m.done {
		t.Error("expected done to be false")
	}
	if m.autoScroll != true {
		t.Error("expected autoScroll to be true")
	}
}

func TestHandleRuntimeEvent_StepStarted(t *testing.T) {
	m := NewModel(Options{Width: 120, Height: 40})

	ev := RuntimeEvent{
		At:   time.Now(),
		Kind: EventStepStarted,
		Role: "coordinator",
		Step: "create_outline",
	}

	m.handleRuntimeEvent(ev)

	if m.currentStep != "create_outline" {
		t.Errorf("expected currentStep 'create_outline', got %q", m.currentStep)
	}

	if len(m.logs) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(m.logs))
	}

	if m.logs[0].role != "coordinator" {
		t.Errorf("expected log role 'coordinator', got %q", m.logs[0].role)
	}
}

func TestHandleRuntimeEvent_ContentDelta(t *testing.T) {
	m := NewModel(Options{Width: 120, Height: 40})

	// First delta
	ev1 := RuntimeEvent{
		At:        time.Now(),
		Kind:      EventContentDelta,
		ChapterID: "ch01",
		Delta:     "This is the first ",
	}
	m.handleRuntimeEvent(ev1)

	if m.currentChapterID != "ch01" {
		t.Errorf("expected currentChapterID 'ch01', got %q", m.currentChapterID)
	}
	if m.contentBuffer != "This is the first " {
		t.Errorf("unexpected contentBuffer: %q", m.contentBuffer)
	}

	// Second delta
	ev2 := RuntimeEvent{
		At:        time.Now(),
		Kind:      EventContentDelta,
		ChapterID: "ch01",
		Delta:     "sentence.\nAnd a second line.",
	}
	m.handleRuntimeEvent(ev2)

	expected := "This is the first sentence.\nAnd a second line."
	if m.contentBuffer != expected {
		t.Errorf("expected contentBuffer %q, got %q", expected, m.contentBuffer)
	}

	if len(m.contentLines) != 2 {
		t.Errorf("expected 2 content lines, got %d", len(m.contentLines))
	}
}

func TestHandleRuntimeEvent_ChapterSwitch(t *testing.T) {
	m := NewModel(Options{Width: 120, Height: 40})

	// First chapter
	ev1 := RuntimeEvent{
		At:        time.Now(),
		Kind:      EventContentDelta,
		ChapterID: "ch01",
		Delta:     "Chapter 1 content",
	}
	m.handleRuntimeEvent(ev1)

	if m.wordCount == 0 {
		t.Error("expected wordCount > 0")
	}

	// Switch to second chapter
	ev2 := RuntimeEvent{
		At:        time.Now(),
		Kind:      EventContentDelta,
		ChapterID: "ch02",
		Delta:     "Chapter 2 content",
	}
	m.handleRuntimeEvent(ev2)

	if m.currentChapterID != "ch02" {
		t.Errorf("expected currentChapterID 'ch02', got %q", m.currentChapterID)
	}

	// Content buffer should reset
	if m.contentBuffer != "Chapter 2 content" {
		t.Errorf("expected buffer reset, got %q", m.contentBuffer)
	}
}

func TestHandleRuntimeEvent_UsageUpdate(t *testing.T) {
	m := NewModel(Options{Width: 120, Height: 40})

	input := int64(1000)
	output := int64(2000)
	cost := 0.05

	ev := RuntimeEvent{
		At:   time.Now(),
		Kind: EventUsageUpdate,
		Usage: &UsageSnapshot{
			InputTokens:  &input,
			OutputTokens: &output,
			CostUSD:      &cost,
			Model:        "gpt-5.5",
		},
	}

	m.handleRuntimeEvent(ev)

	if m.totalTokens != 3000 {
		t.Errorf("expected totalTokens 3000, got %d", m.totalTokens)
	}
	if m.totalCost != 0.05 {
		t.Errorf("expected totalCost 0.05, got %f", m.totalCost)
	}
	if m.model != "gpt-5.5" {
		t.Errorf("expected model 'gpt-5.5', got %q", m.model)
	}
}

func TestHandleRuntimeEvent_ChapterStatus(t *testing.T) {
	m := NewModel(Options{Width: 120, Height: 40})

	ev := RuntimeEvent{
		At:        time.Now(),
		Kind:      EventChapterStatus,
		ChapterID: "ch01",
		Fields: map[string]any{
			"status":         "writing",
			"draft_version":  1,
			"score":          85,
			"citation_score": 92,
			"word_count":     1200,
		},
	}

	m.handleRuntimeEvent(ev)

	state, exists := m.chapters["ch01"]
	if !exists {
		t.Fatal("expected chapter ch01 to exist")
	}

	if state.Status != ChapterWriting {
		t.Errorf("expected status 'writing', got %q", state.Status)
	}
	if state.DraftVersion != 1 {
		t.Errorf("expected draft_version 1, got %d", state.DraftVersion)
	}
	if state.Score == nil || *state.Score != 85 {
		t.Errorf("expected score 85, got %v", state.Score)
	}
	if state.CitationScore == nil || *state.CitationScore != 92 {
		t.Errorf("expected citation_score 92, got %v", state.CitationScore)
	}
	if state.WordCount != 1200 {
		t.Errorf("expected word_count 1200, got %d", state.WordCount)
	}

	if len(m.chapterOrder) != 1 || m.chapterOrder[0] != "ch01" {
		t.Errorf("unexpected chapterOrder: %v", m.chapterOrder)
	}
}

func TestHandleRuntimeEvent_MultipleChaptersProgress(t *testing.T) {
	m := NewModel(Options{Width: 120, Height: 40})

	chapters := []struct {
		id     string
		status string
	}{
		{"ch01", "done"},
		{"ch02", "reviewing"},
		{"ch03", "pending"},
	}

	for _, ch := range chapters {
		ev := RuntimeEvent{
			At:        time.Now(),
			Kind:      EventChapterStatus,
			ChapterID: ch.id,
			Fields: map[string]any{
				"status": ch.status,
			},
		}
		m.handleRuntimeEvent(ev)
	}

	if len(m.chapters) != 3 {
		t.Errorf("expected 3 chapters, got %d", len(m.chapters))
	}

	// Progress: 1 done out of 3 = 33.33%
	expectedProgress := 33.3
	if m.totalProgress < expectedProgress-1 || m.totalProgress > expectedProgress+1 {
		t.Errorf("expected progress ~%.1f%%, got %.1f%%", expectedProgress, m.totalProgress)
	}
}

func TestHandleRuntimeEvent_RuntimeDone(t *testing.T) {
	m := NewModel(Options{Width: 120, Height: 40})

	ev := RuntimeEvent{
		At:      time.Now(),
		Kind:    EventRuntimeDone,
		Message: "All chapters completed",
	}

	m.handleRuntimeEvent(ev)

	if m.running {
		t.Error("expected running to be false")
	}
	if !m.done {
		t.Error("expected done to be true")
	}
	if m.err != nil {
		t.Errorf("expected no error, got %v", m.err)
	}
}

func TestHandleRuntimeEvent_RuntimeError(t *testing.T) {
	m := NewModel(Options{Width: 120, Height: 40})

	ev := RuntimeEvent{
		At:      time.Now(),
		Kind:    EventRuntimeError,
		Message: "API connection failed",
	}

	m.handleRuntimeEvent(ev)

	if m.running {
		t.Error("expected running to be false")
	}
	if m.err == nil {
		t.Error("expected error to be set")
	}
	if m.err.Error() != "API connection failed" {
		t.Errorf("unexpected error message: %v", m.err)
	}
}

func TestUpdateKey_CtrlC_WhileRunning(t *testing.T) {
	m := NewModel(Options{Width: 120, Height: 40})
	m.running = true

	m = m.UpdateKey("ctrl+c")

	// After Ctrl+C, should be stopping (not immediately canceled)
	if !m.stopRequested {
		t.Error("expected stopRequested to be true")
	}
	if !m.stopping {
		t.Error("expected stopping to be true")
	}
	if m.canceled {
		t.Error("expected canceled to be false until checkpoint saved")
	}
	if len(m.logs) == 0 {
		t.Error("expected stop request log entry")
	}
}

func TestUpdateKey_ToggleAutoScroll(t *testing.T) {
	m := NewModel(Options{Width: 120, Height: 40})
	initialAutoScroll := m.autoScroll

	m = m.UpdateKey(" ")

	if m.autoScroll == initialAutoScroll {
		t.Error("expected autoScroll to toggle")
	}
}

func TestEstimateWordCount(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"", 0},
		{"hello", 1},
		{"hello world", 2},
		{"hello  world", 2},
		{"hello\nworld\tfoo", 3},
		{"  leading and trailing  ", 3},
	}

	for _, tt := range tests {
		result := estimateWordCount(tt.input)
		if result != tt.expected {
			t.Errorf("estimateWordCount(%q) = %d, expected %d", tt.input, result, tt.expected)
		}
	}
}

func TestSplitLines(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"", []string{}},
		{"single", []string{"single"}},
		{"line1\nline2", []string{"line1", "line2"}},
		{"line1\nline2\n", []string{"line1", "line2"}},
		{"a\nb\nc", []string{"a", "b", "c"}},
	}

	for _, tt := range tests {
		result := splitLines(tt.input)
		if len(result) != len(tt.expected) {
			t.Errorf("splitLines(%q) length = %d, expected %d", tt.input, len(result), len(tt.expected))
			t.Logf("  got: %#v", result)
			t.Logf("  expected: %#v", tt.expected)
			continue
		}
		for i := range result {
			if result[i] != tt.expected[i] {
				t.Errorf("splitLines(%q)[%d] = %q, expected %q", tt.input, i, result[i], tt.expected[i])
			}
		}
	}
}

func TestBridgeRunEvent(t *testing.T) {
	tests := []struct {
		name         string
		inputKind    string
		inputFields  map[string]any
		expectedKind RuntimeEventKind
		expectedStep string
	}{
		{
			name:         "tool_exec_start maps to EventStepStarted",
			inputKind:    "tool_exec_start",
			inputFields:  map[string]any{"tool": "requirements_read"},
			expectedKind: EventStepStarted,
			expectedStep: "requirements_read",
		},
		{
			name:         "tool_exec_end success maps to EventStepDone",
			inputKind:    "tool_exec_end",
			inputFields:  map[string]any{"tool": "writer_run", "is_error": false},
			expectedKind: EventStepDone,
			expectedStep: "writer_run",
		},
		{
			name:         "tool_exec_end error maps to EventStepFailed",
			inputKind:    "tool_exec_end",
			inputFields:  map[string]any{"tool": "writer_run", "is_error": true},
			expectedKind: EventStepFailed,
			expectedStep: "writer_run",
		},
		{
			name:         "agent_end maps to EventRuntimeDone",
			inputKind:    "agent_end",
			inputFields:  map[string]any{},
			expectedKind: EventRuntimeDone,
			expectedStep: "",
		},
		{
			name:         "error maps to EventRuntimeError",
			inputKind:    "error",
			inputFields:  map[string]any{},
			expectedKind: EventRuntimeError,
			expectedStep: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runEvent := contracts.RunEvent{
				At:      time.Now(),
				Kind:    tt.inputKind,
				Message: "test message",
				Fields:  tt.inputFields,
			}

			runtimeEv := BridgeRunEvent(runEvent)

			if runtimeEv.Kind != tt.expectedKind {
				t.Errorf("expected Kind %q, got %q", tt.expectedKind, runtimeEv.Kind)
			}
			if runtimeEv.Step != tt.expectedStep {
				t.Errorf("expected Step %q, got %q", tt.expectedStep, runtimeEv.Step)
			}
		})
	}
}
