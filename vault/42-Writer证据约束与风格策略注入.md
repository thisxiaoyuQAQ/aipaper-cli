# 42-Writer证据约束与风格策略注入

**状态**: 待开发

## 任务目标

将 PaperQualityPolicy 注入 Writer 输入与章节写作 prompt，使正文写作遵守 evidence-grounded writing、证据深度、风格 guard 和“证据缺口不进正文主体”的规则。

## 前置依赖

- 40-PaperQualityPolicy本地策略底座
- 25-Writer证据使用协议

## 需要读取

- `internal/agent/writer_quality.go`
- `internal/app/writer_runner.go`
- `internal/quality/paper_quality_policy.go`

## 实现要求

1. 在 `attachWriterPolicy` 中追加 Paper Quality writer guidance 到 `TemplateGuidance`。
2. 将低质量正文模式追加到 `ForbiddenDraftPatterns`。
3. 在 `WriterLLMRunner.buildPrompt` 中加入 Paper quality writing policy 小节。
4. 不改变 `writerModelOut`、`DraftBundle` 和 Writer Guard schema。

## 测试要求

- `BuildWriterChapterInput` 测试断言新增 guidance / forbidden patterns。
- Writer prompt 测试断言包含 evidence depth、gap 不进正文主体、confirmed references 等规则。
- Writer Guard 现有硬阻断测试继续通过。

## 验收标准

- 运行时 Writer prompt 明确要求 evidence 不足进入 `writer_notes`，不得填充正文主体。
- `go test ./internal/agent ./internal/app` 通过。

## 已知风险/边界

- 不把风格规则变成新的机器硬阻断；首期由 prompt、Editor 和 Gate 共同约束。
