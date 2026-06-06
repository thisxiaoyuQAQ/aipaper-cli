# Search 学术搜索契约

学术搜索的查询输入、数据源、标准化字段与去重策略。搜索结果标准化后即为候选文献（`ReferenceCandidate`，见 [references.md](./references.md)），落盘为 `references/candidates.json`。

来源：spec 第 7 节「学术搜索 / 搜索流程」；标准化字段对照 `internal/contracts/types.go` 的 `ReferenceCandidate`。

## 1. 搜索查询输入

Coordinator 根据 `requirements.json` 生成查询（spec 第 7 节「搜索流程」第 1 步）：

| 输入项 | 来源 | 说明 |
| --- | --- | --- |
| 主主题 | `requirements.topic` | 核心检索词 |
| 研究问题关键词 | `requirements.research_questions` | 派生检索词 |
| 时间范围 | `requirements.scope` | 年份过滤 |
| 中英文关键词 | 主题/语言派生 | 跨语种召回 |
| 排除词 | `requirements.constraints` | 过滤无关结果 |
| 启用数据源 | `requirements.search_providers` | 决定调用哪些源 |

查询仅在 `requirements.allow_online_search=true` 时执行。

## 2. 数据源

### 内置免费公开源（始终可用）

- `semantic_scholar`（Semantic Scholar）
- `crossref`（Crossref）
- `arxiv`（arXiv）
- `pubmed`（PubMed）

### 可选增强源（通过 `search_providers` 配置启用）

- `serpapi`（SerpAPI）
- `tavily`（Tavily）
- `exa`（Exa）
- `google_scholar`（Google Scholar 代理）
- 其他自定义搜索 provider

> 增强源稳定性不保证（spec 第 8 节边界：不承诺 Google Scholar 原生稳定抓取）。

## 3. 标准化字段

各数据源返回结果统一映射为下列字段（对应 `ReferenceCandidate`）：

| 标准字段 | JSON tag | 说明 |
| --- | --- | --- |
| title | `title` | 标题 |
| authors | `authors` | 作者列表 |
| year | `year` | 年份（`omitempty`） |
| doi | `doi` | DOI（`omitempty`） |
| url | `url` | URL（`omitempty`） |
| abstract | `abstract` | 摘要（`omitempty`） |
| venue | `venue` | 期刊/会议（`omitempty`） |
| source | `source` | 来源标识，如 `semantic_scholar` |
| citation_count | `citation_count` | 被引数（如有，`omitempty`） |
| relevance_score | `relevance_score` | 相关性评分（`omitempty`） |
| dedupe_group | `dedupe_group` | 去重分组键（`omitempty`） |
| status | `status` | 候选状态，初始 `pending` |

字段缺失（如来源未返回必需字段）归类搜索错误 `REFERENCE_SEARCH_FIELD_MISSING`。

## 4. 去重策略

优先级（spec 第 7 节第 4 步 / 第 5 节幂等）：

1. **DOI 优先**：DOI 相同视为同一文献，`dedupe_group = "doi:<doi>"`。
2. **URL 次之**：无 DOI 时按规范化 URL 合并，`dedupe_group = "url:<url>"`。
3. **title+author+year hash 兜底**：均缺失时，对 `title+firstAuthor+year` 计算 hash，`dedupe_group = "hash:<hash>"`。

合并后保留信息最完整的一条，并在候选项保留 `dedupe_group` 供 TUI 展示「去重合并信息」。

## 5. 输出与流程衔接

- 标准化 + 去重后生成 `references/candidates.json` 与人类可读的 `references/candidates.md`。
- 对应 checkpoint step `search_references`。
- 空结果归类文献错误 `REFERENCE_CANDIDATES_EMPTY`（不可重试，需用户处理）。
- 候选文献交 TUI 多选确认后写入 `references/confirmed.json`（见 [references.md](./references.md)）。
