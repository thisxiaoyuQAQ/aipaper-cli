# TUI 增量接口与事件契约

> 本文件记录下一阶段 TUI 全流程新增的屏幕、转场、配置向导和 RuntimeEvent 契约。权威实现预计位于 `internal/tui/app`、`internal/tui/configwizard`、`internal/tui/writing`。

## 1. Screen 枚举

```go
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
```

## 2. 屏幕转场消息

```go
type ScreenTransitionMsg struct {
    Next Screen
    Data any
}
```

约定：

- 子屏幕通过 transition 表达完成、返回、重试、退出等意图。
- RootModel 负责读取/写入全局状态并初始化下一屏。
- 子屏幕不得直接清空 Store 或绕过 RootModel 切换流程。

## 3. StateProbe 结果

```go
type ProbeResult struct {
    WorkDir          string
    ConfigOK         bool
    ConfigError      error
    HasRequirements  bool
    HasMaterials     bool
    HasCandidates    bool
    HasConfirmedRefs bool
    HasAcceptedWork  bool
    HasFinalOutputs  bool
    CheckpointValid  bool
    CheckpointStep   int
    SuggestedScreen  Screen
}
```

`SuggestedScreen` 按配置、checkpoint、requirements、materials、candidates、confirmed refs、writing artifacts、final outputs 顺序判断。

## 4. ConfigWizard 模板

```go
type ProviderTemplate struct {
    Name      string
    Type      string
    BaseURL   string
    Model     string
    APIKeyEnv string
    APIKeyRequired bool
}
```

默认模板：

| Name | Type | BaseURL | Model | API Key |
|---|---|---|---|---|
| OpenAI | `openai` | `https://api.openai.com/v1` | `gpt-5.5` | 必填 |
| Anthropic | `anthropic` | `https://api.anthropic.com` | `claude-opus-4-8` | 必填 |
| Ollama | `ollama` | `http://localhost:11434` | `llama3` | 可空 |
| Custom | 用户填写 | 用户填写 | 用户填写 | 按用户选择 |

保存后生成现有配置结构。API Key 保存策略：优先保存 `env:OPENAI_API_KEY`、`env:ANTHROPIC_API_KEY`、`env:CUSTOM_LLM_API_KEY` 等环境变量引用；若用户选择直接写入项目 `aipaper.json`，必须在 UI 中提示风险，并确保所有摘要、日志、事件和 report 只显示脱敏值。

Anthropic 模板默认 `claude-opus-4-8`，不应默认写入 `temperature`、`top_p`、`top_k` 等采样参数；后续 runtime 也不得因空配置而向 Claude 请求发送不兼容参数。

```go
type Config struct {
    Provider        string                    `json:"provider,omitempty"`
    Model           string                    `json:"model,omitempty"`
    DefaultLanguage string                    `json:"default_language,omitempty"`
    CitationStyle   string                    `json:"citation_style,omitempty"`
    Style           string                    `json:"style,omitempty"`
    Providers       map[string]ProviderConfig `json:"providers,omitempty"`
    Roles           map[string]RoleConfig     `json:"roles,omitempty"`
}
```

建议角色映射至少覆盖：`coordinator`、`architect`、`writer`、`editor`。

## 5. RuntimeEvent

TUI 消费内部事件，不直接消费第三方 agentcore/litellm 原始事件。

```go
type RuntimeEventKind string

const (
    EventStepStarted       RuntimeEventKind = "step_started"
    EventStepDone          RuntimeEventKind = "step_done"
    EventStepFailed        RuntimeEventKind = "step_failed"
    EventRoleLog           RuntimeEventKind = "role_log"
    EventContentDelta      RuntimeEventKind = "content_delta"
    EventUsageUpdate       RuntimeEventKind = "usage_update"
    EventChapterStatus     RuntimeEventKind = "chapter_status"
    EventQualityReview     RuntimeEventKind = "quality_review"
    EventCheckpointSaved   RuntimeEventKind = "checkpoint_saved"
    EventExportArtifact    RuntimeEventKind = "export_artifact"
    EventRuntimeDone       RuntimeEventKind = "runtime_done"
    EventRuntimeError      RuntimeEventKind = "runtime_error"
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

## 6. UsageSnapshot

```go
type UsageSnapshot struct {
    InputTokens         *int64   `json:"input_tokens,omitempty"`
    OutputTokens        *int64   `json:"output_tokens,omitempty"`
    CacheReadTokens     *int64   `json:"cache_read_tokens,omitempty"`
    CacheCreationTokens *int64   `json:"cache_creation_tokens,omitempty"`
    ContextTokens       *int64   `json:"context_tokens,omitempty"`
    MaxContextTokens    *int64   `json:"max_context_tokens,omitempty"`
    CostUSD             *float64 `json:"cost_usd,omitempty"`
    Model               string   `json:"model,omitempty"`
}
```

缺失字段在 UI 中显示 `--`，不得阻塞写作。

## 7. ChapterStatus

```go
type ChapterStatus string

const (
    ChapterPending     ChapterStatus = "pending"
    ChapterWriting     ChapterStatus = "writing"
    ChapterReviewing   ChapterStatus = "reviewing"
    ChapterRewriting   ChapterStatus = "rewriting"
    ChapterDone        ChapterStatus = "done"
    ChapterNeedsReview ChapterStatus = "needs_human_review"
)
```

右侧进度区展示章节状态、评分、重写次数、引用一致性。

## 8. 安全约定

- `RuntimeEvent.Fields` 不得包含完整 API key、Authorization header、provider raw request。
- ConfigWizard 摘要只显示脱敏 key，例如 `sk-...abcd`。
- 错误详情中若包含请求上下文，必须先 redact。

## 9. 与既有接口的关系

- `requirements.json` 仍使用 [requirements.md](./requirements.md)。
- `references/candidates.json` 和 `references/confirmed.json` 仍使用 [references.md](./references.md)。
- `progress.json` / `run.json` 仍使用 [common.md](./common.md)。
- checkpoint 恢复仍使用 [checkpoint.md](./checkpoint.md)。
- final outputs 仍使用 [export.md](./export.md)。
