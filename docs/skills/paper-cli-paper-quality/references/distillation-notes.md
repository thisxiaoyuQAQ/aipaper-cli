# 蒸馏说明：从 docs/skills 到 Paper-Cli 论文质量工具

本文件记录本次蒸馏的来源、保留内容和刻意不照搬的内容，便于后续开发者理解 `paper-cli-paper-quality` skill 的边界。

## 主要输入来源

- `docs/skills/awesome-ai-research-writing/README.md`
  - 含中英论文翻译、润色、缩写、扩写、逻辑检查、去 AI 味、图题/表题、实验分析、Reviewer 视角审查等 prompt。
- `docs/skills/AI-Research-SKILLs/20-ml-paper-writing/ml-paper-writing/SKILL.md`
  - 含 narrative principle、What/Why/So What、citation verification、conference checklist、figure/table 规则。
- `docs/skills/AI-Research-SKILLs/20-ml-paper-writing/ml-paper-writing/references/*`
  - 含写作原则、引用工作流、checklist、reviewer guidelines。
- `docs/skills/AI-Research-SKILLs/20-ml-paper-writing/systems-paper-writing/SKILL.md`
  - 含 systems paper 的 paragraph-level blueprint、Gap Analysis、Observation-Driven、Thesis Formula。
- `docs/skills/AI-Research-SKILLs/20-ml-paper-writing/academic-plotting/SKILL.md`
  - 含图表选择、学术图风格、caption/figure 质量要求。
- `docs/skills/AI-Research-SKILLs/22-agent-native-research-artifact/*`
  - 借鉴 provenance、rigor review、type-aware evidence review 的思想。
- `docs/interfaces/quality.md`
  - Paper-Cli 当前 Quality Engine 契约，是本 skill 的落点。

## 提炼出的核心资产

### 1. Citation Discipline

保留为 Paper-Cli 硬规则：

- 不从模型记忆生成引用；
- 不引用 candidates/rejected/search results；
- confirmed reference 只是“可引用”，不自动代表“支撑 claim”；
- 无法确认的引用进入 placeholder 或 human review，不进入正式证据链。

映射位置：`references/confirmed.json`、`quality/evidence-table.json`、Writer Guard、Quality Gate。

### 2. Narrative Principle

保留为写前约束：

- one-sentence thesis；
- What / Why / So What；
- 每章承担一个叙事功能；
- 每章问题、证据、边界、禁用泛化都先写入 SectionPlan。

映射位置：`quality/section-quality-plan.json`。

### 3. Type-Aware Claim Verification

保留为 Verifier 规则：

- descriptive / comparative / causal / generalization / methodological / limitation / reproducibility；
- claim 类型决定需要什么证据；
- supported 不只是“有 citation”，而是 evidence 与 claim 同对象、同关系、同范围。

映射位置：`quality/claim-graph.json`、`quality/verification-result.json`、`GateOutcome`。

### 4. Reviewer-Style Editing

保留为 Editor 规则：

- 按 Quality / Clarity / Significance / Originality 审查；
- finding 必须具体、可执行；
- required revision 必须指向 location、claim_id、evidence_id；
- 无法靠现有 evidence 修复时转 human review。

映射位置：`reviews/{chapter}/review-vN.json` 的 `rewrite_instructions`。

### 5. Academic Style Guard

保留为软规则：

- 去除 AI 味和空泛词；
- 避免机械连接词和无意义强调；
- 术语一致；
- 每段一个核心观点；
- 不为了改写而改写。

映射位置：Writer prompt、Editor language review。它不应覆盖 evidence gate。

## 没有照搬的内容

### 长篇手工 prompt

`awesome-ai-research-writing` 的 prompt 很适合人工复制，但不适合 Paper-Cli 直接内置：

- 输出格式常含 Part 1 / Part 2 / Part 3，不符合本项目 JSON artifact；
- 双语直译会增加运行成本；
- 很多是场景化手工润色偏好；
- 不具备 Host 机器校验接口。

处理方式：只抽取背后的规则和 reviewer self-check。

### 模型选择与榜单

不保留具体模型偏好、版本、榜单。原因：时间敏感，且 Paper-Cli 应由配置控制 provider/model。

### 会议页数与政策硬编码

会议 page limit、AI disclosure、checklist 每年变化。处理方式：作为 Quality Report 中的 `Venue / Format Reminders`，提醒用户按当前 CFP 核验，不作为不可变 gate。

### ARA 完整目录结构

ARA 的 provenance 和 rigor review 有价值，但不搬其完整 schema。Paper-Cli 已有 artifact 结构，应保持本项目产物稳定。

### Reviewer 分数

不把模拟 reviewer score 当真值。Paper-Cli 更应输出 blocker、risk、evidence coverage、human action items。

## 推荐后续落地顺序

1. 先把 `references/prompt-modules.md` 中的 Global Hard Rules、Writer、Verifier、Editor 模块折入现有 runner prompt。
2. 增强 SectionQualityPlan prompt，让 `boundaries` 和 `forbidden_generalizations` 更具体。
3. 在 Verification prompt 中加入 claim type 判断，即使暂时只写入 `verifier_note`。
4. 在 Quality Report 中增加 Evidence Coverage、Claim Support Summary、Human Action Items。
5. 如需改 schema，再考虑给 ClaimNode 增加 `claim_type`、`scope`、`expected_evidence` 等字段。

## 一句话总结

本次蒸馏不是把更多“漂亮写作”模板塞进项目，而是把论文写作经验转化为 Paper-Cli 可追踪、可验证、可恢复的质量契约：叙事先行，证据约束，claim 类型感，审稿式修复，人工复核透明。
