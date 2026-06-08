# 19-ExportSummary与完成页

## 模块概述

实现写作完成后的导出摘要和完成页：调用既有 export 接口生成 final artifacts，展示输出目录、文件列表、docx 降级和常见下一步提示。

## 前置依赖

- 依赖模块：08-最终导出与报告、17-WritingProgress四区布局与Runtime事件桥
- 可并行模块：18-暂停退出与安全恢复

## 最小上下文清单

- 项目备忘录.md
- docs/TUI全流程增量需求.md
- docs/interfaces/export.md
- docs/interfaces/references.md
- internal/export/load.go
- internal/export/export.go
- internal/export/docx.go
- internal/store/paths.go

## 接口与类型定义

既有导出入口：

```go
func LoadInput(s store.Store) (ExportInput, error)
func ExportFinal(s store.Store, input ExportInput, opts Options) (Result, error)
```

当前 final 输出契约：

```text
output/aipaper/final/paper.md
output/aipaper/final/paper.docx
output/aipaper/final/references.md
output/aipaper/final/citation-trace.json
output/aipaper/final/report.md
```

确认阶段 BibTeX 输出：

```text
output/aipaper/references/confirmed.bib
```

## 实现要求

- 新增 `internal/tui/exportsummary` 和 `internal/tui/done` 包，或在 app 中实现轻量完成页。
- ExportSummary 调用 `export.LoadInput` / `export.ExportFinal`，不要自行拼接 final 内容。
- 展示 `Result.Outputs` 和 `Result.Issues`。
- Docx 降级时明确提示 Markdown/report 已成功生成。
- 未确认引用、缺 accepted chapter、导出失败时展示明确错误并允许重试或返回。
- 输出文件说明必须与实际实现一致；BibTeX 当前为 `references/confirmed.bib`，如需 `final/references.bib` 必须新增实现和测试。
- Done 页展示输出目录、文件列表、恢复/状态/配置高级命令提示。

## 测试要求

- 成功导出后展示 final 输出。
- docx exporter 失败时展示降级 issue，paper.md/report.md 仍可见。
- 缺 confirmed refs 时不生成错误文件，提示明确。
- Done 页路径和文件名与 `ExportFinal` Result 一致。

## 任务清单（预期产出）

- `internal/tui/exportsummary/model.go`
- `internal/tui/exportsummary/view.go`
- `internal/tui/exportsummary/model_test.go`
- `internal/tui/done/model.go`（可选）
- RootModel 接入 ExportSummary -> Done

## 模块代码行数预估

- 导出摘要逻辑应薄封装，主要复用 export 包。
