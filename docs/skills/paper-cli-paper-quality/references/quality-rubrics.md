# Paper-Cli 论文质量 Rubrics

本文件把上游论文写作与审稿资料压缩成 Paper-Cli 可验证的 rubric。重点不是“写得漂亮”，而是：证据链完整、claim 范围准确、章节叙事清晰、审稿风险可见。

## 1. Narrative Rubric

| 维度 | 通过标准 | 风险信号 |
|---|---|---|
| Thesis | 一句话能说明文章综合了什么、为什么重要 | 只能说“本文综述了某领域”，没有独立组织视角 |
| What | 贡献具体，可映射到章节和 claim | 贡献是泛泛词，如“系统总结”“全面分析” |
| Why | 有 confirmed references 和 evidence 支撑 | 只靠常识或模型记忆 |
| So What | 说明读者为什么需要这篇综述 | 只描述领域很热门 |
| Chapter Role | 每章有单一职责 | 章节之间重复介绍背景或重复同一结论 |
| Flow | old → new，问题 → gap → synthesis → limitation | 段落跳跃，读者无法知道为什么进入下一节 |

## 2. Evidence Depth Rubric

| Evidence depth | 可支撑 | 不应支撑 |
|---|---|---|
| metadata_only | 文献存在、主题相关、年份/作者/来源等元信息 | 方法细节、结果数值、因果机制、强结论 |
| abstract | 论文高层目标、方法类别、主要结论概述 | 细粒度实验设置、失败案例、复杂机制细节 |
| snippet | snippet 中直接出现的局部事实或发现 | snippet 之外的全文结论、全局泛化 |
| fulltext_excerpt | excerpt 覆盖的具体论断、方法、实验或限制 | excerpt 未覆盖的其他章节或未读内容 |

证据使用原则：

- strong claim 需要 high confidence 且 depth 至少为 snippet，最好为 fulltext_excerpt；
- abstract-only 支撑 strong claim 时应降级或标 warning；
- metadata-only 作为关键论断唯一支撑时应进入 needs_revision 或 human review；
- evidence 的 limitations 和 risk_flags 必须影响 claim 语气。

## 3. Claim Type Rubric

| Claim type | 典型表述 | 需要的证据 | 常见降级方式 |
|---|---|---|---|
| descriptive | “X 方法采用...” | source 直接描述 | “该文摘要显示...” |
| comparative | “A 优于 B” | 同条件 baseline、指标、数据集 | “在给定设置下 A 表现更好” |
| causal | “组件 X 带来提升” | ablation、控制变量、机制分析 | “结果表明 X 可能与提升相关” |
| generalization | “适用于多场景” | 跨数据集/任务/人群/模型证据 | “在已报告场景中...” |
| methodological | “该类方法可分为...” | 多篇文献或清晰分类依据 | “本文按...视角划分” |
| limitation | “仍受限于...” | 原文限制、失败结果、合理边界 | 保留限制，不要改成优势 |
| reproducibility | “实验使用...” | 配置、硬件、超参、数据 | “材料未报告该细节” |

Verifier 应先判 claim type，再判 evidence 是否足够。不要因为 claim “听起来合理”就判 supported。

## 4. Reviewer Rubric

### Quality / Technical Soundness

通过标准：

- 每个重要 claim 有 evidence_ids；
- evidence 与 claim 同对象、同关系、同范围；
- 实验或比较结论有 baseline、metric、条件；
- 不能验证的部分进入 gaps 或 human review。

风险信号：

- 引用存在但不支撑该句；
- 用综述性引用支持本研究有效性；
- 把单一场景写成普遍规律；
- 没有数据却声称显著提升。

### Clarity

通过标准：

- 术语一致；
- 每段一个中心；
- 主谓接近；
- 旧信息在前，新信息在后；
- 图表/表格说明能独立理解。

风险信号：

- 同一概念多名；
- 段落同时讲背景、方法、结果；
- “这表明”“该方法”等指代不明；
- 连接词堆砌但逻辑关系不清。

### Significance

通过标准：

- 章节内容服务 thesis；
- 说明文献分歧、趋势、缺口或边界；
- 不只是逐篇罗列。

风险信号：

- Related Work 写成论文清单；
- Introduction 只说领域重要，没有缺口；
- Conclusion 引入新 claim；
- 综述没有综合视角。

### Originality / Synthesis

通过标准：

- 按方法、假设、证据类型、应用场景组织文献；
- 明确不同路线的差异和适用边界；
- 能形成 taxonomy、gap map 或 evidence map。

风险信号：

- 每篇文献一句话；
- 没有比较维度；
- 只重复摘要，没有综合判断。

## 5. Language and Anti-AI Rubric

应避免：

- 空泛宏大词：深刻、颠覆性、范式转移、不可磨灭、remarkable、groundbreaking、comprehensive、pivotal；
- 机械连接：首先其次最后、first and foremost、it is worth noting that；
- AI 味三段式：背景很重要 → 挑战很多 → 本文意义重大；
- 无证据程度副词：显著、极大、全面、充分；
- 为了“高级”替换准确术语；
- 无必要的加粗、斜体、引号、列表化。

应偏好：

- 具体名词和动词；
- 明确对象、条件、范围；
- 直接说明数据或文献支持什么；
- 保留不确定性和限制；
- 中文论文使用自然书面语，不用陈旧公文腔；
- 英文论文使用简单清晰的学术词，不堆复杂词。

## 6. Section-Specific Rubric

### Abstract

- 说明主题和贡献，不要泛泛开头；
- 说明为什么问题重要；
- 说明综述/方法如何组织；
- 提供关键发现或结论；
- 不引入未经正文支持的 claim。

### Introduction

- 1-2 段建立问题和读者动机；
- 明确 gap；
- 给出 thesis；
- 列出 2-4 个可验证贡献；
- 贡献必须能映射到后文章节和 evidence。

### Background / Related Work

- 按方法或问题分组，不按论文逐篇流水账；
- 每组说明共同假设、优势、限制；
- 说明本文综述视角与已有综述或研究的不同；
- 不把 Related Work 变成 introduction 的重复。

### Method / Taxonomy / Framework

- 先定义分类维度；
- 每个维度有来源或依据；
- 说明边界和不适用场景；
- 避免把个人偏好写成客观事实。

### Experiments / Evidence Analysis

- 每个实验或证据块先说明测试哪个 claim；
- 报告核心数字、条件、metric direction；
- 不声称没有统计支持的显著性；
- 有混合结果时如实说明；
- 结论必须回连 thesis。

### Limitations

- 明确 evidence coverage 限制；
- 明确文献选择、检索、材料深度限制；
- 说明限制为什么不推翻核心结论；
- 不把 limitation 写成形式主义段落。

### Conclusion

- 总结 thesis、综合发现和边界；
- 不引入新文献、新数据或新强 claim；
- 指出后续研究或人工复核方向。

## 7. Gate Mapping

| Rubric failure | Suggested gate / action |
|---|---|
| unconfirmed reference | blocked |
| claim missing evidence_id | blocked |
| unknown evidence_id | blocked |
| metadata-only supports high-risk claim | needs_revision in strict, warning/enhanced depending mode |
| abstract-only supports strong causal claim | overstated or partially_supported |
| unsupported high-risk claim | needs_revision |
| repeated claim across chapters | warning + merge/rewrite instruction |
| rewrite rounds exceeded | needs_human_review |
| venue rule unknown | human action item, not hard-coded blocker |
| language is AI-like but evidence is sound | style rewrite, not evidence blocker |

## 8. Human Review Triggers

必须提示人工复核：

- evidence 只到 metadata/abstract，但章节需要强结论；
- 文献 metadata 冲突；
- 用户指定会议规则未联网核验；
- claim 需要领域判断才能确定是否过强；
- 需要新增实验、全文阅读、统计检验或图表重绘；
- Writer/Editor 已重写两轮仍无法通过；
- Quality Report 中存在 high-risk unsupported 或 overstated claim。
