# 14-MaterialsScan与候选材料入口

## 模块概述

实现 MaterialsScan 屏幕：自动扫描 `materials/`，调用现有材料解析管道，展示空目录、扫描中、完成、详情状态，并保留 BibTeX 候选供搜索后合并。

## 前置依赖

- 依赖模块：02-材料解析与索引、13-Requirements屏幕桥接与落盘
- 可并行模块：无

## 最小上下文清单

- 项目备忘录.md
- docs/TUI全流程增量需求.md
- docs/interfaces/materials.md
- docs/interfaces/references.md
- internal/materials/materials.go
- internal/materials/parsers.go
- internal/references/bibtex.go
- internal/store/paths.go

## 接口与类型定义

材料扫描入口：

```go
func ProcessDir(materialDir string, s store.Store) (Result, error)
```

候选文献摘录：

```go
type ReferenceCandidate struct {
    ID              string   `json:"id"`
    Title           string   `json:"title"`
    Authors         []string `json:"authors"`
    Year            int      `json:"year,omitempty"`
    Source          string   `json:"source"`
    DOI             string   `json:"doi,omitempty"`
    URL             string   `json:"url,omitempty"`
    Abstract        string   `json:"abstract,omitempty"`
    Venue           string   `json:"venue,omitempty"`
    CitationCount   int      `json:"citation_count,omitempty"`
    RelevanceScore  float64  `json:"relevance_score,omitempty"`
    RelevanceReason string   `json:"relevance_reason,omitempty"`
    DedupeGroup     string   `json:"dedupe_group,omitempty"`
    Status          string   `json:"status"`
}
```

## 实现要求

- 新增 `internal/tui/materials` 包，提供 model/view/update 和异步扫描 command。
- 若 `materials/` 不存在，创建目录并提示用户放入材料；用户确认后重新扫描。
- 调用 `materials.ProcessDir`，不要在 TUI 内重复实现解析或格式规则。
- 展示统计：成功、降级、失败、跳过数量。
- Details 状态展示每个 material 的简要元信息和失败原因。
- 单文件失败不阻塞转场；全部失败时提供重试、跳过、返回 Requirements。
- `Result.Candidates` 中的 BibTeX 候选必须在 RootModel 或上下文中保留，供 15 合并。

## 测试要求

- 缺目录时创建并提示。
- 空目录展示 Empty 状态。
- Markdown/TXT/BibTeX fixture 扫描成功。
- DOCX/URL/CSV 降级状态可展示。
- 单文件失败不阻塞；全部失败路径可重试/跳过。
- BibTeX 候选在转场数据中保留。

## 任务清单（预期产出）

- `internal/tui/materials/model.go`
- `internal/tui/materials/view.go`
- `internal/tui/materials/model_test.go`
- RootModel 接入 MaterialsScan 转场

## 模块代码行数预估

- model/view/command 分离；避免把解析逻辑复制进 TUI。
