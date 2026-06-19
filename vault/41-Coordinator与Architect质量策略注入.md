# 41-Coordinator与Architect质量策略注入

**状态**: 待开发

## 任务目标

将 PaperQualityPolicy 注入 Coordinator 和 Architect 三个任务，使运行时从编排、大纲、证据提炼和章节计划开始执行论文质量策略。

## 前置依赖

- 40-PaperQualityPolicy本地策略底座
- 24-SectionQualityPlan写前约束

## 需要读取

- `internal/agent/prompt.go`
- `internal/agent/quality.go`
- `internal/app/architect_runner.go`
- `internal/quality/paper_quality_policy.go`

## 实现要求

1. `CoordinatorSystemPrompt` 追加 Paper Quality 全局规则。
2. `runOutline` 注入 narrative / thesis / chapter role 规则。
3. `runEvidenceExtraction` 注入 evidence depth honesty 规则。
4. `runSectionQualityPlan` 注入 SectionPlan rubric。
5. 不改变 `RoleTools`、checkpoint step、EvidenceTable/SectionQualityPlan schema。

## 测试要求

- `internal/agent` prompt 测试断言 policy version 和核心规则进入 Coordinator prompt。
- `internal/app` role runner 测试断言 Architect 三类 prompt 包含 narrative、evidence depth、SectionPlan rubric。

## 验收标准

- Architect 生成的 prompt 能明确要求章节有单一职责、证据深度诚实、forbidden_generalizations 和 gaps 不可编造。
- 既有 Architect runner 流程测试不退化。

## 已知风险/边界

- 不要求 outline JSON 新增 thesis 字段，避免 schema 变更。
