# Export 交付产物契约

最终交付目录 `output/aipaper/final/` 的产物契约。导出对应 checkpoint step `export_docx`，可重复执行并记录导出版本。

来源：spec 第 4 节输出目录、第 7 节来源追踪（首版无独立 Go 结构体，按下列约定 JSON / 文件落盘）。

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

## 3. 生成关系

`citation-trace.json` 由各章 `citation-map-vN.json`（段落↔claim↔reference）、`claims-vN.json`（论断重要性/置信度）、`review-vN.json`（`unsupported_claims`、`passed`）与 `confirmed.json`（来源类型）合成：

- `source_type` 来自 `ConfirmedReference` 的来源（`source_material_ids` 非空 → 用户材料或 BibTeX；否则学术搜索）。
- `editor_verified`：该 claim 不在最新 review 的 `unsupported_claims` 中即为 `true`。
- `needs_human_review`：所属章节状态为 `needs_human_review`，或该 claim 属 unsupported 时为 `true`。

## 4. report.md 内容要点

- 最终 Markdown / Docx / 参考文献 / 质量报告路径；
- 需人工复核的章节列表（状态 `needs_human_review`）；
- unsupported claim 统计；
- 引用格式化失败项（不阻塞但需告知）；
- 总耗时与成本估算（取自 `run.json` 的 `cost_estimate`）。

## 5. 导出规则

- 导出幂等可重复，但记录导出版本；不覆盖已通过章节的中间 artifact。
- `paper.md` 仅汇总 `accepted.md`（已 `committed` 章节）。
- Docx 转换失败时降级：保留 `paper.md` 与 `references.md`，在 `report.md` 标注 Docx 导出失败。
