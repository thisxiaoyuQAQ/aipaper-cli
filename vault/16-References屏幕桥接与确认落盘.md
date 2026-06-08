# 16-References屏幕桥接与确认落盘

## 模块概述

把已稳定的 `internal/tui/references` model 接入 RootModel，读取最终候选文献，完成用户确认、拒绝、BibTeX 输出，并阻止未确认文献进入写作。

## 前置依赖

- 依赖模块：04-文献确认TUI、15-SearchProgress与候选合并
- 可并行模块：无

## 最小上下文清单

- 项目备忘录.md
- docs/TUI全流程增量需求.md
- docs/interfaces/references.md
- internal/tui/references/model.go
- internal/tui/references/view.go
- internal/references/confirm.go
- internal/references/dedupe.go

## 接口与类型定义

既有确认入口：

```go
type ConfirmationDecision struct {
    ConfirmedIDs []string
    RejectedIDs  []string
    ConfirmedAt  time.Time
}

type ConfirmationResult struct {
    Confirmed contracts.ConfirmedReferences
    Rejected  contracts.ReferenceCandidates
    Outputs   []string
}

func LoadCandidates(s store.Store) (contracts.ReferenceCandidates, error)
func ConfirmCandidates(s store.Store, candidates contracts.ReferenceCandidates, decision ConfirmationDecision) (ConfirmationResult, error)
```

既有 TUI model：

```go
func NewModel(candidates contracts.ReferenceCandidates) Model
func (m Model) UpdateKey(key string) Model
func (m Model) View() string
func (m Model) Decision(now time.Time) references.ConfirmationDecision
```

## 实现要求

- RootModel 初始化 References 屏时调用 `references.LoadCandidates`。
- 使用既有 `internal/tui/references` model 处理选择、搜索、排序、拒绝、确认。
- 提交确认时调用 `references.ConfirmCandidates`。
- 未确认任何文献时展示错误并停留或返回，不进入 WritingProgress。
- 候选为空时提供返回 SearchProgress、返回 MaterialsScan、退出路径。
- 成功后转场到 WritingProgress。
- 保持 `references/confirmed.json` 是 Writer/Editor 唯一可用文献来源。

## 测试要求

- 现有 references model 测试不回退。
- 确认后写入 `references/confirmed.json`、`references/rejected.json`、`references/confirmed.bib`。
- 未选择任何文献时返回 `REFERENCE_NONE_CONFIRMED` 并阻塞写作。
- key 冲突处理保持稳定。
- 空 candidates 路径可返回前一步。

## 任务清单（预期产出）

- RootModel 中 References adapter
- `internal/tui/app/references_bridge_test.go` 或等价测试
- 空候选 / 无确认错误展示

## 模块代码行数预估

- 以桥接为主，不新增重复确认逻辑。
