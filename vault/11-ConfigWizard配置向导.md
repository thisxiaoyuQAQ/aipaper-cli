# 11-ConfigWizard配置向导

## 模块概述

实现首次运行的 Provider 配置向导，使用模板生成现有 `config.Config`，默认保存到项目级 `aipaper.json`，并保证 API Key 在 UI、日志和错误中脱敏。

## 前置依赖

- 依赖模块：01-基础脚手架与配置Store、10-无参数TUI启动与RootModel骨架
- 可并行模块：12-启动状态探测与恢复入口（状态探测框架可并行，配置校验需等本模块）

## 最小上下文清单

- 项目备忘录.md
- docs/TUI全流程增量需求.md
- docs/interfaces/config.md
- docs/interfaces/tui.md
- internal/config/config.go
- internal/app/agent_runtime.go
- internal/cli/cli.go（参考配置脱敏）

## 接口与类型定义

从 `docs/interfaces/config.md` 摘录：

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

type ProviderConfig struct {
    Type    string         `json:"type,omitempty"`
    APIKey  string         `json:"api_key,omitempty"`
    BaseURL string         `json:"base_url,omitempty"`
    Models  []string       `json:"models,omitempty"`
    Extra   map[string]any `json:"extra,omitempty"`
}

type RoleConfig struct {
    Provider string  `json:"provider,omitempty"`
    Model    string  `json:"model,omitempty"`
    MaxTurns int     `json:"max_turns,omitempty"`
    Temp     float64 `json:"temperature,omitempty"`
}
```

从 `docs/interfaces/tui.md` 摘录模板契约：

| Name | Type | BaseURL | Model | API Key |
|---|---|---|---|---|
| OpenAI | `openai` | `https://api.openai.com/v1` | `gpt-5.5` | 必填 |
| Anthropic | `anthropic` | `https://api.anthropic.com` | `claude-opus-4-8` | 必填 |
| Ollama | `ollama` | `http://localhost:11434` | `llama3` | 可空 |
| Custom | 用户填写 | 用户填写 | 用户填写 | 按用户选择 |

## 实现要求

- 新增 `internal/tui/configwizard` 包，提供 model/view/update 测试层。
- 三步流程：选择模板 → 填写必要字段 → 摘要确认并保存。
- 默认 provider key 使用 `default`；Custom 可让用户填写 provider name。
- 生成 roles 至少覆盖 `coordinator`、`architect`、`writer`、`editor`，均指向默认 provider/model，保证 runtime role 解析可用。
- 默认 `default_language=zh-CN`、`citation_style=gbt7714`，除非用户修改。
- 新增或复用配置保存函数，项目 TUI 默认写 `./aipaper.json`。
- 新增公共脱敏函数（如 `config.MaskSecret` / `config.Redact`），避免 TUI 复制 `internal/cli` 私有逻辑。
- API Key 保存优先使用环境变量引用（如 `env:OPENAI_API_KEY` / `env:ANTHROPIC_API_KEY`）；若用户选择直接写明文到项目配置，必须提示风险并只在摘要中显示脱敏值。
- Anthropic/Claude 默认不主动写入 temperature/top_p/top_k 等采样参数。
- 保存失败时停留当前屏，允许重试、返回修改或退出。

## 测试要求

- OpenAI / Anthropic / Ollama / Custom 模板默认值正确。
- 保存后的 `aipaper.json` 可被 `config.Load` 读取，`cfg.Validate()` 通过。
- `app.ResolveRoleRuntime(cfg, "coordinator")` 和 `writer` 可解析。
- API Key 摘要、错误、日志中不出现完整 key。
- Ollama 允许空 API key。

## 任务清单（预期产出）

- `internal/tui/configwizard/model.go`
- `internal/tui/configwizard/view.go`
- `internal/tui/configwizard/model_test.go`
- `internal/config` 公共保存 / 脱敏 helper（若尚不存在）
- RootModel 接入 ConfigWizard 首屏与保存成功转场

## 模块代码行数预估

- model/view/test 分离；单文件目标 < 500 行。
