# 配置契约

本文件记录 `aipaper-cli` 配置文件结构、加载顺序和合并规则。字段以 `internal/config/config.go` 为权威来源。

## 配置文件位置

加载顺序从低到高：

1. 全局配置：`~/.aipaper/config.json`
2. 项目配置：`./aipaper.json`
3. 显式配置：`--config FILE`

后加载的配置覆盖前面的配置。`providers` 和 `roles` 按 key 合并，同名 key 内部再按字段覆盖。

## Config

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

| 字段 | 说明 |
|---|---|
| `provider` | 默认 LLM provider key，需能在 `providers` 中找到 |
| `model` | 默认模型 ID，不自动改写 |
| `default_language` | 默认写作语言，如 `zh-CN` / `en` |
| `citation_style` | 默认引用格式，如 `gbt7714` / `apa` |
| `style` | 预留写作风格配置 |
| `providers` | provider 配置表 |
| `roles` | Coordinator / Architect / Writer / Editor 等角色级覆盖 |

## ProviderConfig

```go
type ProviderConfig struct {
    Type    string         `json:"type,omitempty"`
    APIKey  string         `json:"api_key,omitempty"`
    BaseURL string         `json:"base_url,omitempty"`
    Models  []string       `json:"models,omitempty"`
    Extra   map[string]any `json:"extra,omitempty"`
}
```

`ollama`、`bedrock` 或显式无鉴权自定义代理允许 `api_key` 为空。其他 provider 的 key 校验可在后续 provider 初始化阶段做。

## RoleConfig

```go
type RoleConfig struct {
    Provider string  `json:"provider,omitempty"`
    Model    string  `json:"model,omitempty"`
    MaxTurns int     `json:"max_turns,omitempty"`
    Temp     float64 `json:"temperature,omitempty"`
}
```

角色配置用于给 Coordinator、Architect、Writer、Editor 指定不同 provider/model 或推理参数。

## 配置示例

```json
{
  "provider": "openrouter",
  "model": "google/gemini-2.5-pro",
  "default_language": "zh-CN",
  "citation_style": "gbt7714",
  "providers": {
    "openrouter": {
      "type": "openrouter",
      "api_key": "env:OPENROUTER_API_KEY",
      "base_url": "https://openrouter.ai/api/v1"
    }
  },
  "roles": {
    "editor": {
      "provider": "openrouter",
      "model": "anthropic/claude-sonnet-4",
      "max_turns": 4,
      "temperature": 0.2
    }
  }
}
```

## 校验规则

- `provider` 和 `model` 必须成对出现。
- 配置了 `providers` 时，`provider` 必须存在于 `providers` map 中。
- `config` CLI 输出必须 redact 非空 `api_key`。
- 配置读取错误和 JSON 解析错误直接返回，不静默忽略。
