# 15-SearchProgress与候选合并

## 模块概述

实现 SearchProgress 屏幕：根据 requirements 执行学术搜索，展示 provider 错误，并将搜索候选与材料 BibTeX 候选合并、去重、重新分配 ID 后写入最终 candidates 文件。

## 前置依赖

- 依赖模块：03-学术搜索与文献候选、14-MaterialsScan与候选材料入口
- 可并行模块：无

## 最小上下文清单

- 项目备忘录.md
- docs/TUI全流程增量需求.md
- docs/interfaces/search.md
- docs/interfaces/references.md
- internal/search/run.go
- internal/search/types.go
- internal/references/dedupe.go
- internal/store/paths.go

## 接口与类型定义

搜索入口：

```go
func Run(ctx context.Context, s store.Store, opts Options) (Result, error)
func QueryFromRequirements(req contracts.Requirements, limit int) Query
```

候选去重工具：

```go
func DedupeCandidates(candidates []contracts.ReferenceCandidate) []contracts.ReferenceCandidate
func AssignCandidateIDs(candidates []contracts.ReferenceCandidate, startIndex int) []contracts.ReferenceCandidate
func CandidateDedupeGroup(candidate contracts.ReferenceCandidate) string
```

## 实现要求

- 新增 `internal/tui/search` 包，提供 SearchProgress model/view/update。
- 根据 `requirements.allow_online_search` 决定是否调用 `search.Run`。
- provider 单点失败不阻塞；在 UI 中展示 `Result.Errors`。
- 所有 provider 失败时提供重试、跳过搜索、返回材料步骤。
- 搜索完成后合并：`materialsResult.Candidates + searchResult.Candidates.Items`。
- 使用 `references.DedupeCandidates` 去重，再用 `AssignCandidateIDs(..., 1)` 重新分配连续 ID。
- 写入最终 `references/candidates.json` 和 `references/candidates.md`。
- `candidates.md` 的格式应复用 search/references 既有 writer；若当前无公共 helper，需新增公共 helper 和测试，不在 TUI 内手写漂移格式。
- 注意：现有 `search.Run` 会写 candidates 文件，合并后必须覆盖为最终合并结果。

## 测试要求

- 搜索关闭时，BibTeX 候选仍进入最终 candidates。
- 搜索开启时，材料候选和搜索候选都保留。
- DOI/URL/title 重复时只保留一条且字段更完整。
- provider 部分失败不阻塞。
- provider 全失败时可重试/跳过/返回。
- 最终 ID 从 `cand_001` 连续。

## 任务清单（预期产出）

- `internal/tui/search/model.go`
- `internal/tui/search/view.go`
- `internal/tui/search/model_test.go`
- 候选合并 helper 及测试
- RootModel 接入 SearchProgress 转场到 References

## 模块代码行数预估

- 搜索 UI 与候选合并 helper 分离；单文件目标 < 500 行。
