package writing

import (
	"testing"
	"time"
)

// TestBoundary_LogsTruncation 验证 logs 超过上限时正确截断
func TestBoundary_LogsTruncation(t *testing.T) {
	m := NewModel(Options{Width: 120, Height: 40})
	m.maxLogs = 10 // 降低上限便于测试

	// 添加超过上限的 logs
	for i := 0; i < 15; i++ {
		ev := RuntimeEvent{
			At:      time.Now(),
			Kind:    EventRoleLog,
			Role:    "test",
			Message: "log entry",
		}
		m.handleRuntimeEvent(ev)
	}

	if len(m.logs) != 10 {
		t.Errorf("expected logs to be truncated to 10, got %d", len(m.logs))
	}
}

// TestBoundary_ContentLinesTruncation 验证 contentLines 超过上限时正确截断
func TestBoundary_ContentLinesTruncation(t *testing.T) {
	m := NewModel(Options{Width: 120, Height: 40})
	m.maxContentLines = 50 // 降低上限便于测试

	// 生成超过上限的内容行
	largeContent := ""
	for i := 0; i < 60; i++ {
		largeContent += "Line content here\n"
	}

	ev := RuntimeEvent{
		At:        time.Now(),
		Kind:      EventContentDelta,
		ChapterID: "ch01",
		Delta:     largeContent,
	}
	m.handleRuntimeEvent(ev)

	if len(m.contentLines) != 50 {
		t.Errorf("expected contentLines to be truncated to 50, got %d", len(m.contentLines))
	}
}

// TestBoundary_UsageAllFieldsMissing 验证 usage 所有字段缺失时不崩溃
func TestBoundary_UsageAllFieldsMissing(t *testing.T) {
	m := NewModel(Options{Width: 120, Height: 40})

	// 所有字段都是 nil
	ev := RuntimeEvent{
		At:   time.Now(),
		Kind: EventUsageUpdate,
		Usage: &UsageSnapshot{
			InputTokens:      nil,
			OutputTokens:     nil,
			ContextTokens:    nil,
			MaxContextTokens: nil,
			CostUSD:          nil,
			Model:            "",
		},
	}

	// 不应该 panic
	m.handleRuntimeEvent(ev)

	// 验证默认值
	if m.totalTokens != 0 {
		t.Errorf("expected totalTokens to remain 0, got %d", m.totalTokens)
	}
	if m.totalCost != 0.0 {
		t.Errorf("expected totalCost to remain 0, got %f", m.totalCost)
	}
	if m.model != "--" {
		t.Errorf("expected model to remain '--', got %q", m.model)
	}
}

// TestBoundary_EmptyChaptersProgress 验证空 chapters 时进度显示 0.0%
func TestBoundary_EmptyChaptersProgress(t *testing.T) {
	m := NewModel(Options{Width: 120, Height: 40})

	m.calculateProgress()

	if m.totalProgress != 0.0 {
		t.Errorf("expected progress to be 0.0 with no chapters, got %.1f", m.totalProgress)
	}
}

// TestBoundary_NarrowTerminal 验证窄终端 (width < 60) 不崩溃
func TestBoundary_NarrowTerminal(t *testing.T) {
	m := NewModel(Options{Width: 50, Height: 30})

	// 添加一些数据
	ev := RuntimeEvent{
		At:      time.Now(),
		Kind:    EventRoleLog,
		Role:    "test",
		Message: "test log",
	}
	m.handleRuntimeEvent(ev)

	// View() 不应该 panic
	view := m.View()
	if view == "" {
		t.Error("expected non-empty view for narrow terminal")
	}
}

// TestBoundary_EmptyDelta 验证空 Delta 不被追加
func TestBoundary_EmptyDelta(t *testing.T) {
	m := NewModel(Options{Width: 120, Height: 40})

	ev := RuntimeEvent{
		At:        time.Now(),
		Kind:      EventContentDelta,
		ChapterID: "ch01",
		Delta:     "", // 空 delta
	}

	m.handleRuntimeEvent(ev)

	if m.contentBuffer != "" {
		t.Errorf("expected empty contentBuffer, got %q", m.contentBuffer)
	}
	if len(m.contentLines) != 0 {
		t.Errorf("expected 0 content lines, got %d", len(m.contentLines))
	}
}

// TestBoundary_ChapterStatusMissingFields 验证章节状态缺失字段时不崩溃
func TestBoundary_ChapterStatusMissingFields(t *testing.T) {
	m := NewModel(Options{Width: 120, Height: 40})

	// Fields 为 nil
	ev1 := RuntimeEvent{
		At:        time.Now(),
		Kind:      EventChapterStatus,
		ChapterID: "ch01",
		Fields:    nil,
	}
	m.handleRuntimeEvent(ev1)

	// Fields 为空 map
	ev2 := RuntimeEvent{
		At:        time.Now(),
		Kind:      EventChapterStatus,
		ChapterID: "ch02",
		Fields:    map[string]any{},
	}
	m.handleRuntimeEvent(ev2)

	// 不应该 panic，章节应该被创建
	if len(m.chapters) != 2 {
		t.Errorf("expected 2 chapters, got %d", len(m.chapters))
	}
}

// TestBoundary_ChapterStatusInvalidTypes 验证章节状态字段类型不匹配时处理
func TestBoundary_ChapterStatusInvalidTypes(t *testing.T) {
	m := NewModel(Options{Width: 120, Height: 40})

	ev := RuntimeEvent{
		At:        time.Now(),
		Kind:      EventChapterStatus,
		ChapterID: "ch01",
		Fields: map[string]any{
			"status":         123,         // 应该是 string
			"draft_version":  "invalid",   // 应该是 int
			"score":          "invalid",   // 应该是 int
			"citation_score": []int{1, 2}, // 应该是 int
			"word_count":     nil,         // 应该是 int
		},
	}

	// 不应该 panic
	m.handleRuntimeEvent(ev)

	state, exists := m.chapters["ch01"]
	if !exists {
		t.Fatal("expected chapter ch01 to exist")
	}

	// 验证默认值或安全处理
	if state.Status != ChapterPending {
		t.Logf("status defaulted to: %q", state.Status)
	}
	if state.Score != nil {
		t.Logf("score: %d", *state.Score)
	}
}
