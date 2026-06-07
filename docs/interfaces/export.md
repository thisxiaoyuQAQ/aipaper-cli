# Export 交付产物契约

最终交付目录 `output/aipaper/final/` 的产物契约。导出对应 checkpoint step `export_docx`，可重复执行并记录导出版本。

来源：spec 第 4 节输出目录、第 7 节来源追踪；首版实现位于 `internal/export/`。

## 1. final/ 产物清单

| 文件 | 说明 |
| --- | --- |
| `paper.md` | Markdown 主稿，由各章 `accepted.md` 汇总 |
| `paper.docx` | Docx 交付初稿（不承诺复杂排版；转换失败归类 `EXPORT_DOCX_FAILED`，可重试） |
| `references.md` | 参考文献 Markdown（按 `citation_style` 格式化；格式化失败不阻塞正文，但在报告列出） |
| `citation-trace.json` | 引用追踪文件，claim → reference 全链路 |
| `report.md` | 完成报告：质量、风险、需人工复核项 |

> 参考文献同时存在机器可读的 `references/confirmed.bib`（BibTeX）。

## 2. citation-trace.json 字段

记录每段正文的引用来源与验证状态（spec 第 7 节「来源追踪」）：

```json
{
  "version": "export-20260607T120000Z",
  "generated_at": "2026-06-07T12:00:00Z",
  "items": [
    {
      "chapter_id": "ch01",
      "paragraph_id": "ch01_p003",
      "claim_id": "ch01_claim_001",
      "reference_key": "zhang2024GenerativeAssessment",
      "source_type": "academic_search",
      "editor_verified": true,
      "needs_human_review": false
    }
  ]
}
```

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `chapter_id` | string | 章节 ID |
| `paragraph_id` | string | 段落 ID（对应 `citation-map-vN.json` 的 `paragraph_id`） |
| `claim_id` | string | 关键论断 ID（对应 `claims-vN.json`） |
| `reference_key` | string | 确认文献 key（对应 `confirmed.json`） |
| `source_type` | string | 来源类型：`user_material`（用户材料）/ `academic_search`（学术搜索）/ `bibtex`（BibTeX） |
| `editor_verified` | bool | Editor 是否验证该文献支持该论断 |
| `needs_human_review` | bool | 是否需人工复核 |

Go 类型位于 `internal/export`：

- `CitationTrace`
- `CitationTraceItem`
- `ExportInput`
- `ChapterInput`
- `Result`
- `Issue`

## 3. 生成关系

`citation-trace.json` 由各章 `citation-map-vN.json`（段落↔claim↔reference）、`claims-vN.json`（论断重要性/置信度）、`review-vN.json`（`unsupported_claims`、`passed`）与 `confirmed.json`（来源类型）合成：

- `source_type` 来自 `ConfirmedReference` 的来源：`source_material_ids` 含 `bib` → `bibtex`；`source_material_ids` 非空 → `user_material`；否则 → `academic_search`。
- `editor_verified`：该 claim 不在最新 review 的 `unsupported_claims` 中即为 `true`。
- `needs_human_review`：所属章节状态为 `needs_human_review`、review 未通过，或该 claim 属 unsupported 时为 `true`。

## 4. report.md 内容要点

- 最终 Markdown / Docx / 参考文献 / 质量报告路径；
- 需人工复核的章节列表（状态 `needs_human_review`）；
- unsupported claim 统计；
- 引用格式化失败项（不阻塞但需告知）；
- 总耗时与成本估算（取自 `run.json` 的 `cost_estimate`）。

## 5. 导出规则

- 导出幂等可重复，但记录导出版本；不覆盖已通过章节的中间 artifact。
- `paper.md` 仅汇总 `accepted.md`（已 `committed` 章节）。
- Docx 转换失败时降级：保留 `paper.md`、`references.md`、`citation-trace.json` 与 `report.md`，在 `report.md` 标注 Docx 导出失败，并清理旧的 `final/paper.docx` 以避免 stale 文件误导。
- final 目录固定文件采用覆盖写入；章节 draft/claims/review 等中间 artifact 不会被导出模块覆盖。

## 6. Go API 与错误码

```go
func LoadInput(s store.Store) (ExportInput, error)
func ExportFinal(s store.Store, input ExportInput, opts Options) (Result, error)
```

- `LoadInput`：读取 `requirements.json`、`run.json`、`outline/outline.json`、`references/confirmed.json` 与 `drafts/*/accepted.md`，通过 `accepted.md` 与 `draft-vN.md` 内容哈希匹配对应版本，再读取同版本 claims、citation map 和 review。
- `ExportFinal`：写出 `final/paper.md`、`final/references.md`、`final/citation-trace.json`、`final/paper.docx`、`final/report.md`；返回可写入 checkpoint 的 `OutputArtifact` 列表。
- `Options.DocxExporter`：可替换 Docx exporter；默认 `SimpleDocxExporter` 使用标准库生成基础 OOXML Docx。

| Code | 含义 |
| --- | --- |
| `EXPORT_DOCX_FAILED` | Docx exporter 失败，Markdown/trace/report 继续生成 |
| `EXPORT_NO_ACCEPTED_CHAPTERS` | 未找到可导出的 accepted 章节 |
| `EXPORT_UNCONFIRMED_REFERENCE` | claims 或 citation map 出现未确认 reference key，导出中止 |
| `EXPORT_REFERENCE_FORMAT_WARNING` | confirmed reference 元数据不完整，references.md 用 fallback 格式并在报告列出 |
