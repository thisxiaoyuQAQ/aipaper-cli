# 26-ClaimGraph写后抽取

## 任务目标

实现 Claim Graph 的类型、存储、机器校验与 `claim_extraction` 步骤：每章 Writer 完成后，将正文论断抽取为结构化 claim 节点。

## 前置依赖

- 25-Writer证据使用协议

## 需要读取

- `项目备忘录.skill`
- `docs/interfaces/quality.md`
- `docs/superpowers/specs/2026-06-10-quality-engine-design.md`（第 3.3、4.1 节）
- `internal/quality/`
- `internal/artifacts/`（claims.json、citation_map.json）

## 实现要求

### 1. 类型与存储

- `ClaimGraph`、`ClaimNode` 类型，字段覆盖 spec 3.3：
  - `id`（`claim_001` 风格）；
  - `text`、`chapter_id`；
  - `reference_keys`（机器校验存在于 confirmed.json）；
  - `evidence_ids`（机器校验存在于 Evidence Table）；
  - `support`：`supported` / `partially_supported` / `unsupported` / `overstated` / `skipped`（fast 模式）；
  - `risk_level`、`verifier_note`、`needs_rewrite`、`needs_human_review`。
- 路径：`quality/claim-graph.json` + `.md`，原子写入；
- Claim Graph 按章增量更新（章节级 merge，不整体覆盖），支持恢复。

### 2. claim_extraction 步骤

- 新增 step `claim_extraction`（每章 Writer 完成后），走 checkpoint；
- 从该章 `claims.json` + `citation_map.json` 投影生成/更新 ClaimNode；
- 抽取时 support 字段初始为空（待 verification）或 `skipped`（fast 模式）；
- 跨章重复论断检测：文本相似的 claim 标记重复风险（分级风险，不阻断）。

### 3. Host 工具

- `save_claim_graph`：全部引用 key、evidence ID、chapter_id 机器校验。

## 测试要求

- 类型校验、按章 merge、原子写入测试；
- 引用 key / evidence ID / chapter_id 无效 → 拒绝；
- 跨章重复论断 → 风险标记；
- checkpoint 中断恢复测试。

## 验收标准

- `go test ./...` 通过；
- mock 流程中每章完成后 Claim Graph 增量更新且可恢复。

## 已知风险/边界

- 重复检测首版用简单文本规整 + 相似度阈值，不引入向量库。

## 行数预估

- `internal/quality/claimgraph.go` ≈ 280 行（含按章 merge 与重复检测）+ 测试 ≈ 350 行；单文件 < 500 行。
