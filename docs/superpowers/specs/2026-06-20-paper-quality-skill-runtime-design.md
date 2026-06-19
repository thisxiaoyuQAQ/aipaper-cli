# 论文质量 Skill 运行时启用设计

日期：2026-06-20

状态：增量需求规划，待开发

## 背景

`docs/skills/paper-cli-paper-quality` 已经把论文写作、引用纪律、证据约束、claim 支撑度、审稿式修改和质量报告等内容蒸馏成项目内 skill 文档。但这些内容目前仍主要是开发参考，程序运行时尚未把它们系统性注入 Architect、Writer、Verifier、Editor 和导出报告。

当前 Quality Engine 已具备完整质量链路：confirmed references → EvidenceTable → SectionQualityPlan → Writer claims → ClaimGraph → VerificationResult → quality gate → rewrite instructions → QualityReport。因此本次增量不重写质量引擎，而是把论文质量 skill 内容本地化为可测试的运行时策略包，并挂入现有链路。

## 目标

- 让论文质量 skill 内容在程序运行时默认生效。
- 将质量规则应用到大纲、证据提炼、章节计划、章节写作、claim 验证、审稿重写和质量报告。
- 保持现有 artifact schema 和 checkpoint 恢复语义稳定。
- 通过测试证明策略已注入各角色 prompt，并在质量报告中可见。

## 非目标

- 不在运行时直接读取 `docs/skills/paper-cli-paper-quality` 文件。
- 不依赖 Claude Code / Cursor 的外部 skill runtime。
- 不新增独立 Verifier 角色；Verifier 仍是 `editor_run(task=verify)`。
- 首期不新增 `claim_type` 字段；claim type 作为 verifier 内部判断或写入 `verifier_note`。
- 不改变 confirmed references、Writer Guard、Quality Gate 的硬阻断语义。

## Paper Quality Skill Pack

运行时权威实现为 Go 代码中的本地策略 helper：

```text
internal/quality/paper_quality_policy.go
```

该 helper 输出按 scope 分组的短规则：

- Coordinator：全局质量原则、引用纪律、流程决策边界；
- Architect：叙事契约、证据深度诚实、章节计划 rubric；
- Writer：证据约束写作、禁止低内容信号、风格 guard；
- Verifier：claim type/support 判断、证据深度与 claim 强度匹配；
- Editor：审稿维度、rewrite instruction、human review triggers；
- Export：质量报告说明、证据深度解释、人工行动建议。

## 运行时接入点

| 阶段 | 文件 | 接入方式 |
|---|---|---|
| Coordinator | `internal/agent/prompt.go` | 在 `CoordinatorSystemPrompt` 中追加 Paper Quality 全局策略 |
| Architect outline | `internal/app/architect_runner.go` | 注入 narrative / thesis / chapter role 规则 |
| Architect evidence | `internal/app/architect_runner.go` | 注入 evidence depth honesty 规则 |
| Architect section plan | `internal/app/architect_runner.go` | 注入 SectionPlan rubric |
| Writer | `internal/agent/writer_quality.go`、`internal/app/writer_runner.go` | 通过 `TemplateGuidance`、`ForbiddenDraftPatterns` 和 writer prompt 注入策略 |
| Verifier | `internal/app/editor_runner.go` | 在 `runVerify` 中注入 claim type/support rubric |
| Editor | `internal/app/editor_runner.go` | 在 `buildReviewPrompt` 中注入 reviewer/rewrite rubric |
| Export | `internal/export/quality_report.go` | deterministic 渲染 policy version、证据深度解释与 human action items |

## 角色职责变化

### Architect

Architect 不只生成章节列表，还要让大纲和 SectionQualityPlan 体现：

- one-sentence thesis 或明确综述主线；
- 每章的单一职责；
- 每章 questions 必须可由本章证据回答；
- boundaries / forbidden_generalizations 必须约束证据不足时不能写的强结论；
- gaps / human_review_hints 必须显式记录材料不足。

### Writer

Writer 必须在章节质量计划和 evidence table 范围内写作：

- 每个重要 claim 绑定 evidence id；
- claim 强度匹配 evidence depth；
- 证据不足进入 `writer_notes`，不得成为正文主体；
- 不用候选文献、拒绝文献或模型记忆编造引用；
- 避免空泛套话和 AI 味，但不得改变证据范围。

### Verifier

Verifier 在输出 `ClaimVerdict` 前进行内部 claim type 判断：

- descriptive / comparative / causal / generalization / methodological / limitation / reproducibility；
- 判断 evidence 是否同对象、同关系、同范围；
- 不确定按 unsupported；
- claim type 不新增 schema，必要时写入 `verifier_note`。

### Editor

Editor 按审稿人视角输出 review：

- 区分语言问题、证据问题、结构问题；
- unsupported / overstated / high-risk claim 必须给 required rewrite instruction；
- 无法靠现有 evidence 修复时标 human review；
- 每条 instruction 必须具体到 location、problem、instruction、suggested_evidence_ids、severity。

### Export

QualityReport 不调用 LLM，只根据已有质量产物 deterministic 渲染：

- policy version；
- evidence depth distribution 与解释；
- claim support summary；
- unsupported / overstated claims；
- evidence sufficiency / low content signals；
- human action items。

## 数据与接口契约

首期复用现有契约：

- `Evidence.Depth` 表达证据深度；
- `SectionPlan.Questions/Boundaries/ForbiddenGeneralizations/Gaps/HumanReviewHints` 表达章节质量计划；
- `ClaimVerdict.Support/RiskLevel/VerifierNote` 表达支撑度和风险；
- `RewriteInstruction` 表达审稿式修改；
- `GateIssue.Code` 表达机器可查的质量风险。

不新增必填字段，不改变 tool schema，不改变 `requirements.json` 兼容性。

## TUI 与用户可见行为

- 用户仍通过 `quality_mode` 选择 fast / enhanced / strict。
- 运行时策略默认随质量链路启用；不新增单独开关。
- `final/quality-report.md` 会更清楚解释证据深度、claim 风险和人工下一步。
- 若旧项目缺少质量产物，仍走兼容模式，不生成误导性质量报告。

## 测试与验收

- `internal/quality`：测试 `PaperQualityPolicy` 纯函数、version、各 scope 关键规则。
- `internal/agent`：测试 `CoordinatorSystemPrompt` 包含 Paper Quality policy。
- `internal/app`：测试 Architect / Writer / Verifier / Editor prompt 包含对应策略。
- `internal/export`：测试 quality report 包含 policy version、证据深度解释和 human action items。
- 回归：`go test ./...` 与 `go build ./cmd/aipaper-cli`。

## 实现模块拆分

- 39：运行时设计与接口文档；
- 40：PaperQualityPolicy 本地策略底座；
- 41：Coordinator 与 Architect 策略注入；
- 42：Writer 证据约束与风格策略注入；
- 43：Verifier 与 Editor 审稿 rubric 注入；
- 44：QualityReport 质量 skill 摘要增强；
- 45：端到端验收与文档同步。
