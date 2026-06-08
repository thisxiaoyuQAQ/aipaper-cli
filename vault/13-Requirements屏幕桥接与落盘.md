# 13-Requirements屏幕桥接与落盘

## 模块概述

把已稳定的 `internal/tui/requirements` model 接入 RootModel，完成需求填写、校验、落盘和转场，不重写字段规则。

## 前置依赖

- 依赖模块：10-无参数TUI启动与RootModel骨架、12-启动状态探测与恢复入口；既有 `internal/tui/requirements` 与 Store 能力（01/09 已验收产物）
- 可并行模块：无

## 最小上下文清单

- 项目备忘录.md
- docs/TUI全流程增量需求.md
- docs/interfaces/requirements.md
- internal/tui/requirements/model.go
- internal/tui/requirements/view.go
- internal/store/paths.go
- internal/store/atomic.go

## 接口与类型定义

从 `docs/interfaces/requirements.md` 摘录：

```go
type Requirements struct {
    Topic              string   `json:"topic"`
    ResearchQuestions  []string `json:"research_questions"`
    Scope              string   `json:"scope"`
    Language           string   `json:"language"`
    CitationStyle      string   `json:"citation_style"`
    TargetWords        int      `json:"target_words"`
    MaterialDir        string   `json:"material_dir"`
    AllowOnlineSearch  bool     `json:"allow_online_search"`
    SearchProviders    []string `json:"search_providers,omitempty"`
    ChapterPreferences []string `json:"chapter_preferences"`
    Constraints        []string `json:"constraints"`
}
```

既有 model 入口：

```go
func NewModel(defaults contracts.Requirements) Model
func (m Model) UpdateKey(key string) Model
func (m Model) Requirements() (contracts.Requirements, error)
func Validate(req contracts.Requirements) error
func (m Model) View() string
```

## 实现要求

- RootModel 负责把 Bubble Tea key event 映射到 `requirements.UpdateKey`。
- 提交成功后使用 `store.WriteJSON` 写入 `requirements.json`。
- 默认 material dir 使用 `./materials`；为避免现有 Validate 因目录不存在失败，TUI 可在进入或提交前创建默认目录。
- 提交失败时停留 Requirements 屏并展示错误。
- 成功后转场到 `MaterialsScan`。
- 不修改 requirements 校验规则，除非同步更新 tests 和 docs。

## 测试要求

- 现有 `internal/tui/requirements` 测试不回退。
- RootModel 级测试：输入有效需求后写入 Store 正确 JSON。
- 缺必填字段时不写入、不转场。
- 默认 `./materials` 不存在时，TUI 路径能创建或引导，不造成不可理解失败。

## 任务清单（预期产出）

- RootModel 中 Requirements screen adapter
- `internal/tui/app/requirements_bridge_test.go` 或等价测试
- 必要的默认材料目录创建 helper

## 模块代码行数预估

- 以桥接为主，不应新增大型表单实现。
