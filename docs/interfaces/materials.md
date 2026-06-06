# Materials 材料清单契约

材料目录扫描与解析结果，落盘为 `output/aipaper/materials/manifest.json`，提取文本与元数据分别写入 `materials/extracted/` 与 `materials/parsed/`。

来源：`internal/contracts/types.go` 的 `MaterialManifest` / `MaterialItem`；格式分层见 spec 第 7 节。

## 结构定义

```go
type MaterialManifest struct {
    Items []MaterialItem `json:"items"`
}

type MaterialItem struct {
    ID         string `json:"id"`
    Path       string `json:"path"`
    Kind       string `json:"kind"`
    Status     string `json:"status"`
    Parser     string `json:"parser,omitempty"`
    Degraded   bool   `json:"degraded"`
    OutputText string `json:"output_text,omitempty"`
    OutputMeta string `json:"output_meta,omitempty"`
    Error      string `json:"error,omitempty"`
}
```

## 字段说明

| 字段 | JSON tag | 类型 | 说明 |
| --- | --- | --- | --- |
| ID | `id` | string | 稳定材料 ID，如 `material_001` |
| Path | `path` | string | 原始材料路径 |
| Kind | `kind` | string | 材料类型：`pdf`/`markdown`/`txt`/`bibtex`/`docx`/`url`/`csv` |
| Status | `status` | string | 解析状态：`parsed`/`failed`/`skipped`/`pending` |
| Parser | `parser` | string | 使用的解析器，如 `pdf_text`、`bibtex`（`omitempty`） |
| Degraded | `degraded` | bool | 是否为降级提取（DOCX/URL/CSV 通常为 `true`） |
| OutputText | `output_text` | string | 提取正文相对路径，如 `materials/extracted/material_001.md`（`omitempty`） |
| OutputMeta | `output_meta` | string | 元数据相对路径，如 `materials/parsed/material_001.json`（`omitempty`） |
| Error | `error` | string | 解析失败原因（`omitempty`） |

## manifest.json 示例

```json
{
  "items": [
    {
      "id": "material_001",
      "path": "./materials/paper-a.pdf",
      "kind": "pdf",
      "status": "parsed",
      "parser": "pdf_text",
      "degraded": false,
      "output_text": "materials/extracted/material_001.md",
      "output_meta": "materials/parsed/material_001.json"
    },
    {
      "id": "material_002",
      "path": "./materials/notes.docx",
      "kind": "docx",
      "status": "parsed",
      "parser": "docx_basic",
      "degraded": true,
      "output_text": "materials/extracted/material_002.md"
    }
  ]
}
```

## 格式支持分层（spec 第 7 节）

### 完整支持

| 格式 | Kind | 行为 |
| --- | --- | --- |
| PDF | `pdf` | 提取正文文本；尝试提取标题/作者/年份；提取 DOI/URL；长文本分块；记录页码或段落位置 |
| Markdown / TXT | `markdown` / `txt` | 作为笔记/材料说明；保留标题层级；长文本分块；进入 evidence pool |
| BibTeX | `bibtex` | 解析 reference key，提取 title/author/year/doi/url/journal；直接进入候选文献池，默认标记来自用户材料 |

### 基础 / 降级支持（`degraded=true`）

| 格式 | Kind | 行为 | 限制 |
| --- | --- | --- | --- |
| DOCX | `docx` | 提取纯文本，保留基本标题 | 不承诺复杂表格、批注、脚注 |
| 网页链接 | `url` | 抓取标题/正文摘要/URL；抓取失败保留 URL 待确认 | 非学术页面不默认当文献，除非匹配 DOI/论文元数据 |
| CSV | `csv` | 读取表头与行，作为数据/笔记进入 evidence pool | 不默认生成引用，除非含 DOI/URL/title/author/year |

## 与 checkpoint 关系

材料解析对应 step `parse_materials`，输出 `manifest.json` 及各 extracted/parsed 文件，均记入 checkpoint `outputs` 并带 SHA256。BibTeX 解析出的条目会进一步馈入候选文献池（见 [references.md](./references.md)）。
