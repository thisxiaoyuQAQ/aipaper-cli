# TUI i18n 与默认中文界面设计

日期：2026-06-14

## 背景

当前项目的 TUI 文案混合中英文。配置中已有 `default_language`，但它表达的是论文/生成内容的默认语言，不适合同时控制界面语言。本设计目标是让程序默认使用中文界面，同时允许用户在配置文件中改成英文界面。

## 目标

1. 默认启动 TUI 时，用户可见界面文案使用中文。
2. 在 `aipaper.json` 中新增 `ui_language`，用于单独控制界面语言。
3. `ui_language` 支持 `zh-CN` 与 `en`。
4. `default_language` 继续只控制论文/生成内容语言，不影响界面语言。
5. 旧配置文件不含 `ui_language` 时仍可正常运行，并默认中文。
6. TUI 中由程序生成的可见文案都中文化；英文作为可选翻译保留。

## 非目标

1. 不做外部 JSON/YAML 语言包。
2. 不做运行时热切换语言；配置变更在下次启动或重新进入相关流程时生效。
3. 不引入 ICU、复数规则等复杂国际化框架。
4. 不翻译用户输入、模型生成正文、文件路径、模型名、provider 名、配置 key、内部 ID。
5. 不强行翻译底层 error message，只翻译外层提示前缀。

## 配置设计

在 `internal/config.Config` 中新增：

```go
UILanguage string `json:"ui_language,omitempty"`
```

配置示例：

```json
{
  "provider": "default",
  "model": "gpt-5.5",
  "default_language": "zh-CN",
  "ui_language": "zh-CN",
  "citation_style": "gbt7714"
}
```

语义：

- `default_language`：论文/生成内容语言。
- `ui_language`：TUI/CLI 界面语言。

语言规范化规则：

| 输入 | 规范化结果 |
|---|---|
| 空值 | `zh-CN` |
| `zh-CN` | `zh-CN` |
| `zh` / `cn` / `chinese` | `zh-CN` |
| `en` / `en-US` / `english` | `en` |
| 其他非法值 | `zh-CN` |

非法值温和回退中文，不中断程序。

## i18n 包设计

新增轻量包：

```text
internal/i18n
```

核心类型：

```go
type Language string

const (
    ZhCN Language = "zh-CN"
    En   Language = "en"
)

type Key string

type T struct {
    lang Language
}

func New(lang string) T
func NormalizeLanguage(lang string) Language
func (t T) Lang() Language
func (t T) IsZero() bool
func (t T) Text(key Key) string
func (t T) Format(key Key, args ...any) string
```

翻译表首版放在 Go 文件中：

```go
var zhCN = map[Key]string{...}
var en = map[Key]string{...}
```

查找顺序：

1. 当前语言翻译。
2. 中文翻译。
3. key 字符串。

这样缺翻译不会导致空 UI，默认也始终偏中文。

## TUI 接入设计

Root 层负责决定界面语言：

```text
aipaper.json
  ↓ config.Load
config.Config.UILanguage
  ↓ i18n.New(cfg.UILanguage)
i18n.T
  ↓ RootModel 持有
各 TUI screen model
  ↓ View / Update / 错误提示 / 日志
中文或英文界面
```

`RootModel` 增加：

```go
i18n i18n.T
```

`NewRootModel` 行为：

1. 尝试读取配置。
2. 若读取到 `ui_language`，使用该值创建 translator。
3. 若配置不存在、字段为空或读取失败，默认 `zh-CN`。
4. 配置向导保存配置后，RootModel 更新 translator 或重新读取配置。

各 TUI model 的 `Options` 增加：

```go
I18N i18n.T
```

各 model 保存：

```go
i18n i18n.T
```

`NewModel` 中如果没传 translator，则默认中文：

```go
if opts.I18N.IsZero() {
    opts.I18N = i18n.New("zh-CN")
}
```

## 中文化范围

首版覆盖所有 TUI 屏幕的用户可见静态文案与程序生成提示。

### 配置向导

- `Provider template` → `模型服务模板`
- `Provider settings` → `模型服务配置`
- `Configuration summary` → `配置摘要`
- `Provider name` → `服务名称`
- `Provider type` → `服务类型`
- `Base URL` → `基础地址`
- `Model` → `模型`
- `API key` → `API 密钥`
- `Default language` → `默认写作语言`
- 新增 `UI language` → `界面语言`
- 保存失败、未知模板、配置不完整等提示中文化。

配置向导默认写入：

```json
"ui_language": "zh-CN"
```

### 需求填写页

字段名、帮助说明、校验错误、底部快捷键提示中文化。`default_language` 的值仍保留 `zh-CN` / `en` 等配置语义，不强制显示为“中文/英文”。

### 材料扫描页

状态、统计项、错误提示、继续/重试/返回提示中文化。

### 搜索页

搜索中、搜索完成、全部失败、禁用搜索、provider 错误提示中文化。

### 文献确认页

- `Reference candidates` → `文献候选`
- `Search` → `搜索`
- `No matching candidates` → `没有匹配的候选文献`
- 选择、拒绝、确认、返回等提示中文化。

### 写作进度页

左侧指标：

- `Status` → `状态`
- `Phase` → `阶段`
- `Progress` → `进度`
- `Focus` → `焦点`
- `Pending` → `待处理指令`
- `Words` → `字数`
- `Model` → `模型`
- `Context` → `上下文`
- `Tokens` → `Token`
- `Cost` → `费用`
- `Elapsed` → `耗时`

面板标题：

- `Metrics` → `指标`
- `Logs` → `日志`
- `Content` → `正文`
- `Chapter Progress` → `章节进度`

底部快捷键：

- `Esc: Pause at checkpoint` → `Esc：在最近 checkpoint 暂停`
- `Enter: Submit/Resume` → `Enter：提交指令/继续`
- `Ctrl+C: Exit` → `Ctrl+C：退出`
- `Space: Logs auto-scroll` → `Space：切换日志自动滚动`

状态：

- `Running` → `运行中`
- `Paused` → `已暂停`
- `Done` → `已完成`
- `Error` → `错误`

程序生成日志：

- `Started: writer_run` → `开始：writer_run`
- `Completed: writer_run` → `完成：writer_run`
- `Failed: writer_run` → `失败：writer_run`
- `Writing completed successfully` → `写作已完成`
- `Checkpoint saved` → `已保存 checkpoint`

### 导出总结页

输出目录、生成文件、质量报告、降级说明、重试/继续提示中文化。

### 完成页

当前大部分已是中文，统一接入 i18n，以便 `ui_language=en` 时切换英文。

### 退出确认

写作中退出确认、保存进度说明、确认键提示中文化。

## 不翻译内容

首版保留原样：

- 文件路径，如 `output/aipaper/final/paper.md`
- provider 名称，如 `OpenAI`、`Anthropic`、`Ollama`
- 模型名，如 `gpt-5.5`、`claude-opus-4-8`
- 配置 key，如 `ui_language`、`default_language`
- 内部 ID，如 `ch01`、`writer_run`、`quality-report.md`
- 用户输入
- 模型返回正文
- 底层 error message 的技术细节

错误展示只翻译外层前缀，例如：

```text
保存配置失败：permission denied
```

英文界面：

```text
Failed to save configuration: permission denied
```

## 错误处理

- `ui_language` 为空：默认中文。
- `ui_language` 非法：默认中文，不中断。
- 翻译 key 缺失：回退中文；中文也缺失则返回 key 字符串。
- 配置读取失败：RootModel 默认中文，同时保留原有错误显示逻辑。
- 动态错误消息：底层内容不翻译，只翻译前缀。

## 测试设计

### `internal/i18n`

- `NormalizeLanguage("") == zh-CN`
- `NormalizeLanguage("zh") == zh-CN`
- `NormalizeLanguage("cn") == zh-CN`
- `NormalizeLanguage("chinese") == zh-CN`
- `NormalizeLanguage("en-US") == en`
- `NormalizeLanguage("english") == en`
- 非法值回退 `zh-CN`
- `Text()` 当前语言缺失时回退中文
- `Format()` 正确格式化动态参数

### 配置测试

- 旧配置没有 `ui_language` 时不报错。
- 新配置含 `ui_language=en` 时能读出。
- 配置向导默认生成 `ui_language=zh-CN`。
- `default_language=en` 不影响 `ui_language=zh-CN`。

### TUI view 测试

不做全量 snapshot，只测关键锚点，降低脆弱性：

- 配置向导默认 view 包含 `模型服务模板`。
- 写作页默认 view 包含 `指标`、`日志`、`章节进度`。
- 写作页默认快捷键包含 `暂停`、`退出`。
- 文献确认页默认包含 `文献候选`。
- 导出页默认包含 `输出目录`。
- `ui_language=en` 时至少验证一个或两个屏幕显示英文锚点，例如 `Metrics`。

### 回归命令

```bash
go test ./internal/config ./internal/i18n ./internal/tui/...
```

如果实现中改到 CLI 文案，再额外运行：

```bash
go test ./internal/cli ./cmd/aipaper-cli
```

## 验收标准

1. 默认启动 TUI，用户可见界面文案为中文。
2. `aipaper.json` 设置 `"ui_language": "en"` 后，TUI 主要界面显示英文。
3. `default_language` 与 `ui_language` 互不影响。
4. 旧配置文件兼容。
5. 相关测试通过。
