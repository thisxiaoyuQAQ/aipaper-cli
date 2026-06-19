---
name: paper-cli-paper-quality
description: Distills paper-writing prompts and agent skills into Paper-Cli quality workflows. Use when improving academic review generation, evidence-grounded writing, claim verification, reviewer-style editing, or final paper quality reports in this project.
version: 1.0.0
author: Paper-Cli Project
license: MIT
tags: [Academic Writing, Paper Quality, Evidence, Citations, Reviewer, Prompt Engineering]
---

# Paper-Cli Paper Quality Skill

本 skill 将 `docs/skills` 中论文写作、科研 agent skill、审稿质量、图表与 citation discipline 的材料蒸馏为 Paper-Cli 可直接复用的质量工具。它不是通用写作 prompt 集，而是面向本项目现有流水线的“质量契约”：`references/confirmed.json` → `quality/evidence-table.json` → `quality/section-quality-plan.json` → Writer claims → `quality/claim-graph.json` → verification → rewrite instructions → final quality report。

## 核心原则

1. **论文是证据支撑的单一叙事，不是材料堆砌**：每篇综述必须能说清 one-sentence contribution 或 one-sentence thesis。每章都服务于主线。
2. **引用存在与引用支撑要分层**：Host 校验 reference key 是否来自 confirmed references；Verifier 判断证据是否真的支持 claim。
3. **不同 claim 需要不同证据**：比较类 claim 需要 baseline；因果类 claim 需要 ablation 或机制证据；泛化类 claim 需要跨场景证据；综述类 claim 需要明确边界和代表性。
4. **写前约束优先于写后补救**：Section Quality Plan 必须先定义每章问题、证据、边界、禁用泛化和人工复核点。
5. **Reviewer 视角贯穿流程**：Editor 不只是润色语言，而是产生可执行、可追踪、指向 claim/evidence 的 rewrite instructions。
6. **风格服务于清晰性**：去 AI 味、压缩、扩写、翻译等都不能改变证据范围、不能编造数据、不能把 metadata-only 写成 fulltext certainty。

## 何时使用

使用本 skill：

- 为 Paper-Cli 的 Architect/Writer/Editor/Verifier prompt 增强论文质量规则；
- 设计或审查 Evidence Table、Section Quality Plan、Claim Graph、Verification、Quality Report；
- 把用户材料和已确认文献加工成高质量综述章节；
- 对草稿做“审稿人视角”而不是普通语言润色；
- 生成或维护本项目内的论文质量提示词。

不要使用本 skill：

- 只想复制上游 prompt 到聊天框手工润色；
- 需要当前会议最新 CFP、页数、AI policy 的权威事实，必须联网核验；
- 需要自动确认未核验文献真实性。Paper-Cli 的正式引用仍必须来自 confirmed references。

## 工作流 1：叙事契约与章节计划

目标：在 Writer 写作前，把“这篇文章要证明什么”和“每章允许写什么”变成结构化约束。

```text
Narrative & Section Planning
- [ ] 从 requirements、materials、confirmed references 中提炼 one-sentence thesis。
- [ ] 用 What / Why / So What 描述主线。
- [ ] 将主线拆成 3-6 个章节职责。
- [ ] 为每章生成 questions、required_evidence_ids、boundaries、forbidden_generalizations。
- [ ] 标记 evidence gaps 和 human_review_hints。
- [ ] 检查每章是否只承担一个主要功能，避免章节重复。
```

输出应映射到 `quality/section-quality-plan.json`：

- `questions`：本章必须回答的问题；
- `required_evidence_ids`：本章必须使用的证据；
- `boundaries`：本章范围，不应越界到其他章节；
- `forbidden_generalizations`：证据不足时禁止写出的强结论；
- `gaps`：材料不足或需要人工确认的缺口；
- `human_review_hints`：用户或人工编辑必须重点看的风险。

## 工作流 2：证据约束写作

目标：Writer 只写证据允许写的内容，并把每个重要论断落成可校验 claim。

```text
Evidence-Grounded Writing
- [ ] 读取本章 SectionPlan 和 Evidence items。
- [ ] 只使用 confirmed reference keys。
- [ ] 每个关键论断绑定至少一个 evidence_id。
- [ ] 根据 evidence depth 控制语气强度。
- [ ] 把缺失证据写进 writer_notes，而不是补虚假内容。
- [ ] 输出 draft_markdown、claims、citation_mappings、writer_notes。
```

Writer 必须遵守：

- metadata-only 证据只能支撑“存在某研究/主题相关”的弱表述；
- abstract 证据可以支撑概要级结论，但不应支撑细节性机制或强因果；
- snippet/fulltext_excerpt 才能支撑更具体的发现；
- 如果 evidence 不足，使用保守语气或显式标 gap；
- Related Work 引用不能替代本方法/本综述主张的直接证据；
- 不得把候选文献、被拒绝文献或在线搜索结果写成正式引用。

## 工作流 3：claim 类型与支撑度验证

目标：Verifier 不只判断“有没有引用”，还判断“这种证据是否足以支撑这种 claim”。

```text
Claim Verification
- [ ] 给 claim 分类：descriptive / comparative / causal / generalization / methodological / limitation / reproducibility。
- [ ] 检查 evidence 是否相关。
- [ ] 检查 evidence kind 是否足够。
- [ ] 检查 scope 是否过宽。
- [ ] 判定 support：supported / partially_supported / unsupported / overstated。
- [ ] 设定 risk_level：high / medium / low。
- [ ] 写出 verifier_note，必须说明证据缺口或需降级的原因。
```

常见判定：

| 情形 | Verdict |
|---|---|
| claim 与 evidence 直接一致，范围匹配 | supported |
| 证据只支持一部分条件或一部分对象 | partially_supported |
| evidence 无关、缺失或只是引用存在 | unsupported |
| claim 比 evidence 支持的范围更强、更泛或更确定 | overstated |

## 工作流 4：审稿式编辑与重写指令

目标：Editor 产生可执行的修改任务，而不是泛泛说“逻辑不清”。

```text
Reviewer Editing
- [ ] 按 Quality / Clarity / Significance / Originality 审查章节。
- [ ] 对每个 major finding 指向 chapter、paragraph 或 claim_id。
- [ ] 区分表述问题、证据问题、结构性缺陷。
- [ ] 将必须修改的问题转为 rewrite_instructions。
- [ ] 每条 instruction 给出 location、problem、instruction、suggested_evidence_ids、severity。
- [ ] 如果无法靠已有 evidence 修复，标 human review，不要求 Writer 编造。
```

Editor finding 必须具体到可操作层面。例如：

- 不合格：`实验不够充分。`
- 合格：`claim ch02_claim_003 声称方法在多场景下稳定有效，但 evidence ev_004 只来自单一数据集摘要。请将该句降级为“在该数据集上显示出潜在优势”，或补充跨数据集证据后再保留泛化表述。`

## 工作流 5：终稿质量报告

目标：导出阶段不只生成 paper.md，还要告诉用户“哪些地方可信、哪些地方需要人工复核”。

```text
Final Quality Report
- [ ] 汇总 hard blockers：未确认引用、claim 无 evidence、unknown evidence。
- [ ] 汇总 support verdict：supported / partial / unsupported / overstated。
- [ ] 汇总 evidence depth：metadata / abstract / snippet / fulltext_excerpt。
- [ ] 汇总 high-risk claims 和 needs_human_review。
- [ ] 输出 reviewer-style risk summary。
- [ ] 输出 human action items。
```

质量报告建议结构：

1. Gate Status；
2. Evidence Coverage；
3. Claim Support Summary；
4. Reviewer Risk Summary；
5. Rewrite Summary；
6. Human Action Items；
7. Venue / Format Reminders（若用户指定会议，提醒以当前 CFP 为准）。

## Prompt 模块

可直接复用的完整提示词见：

- [references/prompt-modules.md](references/prompt-modules.md)：Architect、Writer、Verifier、Editor、Export prompt 模块；
- [references/quality-rubrics.md](references/quality-rubrics.md)：claim 类型、证据深度、审稿维度、语言风格规则；
- [references/distillation-notes.md](references/distillation-notes.md)：从上游资料中提炼了什么、没有照搬什么。

## 与 Paper-Cli 现有质量引擎的映射

| 本 skill 概念 | Paper-Cli 产物 |
|---|---|
| Citation discipline | `references/confirmed.json`、`confirmed.bib` |
| Evidence depth / provenance | `quality/evidence-table.json` |
| Narrative contract | `quality/section-quality-plan.json` |
| Evidence-grounded draft | `drafts/{chapter}/draft-vN.md` + `claims-vN.json` |
| Claim support graph | `quality/claim-graph.json` |
| Reviewer verification | `quality/verification-result.json` |
| Required revisions | `reviews/{chapter}/review-vN.json` 的 `rewrite_instructions` |
| Human action list | `final/quality-report.md` |

## 不应照搬的上游内容

- 不直接复制长篇润色 prompt；只抽规则和输出契约。
- 不把模型排行榜或具体模型偏好写进项目核心逻辑。
- 不把会议页数、deadline、AI policy 当永久事实；只作为待核验 checklist。
- 不自动把未核验 metadata 变成 confirmed reference。
- 不把模拟 reviewer score 当作质量真值；只输出风险、证据缺口和行动建议。

## 最小原则

如果只能保留一句规则：

> 每个章节回答一个明确问题；每个重要论断绑定 confirmed evidence；每个证据只支撑它实际能支撑的范围；每个重写建议都指向具体 claim 和 evidence。
