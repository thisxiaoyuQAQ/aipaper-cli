# 04-文献确认TUI

## 模块做啥（1 行）

实现候选文献的 TUI 多选确认流程，并写入 confirmed、rejected 和 BibTeX 产物。

## 依赖谁（1 行）

- 必须先完成：vault/02-材料解析与索引.md、vault/03-学术搜索与文献候选.md
- 可并行：无

## 需要先读哪几个文件（2~5 个）

- 项目备忘录.md
- docs/需求与架构.md「模块职责」「输出目录契约」
- docs/interfaces/references.md
- internal/contracts/types.go
- origindocs/bubbletea.md

## 接口与类型

- `references.LoadCandidates(store.Store)`：读取 `references/candidates.json`。
- `references.ConfirmCandidates(store, candidates, ConfirmationDecision)`：写出 confirmed/rejected/BibTeX。
- `references.ReferenceKey` / `UniqueReferenceKey`：生成稳定 key，冲突追加 A/B 后缀。
- `referencestui.NewModel` / `Model.UpdateKey` / `Model.View` / `Model.Decision`：可测试 TUI model 层，后续可接 Bubble Tea runtime。
- 输入：`references/candidates.json`
- 输出：`references/confirmed.json`、`references/rejected.json`、`references/confirmed.bib`
- 交互键：Space 选择 / 取消，Enter 确认，`/` 搜索，`s` 排序，`a` 全选高相关，`r` 拒绝，`q` 返回或退出。

## 实现要点

- TUI 只负责交互和确认结果落盘，不替用户决定引用。
- confirmed references 生成稳定 key：`firstAuthorYearShortTitle`，冲突追加 A/B 后缀。
- 若用户未确认任何文献，后续写作必须阻塞并返回 `REFERENCE_NONE_CONFIRMED`。
- 列表需要展示标题、作者、年份、来源、DOI/URL、相关性分数和相关性理由。
- 可以先实现 CLI-friendly 的可测试模型层，再接 Bubble Tea view/update。

## 测试要点

- TUI model 单元测试覆盖选择、取消、搜索、排序、拒绝、确认。
- key 生成冲突时稳定。
- confirmed/rejected 写入使用原子写入。
- 未确认任何文献时不进入写作流程。

## 产出清单

- internal/tui/references/
- internal/references/
- docs/interfaces/references.md 更新（如接口变更）
- 对应 `*_test.go`

## 行数预估

- TUI model/view/update 拆分，单文件目标 < 500 行。
