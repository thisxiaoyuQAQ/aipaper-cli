# 17-WritingProgress四区布局与Runtime事件桥

## 模块概述

实现写作进度屏：通过内部 RuntimeEvent 桥接真实 LLM runtime 事件，以四区布局展示指标、日志、流式正文和章节进度，并在完成后进入 ExportSummary。

## 前置依赖

- 依赖模块：05-Agent运行时与Coordinator、06-写作产物与质量门控、07-StepCheckpoint恢复、16-References屏幕桥接与确认落盘
- 可并行模块：18-暂停退出与安全恢复（需共享 runtime 停止接口设计）

## 最小上下文清单

- 项目备忘录.md
- docs/TUI全流程增量需求.md
- docs/TUI全流程增量架构设计.md
- docs/interfaces/tui.md
- docs/interfaces/agent.md
- docs/interfaces/artifacts.md
- internal/app/agent_runtime.go
- internal/agent/events.go
- internal/artifacts/

## 接口与类型定义

从 `docs/interfaces/tui.md` 摘录：

```go
type RuntimeEventKind string

const (
    EventStepStarted     RuntimeEventKind = "step_started"
    EventStepDone        RuntimeEventKind = "step_done"
    EventStepFailed      RuntimeEventKind = "step_failed"
    EventRoleLog         RuntimeEventKind = "role_log"
    EventContentDelta    RuntimeEventKind = "content_delta"
    EventUsageUpdate     RuntimeEventKind = "usage_update"
    EventChapterStatus   RuntimeEventKind = "chapter_status"
    EventQualityReview   RuntimeEventKind = "quality_review"
    EventCheckpointSaved RuntimeEventKind = "checkpoint_saved"
    EventExportArtifact  RuntimeEventKind = "export_artifact"
    EventRuntimeDone     RuntimeEventKind = "runtime_done"
    EventRuntimeError    RuntimeEventKind = "runtime_error"
)

type RuntimeEvent struct {
    At        time.Time        `json:"at"`
    Kind      RuntimeEventKind `json:"kind"`
    Role      string           `json:"role,omitempty"`
    Step      string           `json:"step,omitempty"`
    ChapterID string           `json:"chapter_id,omitempty"`
    Message   string           `json:"message,omitempty"`
    Delta     string           `json:"delta,omitempty"`
    Usage     *UsageSnapshot   `json:"usage,omitempty"`
    Fields    map[string]any   `json:"fields,omitempty"`
}
```

## 实现要求

- 新增 `internal/tui/writing` 包，包含 event state、model、view、layout。
- 新增 runtime event bridge，TUI 不直接依赖 agentcore/litellm 原始事件。
- 四区布局：
  - 左侧指标：运行态、阶段、完成比例、字数、模型、context、tokens、cost、elapsed。
  - 中上日志：role log / step event，默认自动滚动。
  - 中下正文：`EventContentDelta` 实时追加到当前章节。
  - 右侧进度：章节状态、评分、重写次数、总进度条、引用一致性。
- usage 缺失字段显示 `--`，不阻塞。
- 小终端降级为纵向堆叠，不 panic。
- runtime 完成后转场 ExportSummary；错误时展示可恢复路径。
- Host/TUI 不硬编码 Architect→Writer→Editor 业务顺序，仍由 Coordinator 决策；不得复制 artifacts 或质量门控规则，只消费 runtime/artifacts 事件。
- 注意当前默认 WriterRunner 可能未实现真实写作；本模块必须把真实 Architect/Writer/Editor runner 接入 Host 可启动路径作为验收项，mock runtime 仅用于测试，不可替代真实路径。若真实路径无法完成，需明确标阻塞并拆出独立 vault。

## 测试要求

- mock RuntimeEvent 序列驱动 UI 状态变化。
- streaming delta 追加到当前章节；章节切换后内容归属正确。
- role log、usage、chapter status、quality review、checkpoint saved 可展示。
- usage nil 时 token/cost 为 `--`。
- 小宽度布局不崩溃。
- API key 不进入事件 fields 或 view。

## 任务清单（预期产出）

- `internal/tui/writing/events.go`
- `internal/tui/writing/model.go`
- `internal/tui/writing/view.go`
- `internal/tui/writing/model_test.go`
- runtime-to-TUI event bridge
- RootModel 接入 WritingProgress

## 模块代码行数预估

- events/model/view/layout 分文件；布局复杂时继续拆 helper，单文件目标 < 500 行。
