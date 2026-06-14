# 文章生成标准化与内容质量重做设计

- 日期：2026-06-14
- 状态：待用户审阅
- 前置：模块 01-31 已完成；Quality Engine 已具备 Evidence Table、Section Quality Plan、Claim Graph、Verification、Gate、Quality Report 基础能力
- 上游文档：`docs/需求与架构.md`、`docs/superpowers/specs/2026-06-10-quality-engine-design.md`
- 问题样本：`output/aipaper/final/paper.md`

## 1. 背景与问题

上一版最终论文 `output/aipaper/final/paper.md` 暴露出三个核心问题：

1. **格式与排版不稳定**：最终 Markdown 更像章节拼接结果，缺少稳定的题目、摘要、关键词、正文编号、结论、参考文献等完整论文结构。
2. **引用不标准**：正文仍使用 reference key 形式，如 `[原2025StudyEmotionalResonance]`，没有按中文论文常见的顺序编码引用和文末参考文献列表呈现。
3. **内容质量低**：正文大量围绕“证据不足、只能作为框架、待验证”展开，实际主题分析不足。证据弱时系统继续生成正文，导致看起来像论文，但内容主要是限制说明。

本阶段选择“质量管线重构，但分层落地”的方案：一次性覆盖多模板、自动补文献、写作约束、GB/T 7714 引用、最终 Markdown 标准化和质量门控，但不推翻现有 Runtime、TUI、checkpoint 和 Quality Engine 主架构。

## 2. 目标

本阶段目标是新增一条逻辑上的 **Paper Standardization Pipeline**，让最终文章达到以下效果：

1. **多模板**：支持至少两类模板：中文课程论文与综述论文。模板定义必备块、标题层级、编号规则、证据要求和导出结构。
2. **自动补文献**：当 confirmed references 或 evidence table 不足以支撑章节时，系统自动生成检索 query，调用现有 search provider 补充候选文献，而不是直接强写空泛正文。
3. **默认 GB/T 7714**：正文默认渲染为顺序编码引用 `[1]`，文末输出 GB/T 7714 风格参考文献列表。
4. **写作内容质量约束**：Writer 不再把证据缺口写成正文主体；证据缺口应进入 notes、quality report 或触发补文献/质量失败。
5. **最终 Markdown 标准化**：`paper.md` 由统一 renderer 组装，稳定包含题目、摘要、关键词、正文、结论、参考文献。

## 3. 非目标

1. 本阶段不实现完整 Word 排版引擎，不覆盖所有学校或期刊的细则。DOCX 可继续复用现有导出能力。
2. 不推翻现有 Architect / Writer / Editor / Quality Runtime 流程。
3. 不引入新的重型 Agent 角色。优先在 Host 层增加模板、证据充分性、引用渲染和最终排版能力。
4. 不把自动补文献设计成无限循环。补文献必须有轮数、候选数和失败出口。

## 4. 总体架构

新增 `Paper Standardization Pipeline`，由五个模块组成。

### 4.1 Template Registry

Template Registry 负责定义论文模板 schema。每个模板至少包含：

- `template_id`：如 `zh_course_paper`、`review_paper`；
- 必备块：题目、摘要、关键词、引言、正文、结论、参考文献；
- 标题编号规则：如 `## 1 引言`、`### 1.1 研究背景`；
- 每章最少段落数、每章最少 evidence 数；
- 每类块的写作目标和禁忌；
- 默认 citation style，当前默认 `gbt7714`。

模板配置进入 requirements 或由 requirements 派生。若用户未选择模板，系统默认使用 `zh_course_paper`，并记录 warning。

### 4.2 Evidence Sufficiency Checker

Evidence Sufficiency Checker 在写作前读取：

- `references/confirmed.json`；
- `quality/evidence-table.json`；
- `outline/outline.json`；
- requirements/template 配置。

它为每章生成 sufficiency report，判断是否满足模板要求。例如：

```text
chapter_02:
  status: insufficient
  missing_topics:
    - 原神玩家评论实证研究
    - 鸣潮玩家评论实证研究
    - 游戏线上营销策略案例
  weak_evidence:
    - ev_001 only metadata_only
  required_action:
    - expand_references
```

证据不足时，系统不得直接进入 Writer 强写正文，而应触发自动补文献或质量失败。

### 4.3 Reference Expansion Planner

Reference Expansion Planner 根据 sufficiency report 生成检索 query，并调用现有 `internal/search` provider。

示例 query：

```text
原神 玩家评论 情感 共鸣 营销策略
鸣潮 玩家评价 社区运营 战斗体验
Genshin Impact player reviews marketing strategy
Wuthering Waves player feedback community operation
```

补文献流程采用有限循环：

1. 每章最多补文献 1-2 轮；
2. 每轮最多新增固定数量候选；
3. 候选进入现有 dedupe 和 candidates 写入流程；
4. MVP 阶段仍走现有用户确认链路；
5. 后续可增加 trusted auto-confirm 条件，例如 DOI、标题相似度、摘要主题匹配达到阈值；
6. 达到上限仍不足时，输出 `REFERENCE_EXPANSION_EXHAUSTED` 并阻断写作或质量失败。

### 4.4 Standardized Writer Contract

Writer 输入升级为结构化的 Chapter Writing Contract。它不只说明“写第几章”，还要说明模板块、段落目标、可用 evidence、引用要求和禁止模式。

示例：

```json
{
  "template_id": "review_paper",
  "chapter_id": "chapter_02",
  "chapter_title": "用户评价中的内容体验差异",
  "required_blocks": ["section_intro", "comparative_analysis", "section_summary"],
  "allowed_evidence_ids": ["ev_001", "ev_002"],
  "citation_style": "gbt7714",
  "forbidden_patterns": [
    "反复说明证据不足",
    "用研究设计替代实质分析",
    "引用未确认文献"
  ]
}
```

Writer JSON 输出继续保留 `draft_markdown`、`claims`、`citation_mappings`、`writer_notes`，但新增或强化以下要求：

- 每个段落有稳定 `paragraph_id`；
- 每个 claim 必须绑定 evidence ids；
- 每个 citation 只能引用 confirmed reference key；
- 证据缺口写入 `writer_notes`，不得成为正文主要内容；
- 高重要性 claim 不允许只绑定 metadata-only evidence。

### 4.5 Citation & Final Renderer

导出阶段新增统一 renderer，负责从 accepted chapters、citation map 和 confirmed references 中生成最终论文。

职责包括：

- 建立 reference key 到顺序编号的映射；
- 将正文引用渲染为 `[1]`、`[1-2]`、`[1,3]`；
- 按正文首次出现顺序生成文末参考文献；
- 按 GB/T 7714 风格格式化参考文献条目；
- 统一组装题目、摘要、关键词、正文、结论、参考文献；
- 写出 citation trace，便于质量报告追踪。

最终 Markdown 示例结构：

```markdown
# 原神与鸣潮玩家评价及线上营销策略比较研究

## 摘要
本文围绕两款开放世界游戏的玩家评价与线上营销策略展开比较，说明研究对象、材料来源、核心发现与局限。

**关键词：** 原神；鸣潮；玩家评价；线上营销；情感共鸣

## 1 引言
本节交代研究背景、研究问题、材料来源与比较维度。

## 2 用户评价中的内容体验差异
本节依据已确认文献和 evidence table 分析叙事、玩法、视听与运营体验差异。

## 结论
本文归纳主要发现、证据边界与后续研究方向。

## 参考文献
[1] 作者. 题名[J/OL]. 来源, 年份.
```

## 5. 数据流

完整数据流如下：

```text
Requirements + Template
→ Materials / Search
→ Reference Candidates
→ Reference Confirmation
→ Evidence Table
→ Evidence Sufficiency Check
  ├─ sufficient → Section Quality Plan → Writer
  └─ insufficient → Reference Expansion Planner → Search → Candidates/Confirmation → Evidence Table → re-check
→ Standardized Writer Contract
→ Draft + Claims + Citation Map
→ Claim Graph / Verification / Gate
→ Citation & Final Renderer
→ paper.md + citation trace + quality report
```

补文献是质量门前置动作，不是最终失败后的补救动作。

## 6. 错误处理

新增或明确以下错误码：

| 错误码 | 触发条件 | 处理方式 |
|---|---|---|
| `PAPER_TEMPLATE_MISSING` | requirements 没有模板且无法推断 | 回退 `zh_course_paper`，记录 warning |
| `EVIDENCE_INSUFFICIENT` | 某章节 evidence/reference 不足 | 触发自动补文献 |
| `REFERENCE_EXPANSION_EXHAUSTED` | 补文献达到上限仍不足 | 阻断写作或质量失败，输出缺口报告 |
| `CITATION_UNCONFIRMED` | Writer 使用未确认引用 key | 拒绝落盘或进入 rewrite |
| `CITATION_FORMAT_INVALID` | 引用无法渲染为顺序编码 | 导出前修复；失败则质量门失败 |
| `STRUCTURE_INVALID` | 缺摘要、关键词、结论、参考文献等必备块 | final renderer 拒绝导出 |
| `LOW_CONTENT_SIGNAL` | 正文大量空泛表达或用缺口说明替代主题分析 | rewrite 或质量失败 |

关键原则：**证据不足可以出现在质量报告里，但不能变成正文的主要内容。**

## 7. 质量门新增检查

在现有 Quality Engine 基础上新增四类检查。

### 7.1 结构完整性检查

- 是否包含摘要、关键词、引言、正文、结论、参考文献；
- 标题层级是否符合模板；
- 正文章节编号是否连续；
- 最终 Markdown 不允许出现章节编号错位或重复。

### 7.2 引用一致性检查

- 所有正文引用都能映射到 confirmed reference；
- 所有引用编号都出现在参考文献列表；
- 默认按 GB/T 7714 风格输出；
- 未确认文献不能进入正文引用；
- 参考文献列表按正文首次引用顺序排列。

### 7.3 证据充分性检查

- 每章至少满足模板定义的 reference/evidence 数量；
- 高重要性 claim 必须有 evidence；
- metadata-only evidence 不能支撑强结论；
- 弱证据只能支撑谨慎表述，不能支撑确定性比较。

### 7.4 内容信号检查

- 统计“证据不足、待验证、只能提出框架、不能证明、可能”等套话密度；
- 检查章节是否包含具体对象、具体维度、具体比较；
- 正文必须覆盖主题核心实体，如《原神》《鸣潮》、玩家评价、营销策略等；
- 如果正文大篇幅停留在方法说明而非主题分析，判为低质量。

## 8. 与现有代码的接入点

主要接入点如下：

- `internal/app/writer_runner.go`：改造 Writer prompt 和输出 contract；
- `internal/quality/evidence.go`：复用 Evidence Table，并增加 sufficiency 检查所需读取逻辑；
- `internal/quality/sectionplan.go`：让 Section Quality Plan 接收模板和 sufficiency 结果；
- `internal/quality/gate.go`：增加结构、引用、证据充分性和内容信号检查；
- `internal/search/run.go` 与 provider：供 Reference Expansion Planner 调用；
- `internal/references/confirm.go`：复用候选确认、去重和 confirmed references 数据；
- `internal/export/*`：新增 citation renderer、GB/T 7714 formatter、final paper renderer；
- `internal/export/quality_report.go`：汇总模板、补文献和内容信号检查结果。

## 9. 测试与验收

### 9.1 单元测试

新增或扩展以下测试：

1. Template Registry：默认模板、必备块、标题编号规则；
2. Evidence Sufficiency Checker：metadata-only 弱证据触发 insufficient；
3. Reference Expansion Planner：按缺口生成合理 query，并有轮数上限；
4. GB/T 7714 Formatter：中文/英文作者、年份、标题、DOI/URL 的基础格式；
5. Citation Renderer：reference key 到 `[1]` 编号映射、重复引用复用编号、未确认引用失败；
6. Final Renderer：生成题目、摘要、关键词、正文、结论、参考文献结构；
7. Content Signal Checker：大量缺口套话被判为 `LOW_CONTENT_SIGNAL`。

### 9.2 集成测试

新增质量夹具覆盖：

1. 证据不足 → 自动补文献 planner 被触发；
2. 补文献仍不足 → 写作被阻断并输出缺口报告；
3. confirmed references 足够 → Writer contract 正常构建；
4. accepted chapters + citation map → final renderer 输出 GB/T 引用和参考文献。

### 9.3 回归测试

必须保持：

- 现有 TUI 写作流程仍能启动；
- checkpoint/recovery 不被破坏；
- quality-mini、review-mini、export 相关测试继续通过；
- 最终导出仍能生成 `paper.md`、质量报告和已有导出产物。

## 10. 验收标准

本阶段完成后，至少满足：

1. `paper.md` 包含题目、摘要、关键词、正文、结论、参考文献；
2. 正文标题编号规范，没有重复或错位层级；
3. 默认正文引用为顺序编码 `[1]`；
4. 文末参考文献编号与正文引用一致；
5. 未确认文献不能进入正文引用；
6. 证据不足时系统自动补文献或失败报告，不再生成以缺口说明为主体的正文；
7. 每章至少有可追踪 evidence；
8. 核心论点能通过 claim graph 找到支撑；
9. 质量报告说明补文献尝试、证据缺口、引用检查和内容信号检查结果。
