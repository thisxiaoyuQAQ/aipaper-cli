# Requirements 写作需求契约

TUI 表单收集的结构化写作需求，落盘为 `output/aipaper/requirements.json`。

来源：`internal/contracts/types.go` 的 `Requirements`；表单 model 位于 `internal/tui/requirements`；示例与字段语义见 spec 第 4 节。

## 结构定义

```go
type Requirements struct {
    Topic              string   `json:"topic"`
    ResearchQuestions  []string `json:"research_questions"`
    Scope              string   `json:"scope"`
    Language           string   `json:"language"`
    CitationStyle      string   `json:"citation_style"`
    TargetWords        int      `json:"target_words"`
    MaterialDir        string   `json:"material_dir"`
    AllowOnlineSearch  bool     `json:"allow_online_search"`
    SearchProviders    []string `json:"search_providers,omitempty"`
    ChapterPreferences []string `json:"chapter_preferences"`
    Constraints        []string `json:"constraints"`
}
```

## 字段说明

| 字段 | JSON tag | 类型 | 说明 |
| --- | --- | --- | --- |
| Topic | `topic` | string | 论文主题，必填非空 |
| ResearchQuestions | `research_questions` | []string | 研究问题列表 |
| Scope | `scope` | string | 综述范围（时间/语种/领域） |
| Language | `language` | string | 目标语言，`zh-CN` 或 `en` |
| CitationStyle | `citation_style` | string | 引用格式，中文默认 `gbt7714`，英文默认 `apa` |
| TargetWords | `target_words` | int | 目标字数，> 0 |
| MaterialDir | `material_dir` | string | 材料目录路径 |
| AllowOnlineSearch | `allow_online_search` | bool | 是否允许联网学术搜索 |
| SearchProviders | `search_providers` | []string | 启用的搜索增强源（可空，`omitempty`） |
| ChapterPreferences | `chapter_preferences` | []string | 章节偏好 |
| Constraints | `constraints` | []string | 特殊要求/约束 |

> 注意：`search_providers` 带 `omitempty`，为空时不序列化；其余切片字段无 `omitempty`，应序列化为 `[]`。

## requirements.json 示例

```json
{
  "topic": "生成式 AI 在教育评估中的应用综述",
  "research_questions": [
    "生成式 AI 在自动评分中的有效性如何？",
    "其在教育评估中的公平性风险有哪些？"
  ],
  "scope": "2020-2026 年英文与中文核心研究",
  "language": "zh-CN",
  "citation_style": "gbt7714",
  "target_words": 8000,
  "material_dir": "./materials",
  "allow_online_search": true,
  "search_providers": ["semantic_scholar", "crossref"],
  "chapter_preferences": [],
  "constraints": []
}
```

## 校验规则

属于「不可重试」错误类，缺字段需用户在 TUI 修正（错误码族 `WRITER_CHAPTER_CONTRACT_MISSING` 之前的需求校验）：

1. `topic` 非空。
2. `language` ∈ {`zh-CN`, `en`}；若空，按配置 `default_language` 兜底。
3. `citation_style` 合法；为空时按语言推断（zh→`gbt7714`，en→`apa`）。
4. `target_words` > 0。
5. `material_dir` 非空且目录可访问（材料目录不存在归类为材料错误）。
6. 当 `allow_online_search=false` 时，`search_providers` 应被忽略；当为 `true` 且指定增强源时，源名须在支持列表内（见 [search.md](./search.md)）。
7. `research_questions` 建议非空，便于 Coordinator 生成搜索查询；为空不阻塞但会降低搜索召回。

校验通过后写入 `requirements.json`，对应 checkpoint step `collect_requirements`。

## TUI model 入口

当前实现采用可测试的 model/view/update 分层，后续可由 Bubble Tea runtime 桥接：

```go
func NewModel(defaults contracts.Requirements) Model
func (m Model) UpdateKey(key string) Model
func (m Model) SetField(field Field, value string) Model
func (m Model) Requirements() (contracts.Requirements, error)
func Validate(req contracts.Requirements) error
```

- `UpdateKey("tab")` / `UpdateKey("shift+tab")` 在字段间移动。
- `UpdateKey("space")` 在 `allow_online_search` 字段切换布尔值；其他字段输入空格。
- `UpdateKey("enter")` 校验并提交；失败时 `Model.Err()` 返回错误。
- `Requirements()` 会归一化默认语言、引用格式、布尔值、逗号分隔搜索源和多行列表。
