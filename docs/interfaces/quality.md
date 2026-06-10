# Quality Engine 质量产物契约

> 状态：模块 23（EvidenceTable）、24（SectionQualityPlan）、25（Writer 证据使用协议）已实现，权威来源为 `internal/quality/evidence.go`、`internal/quality/sectionplan.go`、`internal/quality/tools.go`、`internal/quality/sectionplan_tools.go`、`internal/agent/writer_quality.go`、`internal/artifacts/claims.go`；模块 26-31 仍为规划产物。
> 设计依据：`docs/superpowers/specs/2026-06-10-quality-engine-design.md` 第 3、4、5 节。

## 1. 存储路径

| 产物 | 路径（相对 Store 根） |
| --- | --- |
| Evidence Table | `quality/evidence-table.json` + `quality/evidence-table.md` |
| Section Quality Plan | `quality/section-quality-plan.json` + `.md` |
| Claim Graph | `quality/claim-graph.json` + `.md` |
| Verification Result | `quality/verification-result.json` |
| Quality Report | `final/quality-report.md` |

写入沿用项目约定：temp + fsync + rename 原子写入、严格 JSON 读取（DisallowUnknownFields）、RFC3339 UTC 时间、正斜杠相对路径。

## 2. EvidenceTable / Evidence（模块 23，已实现）

> 代码：`internal/quality/evidence.go`

```go
type EvidenceTable struct {
    GeneratedAt time.Time  `json:"generated_at"`
    Items       []Evidence `json:"items"`
}

type Evidence struct {
    ID           string   `json:"id"`            // ev_001 风格（^ev_\d{3,}$）
    ReferenceKey string   `json:"reference_key"` // 必须存在于 references/confirmed.json
    MaterialID   string   `json:"material_id,omitempty"` // 来自用户材料时关联 material_001
    Depth        string   `json:"depth"`         // metadata_only|abstract|snippet|fulltext_excerpt
    Topics       []string `json:"topics"`
    KeyFindings  []string `json:"key_findings"`
    Method       string   `json:"method,omitempty"`
    Subjects     string   `json:"subjects,omitempty"`
    Limitations  []string `json:"limitations,omitempty"`
    Excerpt      string   `json:"excerpt,omitempty"` // snippet 级以上才有
    Confidence   string   `json:"confidence"`    // high|medium|low
    Coverage     string   `json:"coverage,omitempty"`
    RiskFlags    []string `json:"risk_flags,omitempty"`
}
```

公开 API：

- `SaveEvidenceTable(s, table) ([]string, error)`：先校验后写入 `quality/evidence-table.json` + `.md`（原子写入），返回相对路径列表；
- `LoadEvidenceTable(s) (EvidenceTable, error)`：严格 JSON 读取（DisallowUnknownFields）；
- `ValidateEvidenceTable(s, table) error`：单独校验；
- `FormatEvidenceTableMarkdown(table) string`：Markdown 渲染。

校验规则与错误码（结构化错误 `quality.Error{code,message,retryable,details}`）：

| 规则 | 错误码 |
| --- | --- |
| `generated_at` 必填；id 必须 `ev_NNN`；depth/confidence 枚举合法；excerpt 仅 snippet 级以上允许 | `evidence_invalid` |
| evidence id 重复 | `evidence_duplicate_id` |
| `reference_key` 不在 confirmed.json（文件缺失视为零确认） | `evidence_unconfirmed_reference` |
| `snippet`/`fulltext_excerpt` 必须有 `material_id` 且 `materials/extracted/<material_id>.md` 非空存在 | `evidence_depth_unsupported` |
| 读写失败、JSON 不合法、文件缺失 | `evidence_io_failed` |

## 3. SectionQualityPlan / SectionPlan（模块 24，已实现）

> 代码：`internal/quality/sectionplan.go`

```go
type SectionQualityPlan struct {
    GeneratedAt time.Time     `json:"generated_at"`
    Sections    []SectionPlan `json:"sections"`
}

type SectionPlan struct {
    ChapterID                string   `json:"chapter_id"` // 必须与 outline 章节一致
    Questions                []string `json:"questions"`
    RequiredEvidenceIDs      []string `json:"required_evidence_ids"` // 必须存在于 Evidence Table
    RecommendedReferenceKeys []string `json:"recommended_reference_keys,omitempty"`
    Boundaries               []string `json:"boundaries,omitempty"`
    ForbiddenGeneralizations []string `json:"forbidden_generalizations,omitempty"`
    Gaps                     []string `json:"gaps,omitempty"`
    HumanReviewHints         []string `json:"human_review_hints,omitempty"`
}
```

公开 API：

- `SaveSectionQualityPlan(s, plan) ([]string, error)`：先校验后写入 `quality/section-quality-plan.json` + `.md`（原子写入），返回相对路径列表；
- `LoadSectionQualityPlan(s) (SectionQualityPlan, error)`：严格 JSON 读取（DisallowUnknownFields）；
- `ValidateSectionQualityPlan(s, plan) error`：单独校验；
- `FormatSectionQualityPlanMarkdown(plan) string`：Markdown 渲染。

校验规则与错误码（结构化错误 `quality.Error{code,message,retryable,details}`）：

| 规则 | 错误码 |
| --- | --- |
| `generated_at` 必填；`chapter_id` 非空 | `section_plan_invalid` |
| 同一 chapter_id 重复出现 | `section_plan_duplicate_chapter` |
| `chapter_id` 不在 `outline/outline.json` 章节列表（outline 缺失视为零章节） | `section_plan_unknown_chapter` |
| `required_evidence_ids` 含 Evidence Table 中不存在的 id（evidence table 缺失视为零证据） | `section_plan_unknown_evidence` |
| 读写失败、JSON 不合法、outline 解析失败 | `section_plan_io_failed` |

新增 Coordinator 步骤（`internal/agent/quality.go`，常量 `StepEvidenceExtraction` / `StepSectionQualityPlan`）：`evidence_extraction`（confirm_references 后）与 `section_quality_plan`（create_outline 同期/后），均走现有 checkpoint 机制；Architect 通过 Coordinator 提示词扩展承担 evidence 提炼与每章质量计划职责，规划规则不硬编码进 Host。

## 4. ClaimGraph / ClaimNode（模块 26、27）

```go
type ClaimGraph struct {
    UpdatedAt time.Time   `json:"updated_at"`
    Claims    []ClaimNode `json:"claims"` // 按章增量 merge，不整体覆盖
}

type ClaimNode struct {
    ID               string   `json:"id"`         // claim_001 风格
    Text             string   `json:"text"`
    ChapterID        string   `json:"chapter_id"`
    ReferenceKeys    []string `json:"reference_keys"` // 机器校验存在于 confirmed.json
    EvidenceIDs      []string `json:"evidence_ids"`   // 机器校验存在于 Evidence Table
    Support          string   `json:"support"`        // supported|partially_supported|unsupported|overstated|skipped(fast)
    RiskLevel        string   `json:"risk_level"`     // high|medium|low
    VerifierNote     string   `json:"verifier_note,omitempty"`
    NeedsRewrite     bool     `json:"needs_rewrite"`
    NeedsHumanReview bool     `json:"needs_human_review"`
}
```

## 5. quality_gate_check（模块 27，纯 Host 逻辑）

输入：Claim Graph + verification result + `quality_mode`。
输出结论枚举：`pass` / `pass_with_warnings` / `needs_revision` / `needs_human_review` / `blocked`。

硬阻断（所有模式）：引用 key 不在 confirmed、claim 无 evidence 绑定、evidence 指向不存在引用、伪造 key。

| 风险情形 | fast | enhanced | strict |
| --- | --- | --- | --- |
| abstract 级证据支撑强结论 | warning | warning | needs_revision |
| metadata_only 作关键论断唯一支撑 | warning | warning | needs_revision（不允许） |
| unsupported claim | warning | needs_revision | needs_revision |
| partially_supported | warning | warning | needs_revision（触发重写） |
| 跨章重复论断 | warning | warning | warning |
| 重写超 2 轮 | needs_human_review | needs_human_review | needs_human_review（report 置顶） |

与 `internal/artifacts` 既有章节门控（总分 ≥80、引用一致性 ≥90）并联：任一 blocked 即阻断。阈值首版固定默认值，不暴露配置。

## 6. 既有契约的规划扩展

实现时在对应模块的边界上扩展，不重写既有规则：

| 契约 | 扩展 | 模块 |
| --- | --- | --- |
| `Requirements`（requirements.md） | 新增 `quality_mode` 字段：`fast` / `enhanced`（默认）/ `strict`；旧文件缺字段时新 run 按 enhanced、恢复旧 run 走兼容模式 | 30 |
| `Claim`（artifacts.md） | ✅ 已实现（模块 25）：`evidence_ids []string`（新 Writer 产物必填 ≥1，JSON `omitempty`）；旧 claims.json 无该字段时严格读取仍通过，按兼容模式处理，`artifacts.ClaimEvidenceWarnings` 产出 `ARTIFACT_CLAIM_MISSING_EVIDENCE` warning（不阻断） | 25 |
| `Review`（artifacts.md） | 新增 `rewrite_instructions` 数组：`claim_id?`、`location`、`problem`、`instruction`、`suggested_evidence_ids`、`severity(required/optional)`；向后兼容 | 28 |
| Step 列表（checkpoint.md） | 新增 `evidence_extraction`（confirm_references 后）、`section_quality_plan`（create_outline 同期/后）、`claim_extraction`（每章 draft 后）、`claim_verification`（claim 抽取后、review 前），全部走现有 checkpoint 机制 | 24, 26, 27 |
| `final/` 导出（export.md） | 新增 `final/quality-report.md`；`report.md` 增加质量摘要；质量报告生成失败不阻塞 paper.md/paper.docx | 29 |
| TUI（tui.md） | Requirements 新增模式选择；WritingProgress 步骤区/章节状态（`verifying`/`needs_revision`）/日志区扩展；ExportSummary 质量结论行；StateProbe 探测 `quality/` 产物；RecoverPrompt 注明质量模式 | 30 |

## 7. Writer 证据使用协议（模块 25，已实现）

> 代码：`internal/agent/writer_quality.go`、`internal/artifacts/claims.go`、`internal/contracts/types.go`（Claim 扩展）

Writer 每章输入（`writer_run` 工具在 confirmed 引用检查通过后注入，runner 收到的 args 即下述结构）：

```go
type WriterChapterInput struct {
    ChapterID   string               `json:"chapter_id"`
    QualityPlan *quality.SectionPlan `json:"quality_plan,omitempty"` // 本章质量计划
    Evidence    []quality.Evidence   `json:"evidence,omitempty"`     // 本章 required evidence 的内容
    Warnings    []string             `json:"warnings,omitempty"`     // 兼容模式提示（质量产物缺失等）
}
```

- 质量产物缺失（旧 run 恢复）→ 兼容模式：注入 warning，不阻断 Writer 运行；
- 质量产物存在但损坏 → 结构化错误（`AGENT_TOOL_FAILED`）。

Host 硬校验（writer guard，公开 API）：

- `agent.GuardWriterClaims(s, claims) error`：每个 claim 必须绑定 ≥1 个 `evidence_ids` 且全部存在于 Evidence Table；引用 key 必须存在于 confirmed.json（既有规则保持）；
- `agent.WriteGuardedDraftBundle(s, bundle) (artifacts.WriteResult, error)`：guard 通过才落盘章节产物（draft/claims/citation map/notes），失败返回结构化错误并整体阻断写入（含 citation map 未确认 key 检查）。

| 规则 | 错误码 |
| --- | --- |
| claim 无任何 evidence 绑定 | `WRITER_CLAIM_MISSING_EVIDENCE` |
| evidence id 不在 Evidence Table | `WRITER_CLAIM_UNKNOWN_EVIDENCE` |
| 引用 key 不在 confirmed.json（含伪造 key） | `WRITER_CLAIM_UNCONFIRMED_REFERENCE` |

错误 `details` 携带 `chapter_id` 与 `claim_id`。Writer prompt 扩展（`CoordinatorSystemPrompt` 追加）：关键论断必须引用 quality plan 绑定的 evidence；材料不足显式标 gap；禁止编造来源（F7）。

## 8. Host 工具（internal/quality）

| 工具 | 状态 | 校验 |
| --- | --- | --- |
| `save_evidence_table` / `load_evidence_table` | ✅ 已实现（`internal/quality/tools.go`，`quality.Tools(s)` 返回） | schema + reference_key 必须 confirmed + depth 渐进规则；save 入参 `{"table": EvidenceTable}`，严格解析未知字段拒绝 |
| `save_section_quality_plan` / `load_section_quality_plan` | ✅ 已实现（`internal/quality/sectionplan_tools.go`，`quality.Tools(s)` 返回） | evidence ID 存在于 Evidence Table + chapter_id 与 outline 一致；save 入参 `{"plan": SectionQualityPlan}`，严格解析未知字段拒绝 |
| `save_claim_graph` | 规划（模块 26） | reference_keys / evidence_ids / chapter_id 全部机器校验 |
| `save_verification_result` | 规划（模块 27） | 支撑关系与风险等级写入，Host 据此算门控 |
| `quality_gate_check` | 规划（模块 27） | 纯 Host 逻辑，接收 mode 参数 |

工具失败统一返回 `{ok:false,error:{code,message,retryable,details}}`，不抛自然语言。工具注册已接线：`internal/agent.DefaultTools` 追加 `quality.Tools(s)`，agent runtime 自动获得全部 quality 工具（模块 24 完成）。

边界原则：「引用存在、claim 有 evidence、evidence 来自 confirmed」由 Host 机器校验；「证据是否真的支撑论断」由 Editor/verifier 语义判断，Host 只记录与执行结果。
