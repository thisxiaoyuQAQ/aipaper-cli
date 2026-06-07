# References 文献契约

候选文献与确认文献的数据结构。候选来自材料解析与学术搜索，经 TUI 多选确认后成为写作唯一可用来源。

来源：`internal/contracts/types.go` 的 `ReferenceCandidates` / `ReferenceCandidate` / `ConfirmedReferences` / `ConfirmedReference`；reference key 规则见 spec 第 7 节。

## 1. 候选文献

```go
type ReferenceCandidates struct {
    Items []ReferenceCandidate `json:"items"`
}

type ReferenceCandidate struct {
    ID              string   `json:"id"`
    Title           string   `json:"title"`
    Authors         []string `json:"authors"`
    Year            int      `json:"year,omitempty"`
    Source          string   `json:"source"`
    DOI             string   `json:"doi,omitempty"`
    URL             string   `json:"url,omitempty"`
    Abstract        string   `json:"abstract,omitempty"`
    Venue           string   `json:"venue,omitempty"`
    CitationCount   int      `json:"citation_count,omitempty"`
    RelevanceScore  float64  `json:"relevance_score,omitempty"`
    RelevanceReason string   `json:"relevance_reason,omitempty"`
    DedupeGroup     string   `json:"dedupe_group,omitempty"`
    Status          string   `json:"status"`
}
```

| 字段 | JSON tag | 说明 |
| --- | --- | --- |
| ID | `id` | 候选 ID，如 `cand_001` |
| Title | `title` | 标题 |
| Authors | `authors` | 作者列表 |
| Year | `year` | 年份（`omitempty`） |
| Source | `source` | 来源，如 `semantic_scholar`、`bibtex` |
| DOI / URL | `doi` / `url` | 标识（`omitempty`） |
| Abstract | `abstract` | 摘要（`omitempty`） |
| Venue | `venue` | 期刊/会议（`omitempty`） |
| CitationCount | `citation_count` | 被引数（`omitempty`） |
| RelevanceScore | `relevance_score` | 相关性评分（`omitempty`） |
| RelevanceReason | `relevance_reason` | 相关性理由（`omitempty`） |
| DedupeGroup | `dedupe_group` | 去重分组（`omitempty`） |
| Status | `status` | `pending`/`confirmed`/`rejected` |

### candidates.json 示例

```json
{
  "items": [
    {
      "id": "cand_001",
      "title": "Generative AI in Educational Assessment",
      "authors": ["Zhang", "Li"],
      "year": 2024,
      "source": "semantic_scholar",
      "doi": "10.1000/abc",
      "url": "https://example.org/abc",
      "abstract": "...",
      "relevance_score": 0.86,
      "relevance_reason": "直接讨论生成式 AI 在评估中的有效性",
      "dedupe_group": "doi:10.1000/abc",
      "status": "pending"
    }
  ]
}
```

### BibTeX 候选入口

当前实现位于 `internal/references/bibtex.go`，供材料解析模块把用户提供的 `.bib` 文件转换为候选文献：

```go
type BibTeXEntry struct {
    Type   string            `json:"type"`
    Key    string            `json:"key"`
    Fields map[string]string `json:"fields"`
}

func ParseBibTeX(data []byte) ([]BibTeXEntry, error)
func CandidatesFromBibTeX(entries []BibTeXEntry, startIndex int) []contracts.ReferenceCandidate
```

转换规则：

- candidate ID 从 `startIndex` 开始，格式 `cand_001`。
- `source` 固定为 `bibtex`，`status` 固定为 `pending`。
- `title` / `author` / `year` / `doi` / `url` / `abstract` / `journal` / `booktitle` / `publisher` 从 BibTeX 字段映射。
- `dedupe_group` 复用统一去重规则：优先 DOI，其次 URL，最后回退到 title+firstAuthor+year hash。

### 去重与 ID 工具

当前实现位于 `internal/references/dedupe.go`，供搜索模块与材料模块共用：

```go
func DedupeCandidates(candidates []contracts.ReferenceCandidate) []contracts.ReferenceCandidate
func AssignCandidateIDs(candidates []contracts.ReferenceCandidate, startIndex int) []contracts.ReferenceCandidate
func CandidateDedupeGroup(candidate contracts.ReferenceCandidate) string
func NormalizeDOI(doi string) string
func NormalizeURL(raw string) string
```

`DedupeCandidates` 按 DOI、URL、title+firstAuthor+year hash 三层规则分组；同组内保留字段更完整的一条，并用较完整候选补齐缺失字段。

## 2. 确认文献

```go
type ConfirmedReferences struct {
    Items []ConfirmedReference `json:"items"`
}

type ConfirmedReference struct {
    Key               string    `json:"key"`
    Title             string    `json:"title"`
    Authors           []string  `json:"authors"`
    Year              int       `json:"year,omitempty"`
    DOI               string    `json:"doi,omitempty"`
    URL               string    `json:"url,omitempty"`
    Abstract          string    `json:"abstract,omitempty"`
    SourceMaterialIDs []string  `json:"source_material_ids"`
    ConfirmedAt       time.Time `json:"confirmed_at"`
}
```

| 字段 | JSON tag | 说明 |
| --- | --- | --- |
| Key | `key` | 稳定引用键（见下规则），全局唯一 |
| Title / Authors / Year | `title` / `authors` / `year` | 文献基本信息 |
| DOI / URL / Abstract | `doi` / `url` / `abstract` | `omitempty` |
| SourceMaterialIDs | `source_material_ids` | 关联的材料 ID（来自用户材料时填充） |
| ConfirmedAt | `confirmed_at` | 用户确认时间 |

### confirmed.json 示例

```json
{
  "items": [
    {
      "key": "zhang2024GenerativeAssessment",
      "title": "Generative AI in Educational Assessment",
      "authors": ["Zhang", "Li"],
      "year": 2024,
      "doi": "10.1000/abc",
      "url": "https://example.org/abc",
      "abstract": "...",
      "source_material_ids": [],
      "confirmed_at": "2026-06-05T10:00:00Z"
    }
  ]
}
```

### 确认与拒绝落盘入口

当前实现位于 `internal/references/confirm.go`：

```go
type ConfirmationDecision struct {
    ConfirmedIDs []string
    RejectedIDs  []string
    ConfirmedAt  time.Time
}

type ConfirmationResult struct {
    Confirmed contracts.ConfirmedReferences
    Rejected  contracts.ReferenceCandidates
    Outputs   []string
}

func LoadCandidates(s store.Store) (contracts.ReferenceCandidates, error)
func ConfirmCandidates(s store.Store, candidates contracts.ReferenceCandidates, decision ConfirmationDecision) (ConfirmationResult, error)
func ReferenceKey(candidate contracts.ReferenceCandidate) string
func UniqueReferenceKey(candidate contracts.ReferenceCandidate, used map[string]int) string
func FormatConfirmedBibTeX(confirmed contracts.ConfirmedReferences) string
```

输出文件：

- `references/confirmed.json`：`ConfirmedReferences`
- `references/rejected.json`：`ReferenceCandidates`，其中每个 rejected item 的 `status="rejected"`
- `references/confirmed.bib`：由 `ConfirmedReferences` 导出的 BibTeX

未确认任何文献时，`ConfirmCandidates` 返回 `REFERENCE_NONE_CONFIRMED`，供后续写作流程阻塞。

### TUI model 入口

当前实现位于 `internal/tui/references`，采用可测试的 model/view/update 分层，后续可由 Bubble Tea runtime 桥接：

```go
func NewModel(candidates contracts.ReferenceCandidates) Model
func (m Model) UpdateKey(key string) Model
func (m Model) View() string
func (m Model) Decision(now time.Time) references.ConfirmationDecision
```

已支持选择、取消、搜索、排序、全选高相关、拒绝、确认和退出状态。

## 3. reference key 生成规则

格式 `firstAuthorYearShortTitle`（spec 第 7 节）：

- `firstAuthor`：第一作者姓氏小写驼峰前缀，如 `zhang`。
- `Year`：四位年份，如 `2024`。
- `ShortTitle`：标题取首要实词拼成驼峰，如 `GenerativeAssessment`。
- 组合示例：`zhang2024GenerativeAssessment`。

### 冲突处理

key 重复时按字母后缀去重：`zhang2024GenerativeAssessmentA`、`zhang2024GenerativeAssessmentB`……保证全局唯一。

## 4. 硬规则（spec 第 7 节质量规则）

- 仅 `confirmed.json` 中的文献可用于正文引用；候选未确认不可引用。
- DOI 不存在可用 URL，但须标记来源可信度；摘要缺失可确认但可信度较低。
- 普通网页不能伪装成论文引用；用户材料笔记不能自动冒充学术文献。
- 确认文献同时写出 `references/confirmed.bib`；被拒写入 `references/rejected.json`。
- 用户未确认任何文献 → 错误 `REFERENCE_NONE_CONFIRMED`，不进入写作。
- 对应 checkpoint step `confirm_references`。
