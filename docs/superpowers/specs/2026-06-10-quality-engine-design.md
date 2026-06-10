# Quality Engine 质量引擎设计

- 日期：2026-06-10
- 状态：已批准
- 前置：模块 01-22 已完成（MVP 闭环 + TUI 全流程 + 真实 Runtime 接入）
- 上游文档：`docs/需求与架构.md`、`docs/superpowers/specs/2026-06-05-aipaper-cli-design.md`、`docs/superpowers/specs/2026-06-08-tui-full-generation-user-guide-design.md`

## 1. 目标与定位

Quality Engine 是 `aipaper-cli` 下一阶段的质量增强体系，覆盖四个质量短板：

1. 大纲质量：章节拆分、综述逻辑、避免章节重复；
2. 正文论证质量：减少空话、堆砌和泛泛而谈；
3. 引用可信度：关键论断严格绑定 confirmed references；
4. Editor 审稿与重写能力：发现问题并给出可执行修改。

质量目标：**最终初稿、中间产物、证据链三者兼顾，但所有验收以证据链为硬门槛。**

架构方式：**混合方案 = Host 工具层增强 + 少量专门质量步骤。** 不新增重型 Agent 角色，沿用「LLM 驱动，Host 服务」原则：

- Host 负责质量产物的定义、持久化、机器校验、硬门槛执行、checkpoint 与 report 汇总；
- Agent（Architect / Writer / Editor）负责证据提炼、章节规划、按计划写作、claim 支撑的语义判断和重写建议。

接入策略：**默认质量增强 + 可降级。** 新 run 默认启用；TUI 提供快速初稿降级；旧项目恢复缺少质量产物时进入兼容模式，不阻断、不强制重跑。

## 2. 增强后的整体流程

```text
Requirements
→ Materials / Search
→ References Confirmation（不变，仍是硬门槛）
→ Evidence Table（新）
→ Architect + Section Quality Plan（增强）
→ Writer + Evidence Use Protocol（增强）
→ Claim Graph（新）
→ Claim Verification（新）
→ Editor + Rewrite Instructions（增强）
→ Export + Quality Report（增强）
```

## 3. 核心产物

### 3.1 Evidence Table（证据底座）

路径：

```text
output/aipaper/quality/evidence-table.json
output/aipaper/quality/evidence-table.md
```

每条 evidence 记录：

- 来源 confirmed reference key（必须存在于 `references/confirmed.json`）；
- 是否来自用户材料（材料 ID 关联）；
- 证据粒度 `depth`：`metadata_only` / `abstract` / `snippet` / `fulltext_excerpt`；
- 支持的主题或研究问题；
- 关键发现、方法、对象、局限性；
- 可引用摘要片段或材料摘录；
- 可信度、覆盖范围和风险标记。

证据粒度采用**渐进式**策略：

- 默认基于 confirmed references 的 title / abstract / metadata；
- 用户材料中有 PDF / Markdown / TXT 解析文本时，提升到段落或 snippet 级；
- 没有全文时如实标记 depth，不假装拥有全文证据。

### 3.2 Section Quality Plan（写前章节质量计划）

路径：

```text
output/aipaper/quality/section-quality-plan.json
output/aipaper/quality/section-quality-plan.md
```

每章记录：

- 本章要回答的问题；
- 必须覆盖的 evidence ID（必须存在于 Evidence Table）；
- 推荐引用组合；
- 本章与其他章节的边界；
- 禁止泛化或不能下结论的点；
- 可能的 gap 和人工复核提示。

Architect 生成大纲时同期产出；Writer 写章节时必须同时输入本章 quality plan 和相关 evidence。

### 3.3 Claim Graph（写后论断核验图）

路径：

```text
output/aipaper/quality/claim-graph.json
output/aipaper/quality/claim-graph.md
```

每个 claim 记录：

- claim 文本；
- 所在章节 ID；
- 对应引用 key（机器校验存在于 confirmed.json）；
- 绑定 evidence ID（机器校验存在于 Evidence Table）；
- 支撑关系：`supported` / `partially_supported` / `unsupported` / `overstated`；
- 风险等级；
- verifier 说明；
- 是否需要重写或人工复核。

### 3.4 Quality Report（汇总报告）

路径：`output/aipaper/final/quality-report.md`，同时在现有 `report.md` 加入质量摘要。

内容：

- 整体质量状态；
- 硬门槛通过/失败；
- 证据深度分布；
- unsupported / overstated claims；
- needs_human_review 的章节；
- 重写摘要；
- 用户下一步人工修改建议。

### 3.5 数据流

```text
confirmed references + parsed materials
→ Evidence Table
→ outline + Section Quality Plan
→ draft + claims + citation_map
→ Claim Graph
→ Verification Result
→ rewrite / accept
→ final quality report
```

## 4. 流程步骤、角色职责与工具边界

### 4.1 新增 Coordinator 步骤

全部走现有 checkpoint 机制，可恢复：

```text
step: evidence_extraction      （References 确认后）
step: section_quality_plan     （Architect 大纲时 / 后）
step: claim_extraction         （每章 Writer 完成后）
step: claim_verification       （claim 抽取后、Editor 评审前）
```

### 4.2 角色职责

| 职责 | 承担者 | 说明 |
|---|---|---|
| Evidence 提炼 | Architect（扩展职责） | 从 confirmed references + 材料解析文本提炼 evidence，调用 Host 工具落盘 |
| Section Quality Plan | Architect | 与 outline 同期产出，每章绑定 evidence ID |
| 按计划写作 | Writer | 输入新增本章 quality plan + 相关 evidence；`claims.json` 必须绑定 evidence ID |
| Claim 支撑判断 | Editor（扩展为 verifier） | 对每个 claim 判断 supported / partially / unsupported / overstated |
| Rewrite Instructions | Editor | 评审产物新增结构化 rewrite instructions，逐条指明改什么、用哪条 evidence |

### 4.3 Host 工具层（`internal/quality`）

- `save_evidence_table` / `load_evidence_table`：schema 校验 + 引用 key 必须存在于 `confirmed.json`；
- `save_section_quality_plan` / `load_section_quality_plan`：每章 evidence ID 必须存在于 Evidence Table；
- `save_claim_graph`：claim 的引用 key、evidence ID、章节 ID 全部机器校验；
- `save_verification_result`：写入支撑关系与风险等级，Host 据此计算硬门槛；
- `quality_gate_check`：纯 Host 逻辑，接收 mode 参数，输出 `pass` / `pass_with_warnings` / `needs_revision` / `needs_human_review` / `blocked`。

关键边界：**「引用存在、claim 有 evidence、evidence 来自 confirmed」由 Host 机器校验，不依赖 LLM 自觉；「证据是否真的支撑论断」由 Editor/verifier 做语义判断，Host 只记录和执行结果。**

存储沿用项目约定：temp + fsync + rename 原子写入、严格 JSON 读取、相对 store 根的正斜杠路径。

### 4.4 质量门控：硬门槛 + 分级风险

硬阻断（Host 强制，所有模式生效）：

- 引用 key 不在 `confirmed.json`；
- claim 没有任何 evidence 绑定；
- evidence 指向不存在的引用；
- 伪造引用 key / 编造来源。

分级风险（verifier 判定 + Host 记录）：

- 证据深度较浅（abstract 级支撑强结论）；
- overstated、措辞过强；
- 章节证据覆盖不足；
- 重复论证。

重写闭环沿用现有规则：最多 2 轮，超限标记 `needs_human_review`，不中断整篇流程。

## 5. 写作模式（三档）

`requirements.json` 新字段：`quality_mode: "fast" | "enhanced" | "strict"`，默认 `enhanced`。

| 模式 | 行为 |
|---|---|
| fast 快速初稿 | 跳过 claim verification 语义判断；保留 Host 硬校验（引用必须 confirmed、不得伪造 key）；质量产物降级为 warnings-only |
| enhanced 质量增强（默认） | 完整闭环：evidence → plan → claim → verification → rewrite；硬门槛阻断底线问题，弱证据进入分级风险 |
| strict 严格证据 | 在 enhanced 基础上收紧：abstract 级证据支撑强结论从 warning 升级为 `needs_revision`；`metadata_only` 证据不允许作为关键论断唯一支撑；`partially_supported` 也触发重写；重写超限一律 `needs_human_review` 并在 report 置顶 |

三档差异**只体现在门控严格度和风险升级规则**，产物结构完全相同，模式切换不影响存储格式和恢复逻辑；`quality_gate_check` 接收 mode 参数。

旧 `requirements.json` 无该字段时：新 run 按 `enhanced` 处理；恢复旧 run 按兼容模式处理（见 6.3）。

## 6. TUI 接入与恢复兼容

### 6.1 Requirements 屏幕

新增模式选择字段（默认 enhanced），落盘到 `requirements.json`。

### 6.2 WritingProgress / ExportSummary

四区布局不重排，在已有区域内增加质量信息：

- 步骤区：显示 `evidence_extraction` / `claim_verification` 等新 step；
- 章节进度：状态增加 `verifying` / `needs_revision` 标记；
- 日志区：显示硬门槛阻断和风险分级事件（不展示完整 evidence 内容）；
- ExportSummary：新增 `final/quality-report.md` 入口和一行整体质量结论。

### 6.3 恢复兼容

StateProbe 增加质量产物探测：

- 旧 run 恢复且缺少 `quality/` 产物 → 兼容模式：继续旧流程完成写作，质量产物缺失记入 report warnings，不阻断、不强制重跑；
- 新 run 中途恢复 → 质量步骤走现有 checkpoint 机制，从最近完成的质量 step 继续；
- RecoverPrompt 文案注明当前 run 的质量模式（三档）。

### 6.4 不改动的部分

- ConfigWizard 不新增质量配置（阈值首版固定默认值，不暴露配置面）；
- References 确认交互不变，仍是硬门槛；
- 现有勿动文件（`internal/tui/requirements`、`internal/tui/references` 等）只做桥接扩展，不重写。

## 7. 测试与三层验收

### 7.1 第一层：自动化结构验收

- `internal/quality` 全部类型的 schema 校验、原子写入、严格 JSON 读取测试；
- 工具层校验测试：引用 key 不在 confirmed → 拒绝；evidence ID 不存在 → 拒绝；claim 无 evidence → 阻断；
- `quality_gate_check` 三种模式门控矩阵测试（同一组输入在 fast / enhanced / strict 下产生不同结论）；
- checkpoint 恢复测试：每个新 step 中断后可从 latest checkpoint 续跑，质量产物哈希校验通过；
- TUI 测试：模式选择落盘、WritingProgress 新事件渲染、ExportSummary 质量结论行、RecoverPrompt 模式文案。

### 7.2 第二层：证据链夹具验收

新增 `fixtures/quality-mini/`，故意埋入坏样本，配套 `internal/e2e/quality_mini_test.go`（全程 mock agent、无网络）：

| 埋入问题 | 期望行为 |
|---|---|
| draft 引用了未确认文献 key | Host 硬阻断 |
| claim 完全没绑定 evidence | Host 硬阻断 |
| 伪造的引用 key | Host 硬阻断 |
| abstract 级证据 + 绝对化结论 | enhanced 下 warning，strict 下 needs_revision |
| 同一论断在两章重复 | 分级风险标记 |
| 重写 2 轮仍不达标 | needs_human_review，流程不中断 |

### 7.3 第三层：短综述 before/after 验收

- 用一组真实材料（可扩充 `fixtures/materials`），分别在兼容旧流程与 enhanced 模式下各跑一次短综述（2-3 章、低字数）；
- 输出对照记录到 `docs/` 验收报告：大纲约束差异、正文论证差异、claim 支撑率、unsupported claim 数量、report 可读性；
- 有真实 API key 时跑真实 provider；无 key 时用 mock 跑结构对照并记录跳过原因（沿用模块 21/22 做法）；
- 主观部分由用户确认，作为最终验收签字项。

### 7.4 验收总线（新增条目）

| 验收项 | 验证方式 |
|---|---|
| 质量产物完整、可恢复、可校验 | 第一层 |
| 底线问题必被硬阻断 | 第二层夹具 |
| 三档模式门控行为正确 | 第一层矩阵 + 第二层夹具 |
| 旧项目恢复不被破坏 | 第一层恢复测试 |
| 用户可感知质量提升 | 第三层 before/after |

## 8. 实现模块拆分（供实现计划参考）

按可独立验收的顺序拆分：

1. Evidence Table 底座（类型、存储、校验、工具）；
2. Section Quality Plan 写前约束；
3. Writer 证据使用协议（claims 绑定 evidence ID）；
4. Claim Graph 写后抽取；
5. Claim Verification 与硬门槛（`quality_gate_check` + 三档模式）；
6. Editor rewrite instructions；
7. Export / report 汇总（quality-report.md + report.md 摘要）；
8. TUI 模式选择、WritingProgress / ExportSummary / RecoverPrompt 增强与恢复兼容；
9. 三层验收夹具与 before/after 验收。

## 9. 明确不做（本阶段边界）

- 不新增重型 Agent 角色（EvidencePlanner / ClaimVerifier 等独立 Agent）；
- 不在 ConfigWizard 暴露质量阈值配置面；
- 不做 OCR / 表格 / 图片 / 公式级证据抽取；
- 不做引用片段精确到页码的强保证（snippet 级即可，depth 如实标记）；
- 不改变 References 人工确认交互与硬门槛语义；
- 不破坏旧项目恢复路径。

## 10. 风险与边界

- 质量步骤会增加 token 成本与运行时长：fast 模式作为降级出口；
- verifier 语义判断的主观性：通过结构化 verification result + 三层验收约束，不确定按不通过处理（沿用 F9 原则）；
- 旧产物兼容：StateProbe 探测 + warnings-only 兼容模式兜底；
- 真实 provider 验收依赖 API key：无 key 时 mock + 记录跳过原因。
