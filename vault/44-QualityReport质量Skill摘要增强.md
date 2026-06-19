# 44-QualityReport质量Skill摘要增强

**状态**: 待开发

## 任务目标

增强 `final/quality-report.md` 的 deterministic 渲染，使用户能看到 Paper Quality Skill policy version、证据深度解释、质量风险说明和人工行动项。

## 前置依赖

- 40-PaperQualityPolicy本地策略底座
- 29-QualityReport导出汇总

## 需要读取

- `internal/export/quality_report.go`
- `internal/quality/gate.go`
- `internal/quality/paper_quality_policy.go`
- `docs/interfaces/paper_quality_skill.md`

## 实现要求

1. Quality Report 显示 `PaperQualityPolicyVersion`。
2. Evidence Depth Distribution 增加解释说明。
3. 对 GateIssue code 增加面向用户的行动解释。
4. Suggested Next Human Edits 区分补证据、降级 claim、删除 unsupported claim、人工复核等动作。
5. 不引入 LLM，不改变 ExportInput/QualityInput schema。

## 测试要求

- `internal/export/quality_report_test.go` 断言 policy version、depth explanation、human action items。
- 兼容模式和质量报告失败不阻塞导出逻辑不退化。

## 验收标准

- `go test ./internal/export` 通过。
- `final/quality-report.md` 能解释论文质量策略触发的风险和下一步。

## 已知风险/边界

- 报告只消费已有 GateOutcome / ClaimGraph / EvidenceTable，不新增语义判断。
