# 24-SectionQualityPlan写前约束

## 任务目标

实现 Section Quality Plan 的类型、存储、校验与 Host 工具，并扩展 Architect 在生成大纲时同期产出每章质量计划。

## 前置依赖

- 23-EvidenceTable底座
- 05-Agent运行时与Coordinator
- 06-写作产物与质量门控

## 需要读取

- `项目备忘录.skill`
- `docs/interfaces/quality.md`
- `docs/superpowers/specs/2026-06-10-quality-engine-design.md`（第 3.2、4.1、4.2 节）
- `internal/quality/`（模块 23 产出）
- `internal/agent/`（Coordinator prompt、step runner、事实工具——勿动文件，只在边界上新增）
- `outline.json` 既有结构

## 实现要求

### 1. 类型与存储

- `SectionQualityPlan`、`SectionPlan` 类型，字段覆盖 spec 3.2：
  - `chapter_id`（与 outline 章节对应）；
  - `questions`（本章要回答的问题）；
  - `required_evidence_ids`（必须存在于 Evidence Table）；
  - `recommended_reference_keys`；
  - `boundaries`（与其他章节边界）；
  - `forbidden_generalizations`（禁止泛化点）；
  - `gaps`、`human_review_hints`。
- 路径：`quality/section-quality-plan.json` + `.md`，原子写入。

### 2. 校验

- 每章 `required_evidence_ids` 必须存在于 Evidence Table，否则结构化错误；
- `chapter_id` 必须与 outline 章节一致。

### 3. Coordinator 步骤与 Architect 扩展

- 新增 step `evidence_extraction`（References 确认后）与 `section_quality_plan`（outline 同期/后），走现有 checkpoint 机制；
- Architect prompt 扩展：提炼 evidence、产出每章 quality plan，通过 Host 工具落盘；
- 不把章节规划规则硬编码进 Host。

## 测试要求

- 类型校验、原子写入、严格读取测试；
- evidence ID 不存在 → 拒绝；
- chapter_id 与 outline 不一致 → 拒绝；
- mock Coordinator 测试：两个新 step 的 checkpoint 写入与恢复续跑。

## 验收标准

- `go test ./...` 通过；
- mock 流程中 `evidence_extraction` → `section_quality_plan` 两个 step 可执行、可恢复；
- 中断后从 latest checkpoint 续跑且产物哈希校验通过。

## 已知风险/边界

- `internal/agent/` 为勿动目录，只在其边界新增 step 与工具注册，不改既有决策语义。

## 行数预估

- `internal/quality/sectionplan.go` ≈ 200 行 + 测试 ≈ 250 行；agent 边界新增 step 注册 ≈ 150 行；单文件 < 500 行。
