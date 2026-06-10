# 25-Writer证据使用协议

## 任务目标

扩展 Writer 的输入与产物协议：写章节时输入本章 Section Quality Plan 与相关 evidence，`claims.json` 中每个 claim 必须绑定 evidence ID。

## 前置依赖

- 24-SectionQualityPlan写前约束
- 06-写作产物与质量门控

## 需要读取

- `项目备忘录.md`
- `docs/superpowers/specs/2026-06-10-quality-engine-design.md`（第 4.2、4.4 节）
- `internal/artifacts/`（claims.json 既有结构——勿动目录，边界上扩展）
- `internal/agent/`（Writer prompt 与 writer guard）

## 实现要求

### 1. claims.json 扩展

- claim 条目新增 `evidence_ids` 字段（必填，至少 1 个）；
- 保持向后兼容：旧 run 的 claims.json 无该字段时按兼容模式读取（不阻断，记 warning）。

### 2. Writer prompt 与输入

- Writer 每章输入新增：本章 quality plan + 该章 required evidence 的内容；
- prompt 明确：关键论断必须引用 quality plan 绑定的 evidence；材料不足显式标 gap，禁止编造来源（沿用 F7 原则）。

### 3. Host 硬校验（writer guard 扩展）

- claim 的 `evidence_ids` 必须存在于 Evidence Table；
- claim 的引用 key 必须存在于 confirmed.json（既有规则保持）；
- 校验失败返回结构化错误并阻断该章产物写入。

## 测试要求

- claims 扩展字段 schema 与兼容读取测试；
- evidence_ids 缺失/不存在 → 阻断；
- 伪造引用 key → 阻断（回归既有 guard）；
- mock Writer 流程测试：quality plan 注入与 claims 落盘。

## 验收标准

- `go test ./...` 通过；
- mock 流程中 Writer 产物 claims 全部绑定有效 evidence；
- 旧格式 claims.json 仍可读取，不破坏恢复。

## 已知风险/边界

- 不得为通过测试放宽「只能使用 confirmed references」硬规则；
- `internal/artifacts/` 只在边界扩展字段与校验，不重写产物规则。
