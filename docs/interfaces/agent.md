# Agent 协作契约

本文件记录 Coordinator、Architect、Writer、Editor 的职责边界、输入输出和质量门控规则。来源为已批准设计稿第 3 节与 `internal/contracts/types.go`。

## 总原则

- Host 只负责配置、模型、Agent 启动、工具注册、事件投影、恢复 prompt 和退出处理。
- Coordinator 是唯一流程决策者。
- Tools 只返回事实 JSON，不夹带调度建议。
- Architect、Writer、Editor 通过 Store 中的 artifact 协作，不共享长对话上下文。
- Writer 和 Editor 只能使用 `references/confirmed.json` 中的文献。

## Coordinator

Coordinator 维护写作项目状态：

- 当前阶段和 step
- 已确认文献
- 论文大纲
- 每章合同
- 每章草稿状态
- Editor 评审结果
- 当前重写轮次
- 待导出产物

Coordinator 可调用工具：

| 工具组 | 说明 |
|---|---|
| `requirements` | 读写结构化需求 |
| `materials` | 解析材料并写 manifest |
| `search` | 学术搜索和候选文献标准化 |
| `references` | 候选、确认、拒绝文献和引用 key |
| `artifacts` | 大纲、草稿、claims、review 读写 |
| `checkpoint` | step 记录和恢复事实读取 |
| `export` | Markdown、Docx、报告导出 |

## 运行时接口（05 已实现）

`internal/app.NewAgentRuntime` 负责创建 Coordinator runtime：

- 输入：`config.Config`、工作目录、可选恢复 prompt、可选测试用 mock model、可选 Writer runner。
- 输出：`AgentRuntime`，包含 `*agentcore.Agent`、实际 provider/model、system prompt、工具列表和 Store。
- 模型：通过 `github.com/voocel/agentcore/llm` 创建，底层 provider 配置使用 `github.com/voocel/litellm`。
- role 配置：优先使用 `roles.coordinator.provider/model/max_turns/temperature`，否则回落到顶层 `provider/model`。
- secret：`api_key` 支持 `env:NAME`，解析后只传给模型，不写入事件。

已注册的第一版事实工具：

| 工具名 | 说明 |
|---|---|
| `requirements_read` | 读取 `requirements.json` |
| `progress_read` | 读取 `progress.json` |
| `references_confirmed_read` | 返回 confirmed references 数量和 key 列表；缺文件视为 0 |
| `checkpoint_validate_latest` | 返回 latest checkpoint 校验事实 |
| `writer_run` | Writer 边界工具；无 confirmed references 时拒绝调用 Writer runner |

工具统一返回：

```json
{
  "ok": true,
  "data": {}
}
```

失败时：

```json
{
  "ok": false,
  "error": {
    "code": "AGENT_INVALID_JSON",
    "message": "error details",
    "retryable": false
  }
}
```

当前错误码：

| code | 说明 |
|---|---|
| `AGENT_INVALID_JSON` | Coordinator 需要结构化输出时返回了非法 JSON、未知字段或缺少 action |
| `REFERENCE_NONE_CONFIRMED` | Writer 在无 confirmed references 时被调用 |
| `AGENT_TOOL_FAILED` | 工具读取 Store 或下游 runner 失败 |

事件投影：

- `internal/agent.ProjectEvent` 将 `agentcore.Event` 转为 `contracts.RunEvent`。
- 投影保留 agent、tool、progress summary、错误和运行摘要。
- 不复制工具参数和结果，避免 API key 或长正文进入日志。

## Architect 输出

Architect 生成：

- `outline/outline.json`
- `outline/outline.md`

内容包含标题建议、摘要目标、章节列表、每章写作目标、关键问题、应引用文献候选、章节逻辑关系、字数分配、Writer 需要避免的重复点。

## Writer 输出

Writer 每次只写一个章节或小节，输出：

- `draft-vN.md`
- `claims-vN.json`
- `citation-map-vN.json`
- `writer_notes.md`

`claims-vN.json` 使用 `ClaimsFile`：

```go
type ClaimsFile struct {
    ChapterID    string  `json:"chapter_id"`
    DraftVersion int     `json:"draft_version"`
    Claims       []Claim `json:"claims"`
}
```

`citation-map-vN.json` 使用 `CitationMap`：

```go
type CitationMap struct {
    ChapterID    string            `json:"chapter_id"`
    DraftVersion int               `json:"draft_version"`
    Mappings     []CitationMapping `json:"mappings"`
}
```

## Editor 输出

Editor 输出：

- `review-vN.json`
- `review-vN.md`

`review-vN.json` 使用 `Review`：

```go
type Review struct {
    ChapterID           string               `json:"chapter_id"`
    DraftVersion        int                  `json:"draft_version"`
    Scores              ReviewScores         `json:"scores"`
    Passed              bool                 `json:"passed"`
    UnsupportedClaims   []string             `json:"unsupported_claims"`
    RequiredFixes       []string             `json:"required_fixes"`
    OptionalFixes       []string             `json:"optional_fixes"`
    RewriteInstructions []RewriteInstruction `json:"rewrite_instructions,omitempty"`
}

type RewriteInstruction struct {
    ClaimID              string   `json:"claim_id,omitempty"`
    Location             string   `json:"location"`
    Problem              string   `json:"problem"`
    Instruction          string   `json:"instruction"`
    SuggestedEvidenceIDs []string `json:"suggested_evidence_ids,omitempty"`
    Severity             string   `json:"severity"` // required | optional
}
```

## 质量门控

章节通过条件：

- `scores.overall >= 80`
- `scores.citation_consistency >= 90`
- 不存在高风险 unsupported claim
- 不存在 `severity=required` 的 `rewrite_instructions`；required 指令未覆盖时继续重写，超过 2 轮标记 `needs_human_review`

未通过时进入重写。重写最多 2 轮，超过后标记 `needs_human_review`，但不阻塞后续章节。

## 章节状态

建议状态：

- `drafting`
- `reviewing`
- `revision_required`
- `accepted`
- `needs_human_review`
- `committed`

`commit_chapter` 只接受已通过 review 的版本。

## 引用硬规则

- 正文中所有关键论断必须出现在 `claims.json`。
- 每个关键论断必须映射到至少一篇 confirmed reference。
- Editor 必须检查文献是否支持该论断；不确定时按不通过处理。
- 禁止使用无来源泛化表达。
- 最终输出必须保留 claim 到 reference 的追踪文件。
