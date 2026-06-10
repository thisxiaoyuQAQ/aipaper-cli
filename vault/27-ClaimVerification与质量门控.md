# 27-ClaimVerification与质量门控

## 任务目标

实现 `claim_verification` 步骤与 `quality_gate_check` Host 逻辑：Editor/verifier 对 claim 做支撑语义判断，Host 按三档模式执行硬门槛与分级风险。

## 前置依赖

- 26-ClaimGraph写后抽取

## 需要读取

- `项目备忘录.skill`
- `docs/interfaces/quality.md`
- `docs/superpowers/specs/2026-06-10-quality-engine-design.md`（第 4.4、5 节）
- `internal/quality/`、`internal/agent/`

## 实现要求

### 1. claim_verification 步骤

- 新增 step `claim_verification`（claim 抽取后、Editor 评审前），走 checkpoint；
- Editor（扩展为 verifier）对每个 claim 判断 `supported` / `partially_supported` / `unsupported` / `overstated`，写入 verifier_note；
- 不确定按不通过处理（沿用 F9 原则）；
- fast 模式跳过本 step 的语义判断，support 标记 `skipped`，Host 硬校验仍执行。

### 2. quality_gate_check（纯 Host 逻辑）

- 输入：Claim Graph + verification result + `quality_mode`；
- 输出：`pass` / `pass_with_warnings` / `needs_revision` / `needs_human_review` / `blocked`；
- 硬阻断（所有模式）：引用 key 不在 confirmed、claim 无 evidence 绑定、evidence 指向不存在引用、伪造 key；
- 分级规则按模式：
  - enhanced：abstract 级证据支撑强结论 → warning；`unsupported` → needs_revision；
  - strict：abstract 级支撑强结论 → needs_revision；`metadata_only` 不可作关键论断唯一支撑；`partially_supported` 也触发重写；超限一律 needs_human_review 并 report 置顶；
  - fast：分级风险全部降为 warnings-only。
- 门控阈值首版固定默认值，不暴露配置。

### 3. 与既有质量门控集成

- 与 `internal/artifacts` 既有章节门控（总分 ≥80、引用一致性 ≥90）并联：任一 blocked 即阻断；
- 重写闭环沿用最多 2 轮，超限 needs_human_review 不中断全流程。

## 测试要求

- 门控矩阵测试：同一组输入在 fast / enhanced / strict 下产生 spec 规定的不同结论；
- 硬阻断四类底线问题逐项测试；
- 重写超限 → needs_human_review 不中断；
- checkpoint 恢复测试。

## 验收标准

- `go test ./...` 通过；
- 门控矩阵与 spec 第 5 节表格一一对应；
- mock 流程完整跑通 claim_extraction → claim_verification → gate。

## 已知风险/边界

- verifier 语义判断主观性：靠结构化 verification result 与夹具验收约束，不在本模块解决全部边界。

## 行数预估

- `internal/quality/gate.go` ≈ 300 行（三档矩阵）+ 测试 ≈ 400 行；verification step 接入 ≈ 150 行；单文件 < 500 行。
