# 28-EditorRewriteInstructions

## 任务目标

扩展 Editor 评审产物：在 review 中新增结构化 rewrite instructions，逐条指明改什么、用哪条 evidence，驱动 Writer 真正改好。

## 前置依赖

- 27-ClaimVerification与质量门控

## 需要读取

- `项目备忘录.md`
- `docs/superpowers/specs/2026-06-10-quality-engine-design.md`（第 4.2 节）
- `internal/artifacts/`（review.json 既有结构）
- `internal/agent/`（Editor prompt）

## 实现要求

### 1. review.json 扩展

- 新增 `rewrite_instructions` 数组，每条：
  - `claim_id`（可选，关联 Claim Graph）；
  - `location`（章节/段落定位）；
  - `problem`（unsupported / overstated / 泛化 / 重复 / 弱证据等）；
  - `instruction`（具体怎么改）；
  - `suggested_evidence_ids`（建议使用的 evidence）；
  - `severity`（required / optional，对接既有 required/optional fixes）。
- 向后兼容旧 review.json。

### 2. Editor prompt 扩展

- 评审时输入 Claim Graph + verification result + 本章 quality plan；
- 必须对每个 needs_rewrite 的 claim 给出至少一条 rewrite instruction；
- 重写轮中 Writer 输入包含上一轮 rewrite instructions。

### 3. Host 校验

- `suggested_evidence_ids` 必须存在于 Evidence Table；
- required 级 instruction 未被覆盖时，重写后章节不能直接 pass（进入下一轮或 needs_human_review）。

## 测试要求

- review 扩展字段 schema 与兼容读取测试；
- suggested_evidence_ids 无效 → 拒绝；
- mock 重写闭环测试：instruction → Writer 重写 → 再验证；
- 超限 2 轮 → needs_human_review。

## 验收标准

- `go test ./...` 通过；
- mock 流程中重写轮能携带结构化 instructions 并收敛或正确超限。

## 已知风险/边界

- 不改变重写上限 2 轮的既有规则。
