# Paper Quality Skill 运行时策略契约

> 状态：增量规划，模块 39-45 实现。
> 权威目标：把 `docs/skills/paper-cli-paper-quality` 蒸馏内容本地化为运行时可测试策略，不直接读取 docs/skills 文件。

## 1. 设计边界

Paper Quality Skill 在运行时不是外部 skill 调用，而是本项目内部的稳定策略包：

```text
internal/quality/paper_quality_policy.go
```

该策略包只返回静态 prompt sections / report labels：

- 不读文件；
- 不访问 Store；
- 不调用 LLM；
- 不新增 artifact；
- 不改变 JSON schema；
- 不改变 checkpoint 恢复语义。

## 2. Policy Version

```go
const PaperQualityPolicyVersion = "paper-cli-paper-quality-v1"
```

Quality Report 应展示该版本，便于用户知道本次报告使用了哪套论文质量策略。

## 3. 推荐 Go API

```go
type PaperQualityPolicy struct {
    Version               string
    CoordinatorSections   []string
    ArchitectNarrative    []string
    EvidenceDepth         []string
    SectionPlan           []string
    Writer                []string
    Verifier              []string
    Editor                []string
    Report                []string
}

func DefaultPaperQualityPolicy() PaperQualityPolicy
func PaperQualityCoordinatorSections() []string
func PaperQualityArchitectNarrativeSections() []string
func PaperQualityEvidenceDepthSections() []string
func PaperQualitySectionPlanSections() []string
func PaperQualityWriterSections() []string
func PaperQualityVerifierSections() []string
func PaperQualityEditorSections() []string
func PaperQualityReportSections() []string
```

所有函数必须输出稳定顺序，方便测试和 prompt 快照断言。

## 4. Scope 语义

### CoordinatorSections

用于 `CoordinatorSystemPrompt`，只包含全局规则：

- 只能使用 confirmed references；
- citation existence 不等于 claim support；
- evidence depth 控制 claim strength；
- 证据不足要降级、记录 gap 或 human review；
- Host 只机器校验，Coordinator 基于工具事实决策。

### ArchitectNarrative

用于 outline prompt：

- 大纲必须围绕一个清晰论文主线；
- 每章有单一职责；
- 章节列表不应是材料堆砌；
- 不把“证据不足说明”设计成正文主体。

### EvidenceDepth

用于 evidence extraction、Writer、Verifier、Quality Report：

| Depth | 可支撑 | 禁止支撑 |
|---|---|---|
| metadata_only | 文献存在、主题相关、元信息 | 方法细节、结果数值、因果、强结论 |
| abstract | 高层目标、方法类别、主要结论概述 | 全文细节、复杂机制、精确实验条件 |
| snippet | snippet 覆盖的局部事实 | snippet 外的全文结论 |
| fulltext_excerpt | excerpt 覆盖的具体事实 | excerpt 未覆盖的其他内容 |

### SectionPlan

用于 section quality plan prompt：

- `questions` 必须能由本章回答；
- `required_evidence_ids` 必须来自 EvidenceTable；
- `boundaries` 防止章节越界和重复；
- `forbidden_generalizations` 明确证据不能支持的强结论；
- `gaps` 是真实缺口，不是让 Writer 编造的占位；
- `human_review_hints` 标注需要人工判断的风险。

### Writer

用于 `WriterChapterInput.TemplateGuidance`、`ForbiddenDraftPatterns` 和 Writer prompt：

- 每个重要 claim 必须绑定 evidence id；
- claim 强度匹配 evidence depth；
- 不把证据缺口、待验证说明、框架声明写成章节主体；
- 不编造数据、引用、趋势或统计显著性；
- 风格清晰克制，不使用空泛宣传腔。

### Verifier

用于 `editor_run(task=verify)`：

- 内部判断 claim type：descriptive / comparative / causal / generalization / methodological / limitation / reproducibility；
- 检查 evidence 是否同对象、同关系、同范围；
- 输出仍为现有 `ClaimVerdict`；
- claim type 首期不入 schema，必要时写入 `verifier_note`；
- 不确定按 unsupported。

### Editor

用于 `editor_run(task=review)`：

- 按 Quality / Clarity / Significance / Originality / Scope calibration / Citation integrity 审查；
- 区分语言、证据、结构问题；
- unsupported / overstated / high-risk claim 必须有 required rewrite instruction；
- 不能靠现有 evidence 修复时标 human review；
- rewrite instruction 必须具体、可执行、绑定位置或 claim。

### Report

用于 `final/quality-report.md` deterministic 渲染：

- 展示 policy version；
- 解释 evidence depth；
- 解释 GateIssue 的论文质量含义；
- 输出 human action items。

## 5. 与现有质量契约的关系

| Paper Quality 概念 | 现有落点 |
|---|---|
| 引用纪律 | `references/confirmed.json`、Writer Guard、Gate hard blockers |
| Evidence depth | `Evidence.Depth`、`GateIssue`、Quality Report |
| Narrative contract | `SectionPlan.Questions/Boundaries/ForbiddenGeneralizations/Gaps` |
| Evidence-grounded writing | `WriterChapterInput`、`claims-vN.json`、`citation-map-vN.json` |
| Claim support | `ClaimVerdict.Support/RiskLevel/VerifierNote` |
| Reviewer rewrite | `Review.RewriteInstructions` |
| Human review | `ClaimNode.NeedsHumanReview`、`GateIssue.Severity`、Quality Report |

## 6. 兼容规则

- 不新增必填字段；
- 不改变 strict JSON 读取规则；
- 不改变 `quality_mode` 三档语义；
- 不改变 Writer Guard / Editor Guard / Quality Gate 的既有错误码；
- 旧项目缺质量产物时仍为兼容模式。
