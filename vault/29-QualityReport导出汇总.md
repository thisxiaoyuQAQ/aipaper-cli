# 29-QualityReport导出汇总

## 任务目标

实现 `final/quality-report.md` 导出与现有 `report.md` 的质量摘要集成。

## 前置依赖

- 27-ClaimVerification与质量门控
- 28-EditorRewriteInstructions
- 08-最终导出与报告

## 需要读取

- `项目备忘录.md`
- `docs/superpowers/specs/2026-06-10-quality-engine-design.md`（第 3.4 节）
- `internal/export/`（勿动目录，通过既有 LoadInput/ExportFinal 边界扩展）

## 实现要求

### 1. quality-report.md

- 路径：`final/quality-report.md`，内容覆盖 spec 3.4：
  - 整体质量状态；
  - 硬门槛通过/失败清单；
  - 证据深度分布（按 depth 统计）；
  - unsupported / overstated claims 列表；
  - needs_human_review 章节（strict 模式置顶）；
  - 重写摘要（每章轮次与收敛情况）；
  - 用户下一步人工修改建议。

### 2. report.md 质量摘要

- 现有 report.md 增加质量摘要 section，链接到 quality-report.md；
- 兼容模式（旧 run 无质量产物）：report 中记录「质量产物缺失」warnings，不阻断导出。

### 3. 导出集成

- `export.LoadInput` 扩展加载 quality 产物（可缺失）；
- `ExportFinal` 在质量产物存在时生成 quality-report.md；
- 引用格式化失败不阻塞正文（沿用 F10），质量报告生成失败同样不阻塞 paper.md/paper.docx，但必须记入 report。

## 测试要求

- quality-report.md 渲染快照测试（含各 depth 分布、blocked、needs_human_review 用例）；
- 兼容模式导出测试：无 quality 产物时正常导出 + warnings；
- 导出失败不阻塞主产物测试。

## 验收标准

- `go test ./...` 通过；
- mock 全流程导出产物包含 quality-report.md 且 report.md 有质量摘要；
- 旧 fixture（review-mini）导出不回归。

## 已知风险/边界

- `internal/export/` 勿动：通过 LoadInput/ExportFinal 既有接口边界扩展，不重写导出规则。
