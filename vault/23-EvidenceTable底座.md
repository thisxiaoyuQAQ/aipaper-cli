# 23-EvidenceTable底座

## 任务目标

新建 `internal/quality` 包，落地 Evidence Table 的类型、存储、机器校验与 Host 工具，作为 Quality Engine 的证据底座。

## 前置依赖

- 01-基础脚手架与配置Store
- 02-材料解析与索引
- 04-文献确认TUI

## 需要读取

- `项目备忘录.skill`
- `docs/superpowers/specs/2026-06-10-quality-engine-design.md`（第 3.1、4.3 节）
- `docs/interfaces/_index.md`
- `internal/store/`、`internal/contracts/`
- `internal/artifacts/`（参考既有产物写入与校验模式）
- `references/confirmed.json` 的读取接口

## 实现要求

### 1. 类型定义

- `EvidenceTable`、`Evidence` 类型，字段覆盖 spec 3.1：
  - `id`（`ev_001` 风格，沿用项目 ID 策略）；
  - `reference_key`（必须存在于 confirmed.json）；
  - `material_id`（可选，来自用户材料时关联）；
  - `depth`：`metadata_only` / `abstract` / `snippet` / `fulltext_excerpt`；
  - `topics`、`key_findings`、`method`、`subjects`、`limitations`；
  - `excerpt`（可引用片段，snippet 级以上才有）；
  - `confidence`、`coverage`、`risk_flags`。
- 时间字段 RFC3339 UTC，路径相对 store 根正斜杠。

### 2. 存储与校验

- 路径：`quality/evidence-table.json` + `quality/evidence-table.md`；
- temp + fsync + rename 原子写入，严格 JSON 读取；
- 写入校验：`reference_key` 必须存在于 `references/confirmed.json`，否则结构化错误 `{ok:false,error:{code:"evidence_unconfirmed_reference",...}}`；
- depth 渐进规则：无全文解析文本时不允许 `snippet` / `fulltext_excerpt`（需材料 extracted 文本存在性校验）。

### 3. Host 工具

- 注册 `save_evidence_table` / `load_evidence_table` 工具，供 Coordinator/Architect 调用；
- 工具失败返回结构化错误，不抛自然语言。

## 测试要求

- 类型 schema 校验、原子写入、严格读取单元测试；
- `reference_key` 不在 confirmed → 拒绝；
- depth 与材料全文存在性不符 → 拒绝；
- Markdown 渲染快照测试。

## 验收标准

- `go test ./internal/quality/...` 通过；
- `go build ./...` 通过；
- Evidence Table 可写入、可读取、可被后续模块加载；
- 文档：`docs/interfaces/quality.md` 新增类型与工具说明。

## 已知风险/边界

- 不做 OCR/表格/公式级证据；
- 不保证页码级定位，snippet 级即可。

## 行数预估

- `internal/quality/evidence.go` ≈ 250 行、`evidence_test.go` ≈ 300 行、工具注册 ≈ 100 行；单文件 < 500 行。
