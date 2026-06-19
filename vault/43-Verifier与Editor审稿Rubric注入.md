# 43-Verifier与Editor审稿Rubric注入

**状态**: 待开发

## 任务目标

将 PaperQualityPolicy 注入 `editor_run verify/review`，让 claim verification 和 review 输出体现 claim type/support rubric、审稿维度、rewrite instruction 和 human review triggers。

## 前置依赖

- 40-PaperQualityPolicy本地策略底座
- 27-ClaimVerification与质量门控
- 28-EditorRewriteInstructions

## 需要读取

- `internal/app/editor_runner.go`
- `internal/agent/verification_quality.go`
- `internal/agent/editor_quality.go`
- `internal/quality/paper_quality_policy.go`

## 实现要求

1. `runVerify` prompt 注入 claim type/support/evidence depth rubric。
2. claim type 首期只作为内部判断或写入 `verifier_note`，不新增字段。
3. `buildReviewPrompt` 注入 reviewer-style review、rewrite instruction、human review triggers。
4. 不改变 `ClaimVerdict`、`Review`、`RewriteInstruction` schema。

## 测试要求

- Verify prompt 断言 claim type、evidence sufficiency、uncertain unsupported 规则。
- Review prompt 断言 required rewrite instruction、human review、语言/证据/结构区分规则。
- `GuardReviewInstructions` 现有测试不退化。

## 验收标准

- Unsupported/overstated/high-risk claim 的 review prompt 明确要求 required rewrite instruction。
- `go test ./internal/app ./internal/agent` 通过。

## 已知风险/边界

- 不新增独立 Verifier 角色。
- 不让 Editor 要求 Writer 编造不存在的证据。
