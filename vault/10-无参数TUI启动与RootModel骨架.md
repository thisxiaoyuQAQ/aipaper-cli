# 10-无参数TUI启动与RootModel骨架

## 模块概述

建立下一阶段 TUI 的入口和根状态机骨架：无参数运行进入 Bubble Tea TUI，带参数继续走现有 CLI；新增 RootModel、Screen 枚举、转场消息和 program 启动封装。

## 前置依赖

- 依赖模块：01-基础脚手架与配置Store
- 可并行模块：无

## 最小上下文清单

- 项目备忘录.md
- docs/TUI全流程增量需求.md
- docs/TUI全流程增量架构设计.md
- docs/interfaces/tui.md
- cmd/aipaper-cli/main.go
- internal/cli/cli.go
- internal/store/paths.go

## 接口与类型定义

从 `docs/interfaces/tui.md` 摘录：

```go
type Screen string

const (
    ScreenConfigWizard   Screen = "config_wizard"
    ScreenRecoverPrompt  Screen = "recover_prompt"
    ScreenRequirements   Screen = "requirements"
    ScreenMaterialsScan  Screen = "materials_scan"
    ScreenSearchProgress Screen = "search_progress"
    ScreenReferences     Screen = "references"
    ScreenWriting        Screen = "writing_progress"
    ScreenExportSummary  Screen = "export_summary"
    ScreenDone           Screen = "done"
)

type ScreenTransitionMsg struct {
    Next Screen
    Data any
}
```

现有 CLI 入口需保持：

```go
func Run(ctx context.Context, args []string, stdout, stderr io.Writer) int
```

## 实现要求

- `cmd/aipaper-cli/main.go` 中实现：`len(os.Args)==1` 启动 TUI；`len(os.Args)>1` 调用 `cli.Run`。
- 新增 `internal/tui/app` 包，至少包含：
  - `Screen` 枚举；
  - `ScreenTransitionMsg`；
  - `RootModel`；
  - `RunProgram(ctx, opts)` 或等价启动函数；
  - 可注入的 program runner，便于测试 main 分流。
- RootModel 先实现最小可启动骨架和错误兜底，不要求本模块完成所有屏幕。
- 子屏幕通过 transition 请求切换；RootModel 统一初始化下一屏。
- TUI 启动失败时恢复终端状态，向 stderr 输出简短错误并返回非零。
- 禁止改变 `internal/cli.Run` 对现有子命令的行为。

## 测试要求

- 覆盖无参数路径调用 TUI runner。
- 覆盖有参数路径仍调用 `cli.Run`，`init/status/recover/config` 测试不回退。
- 覆盖 TUI runner 返回错误时 main/helper 返回非零并写 stderr。
- RootModel 接收 `ScreenTransitionMsg` 后切换 `CurrentScreen`。

## 任务清单（预期产出）

- `internal/tui/app/root.go`
- `internal/tui/app/program.go`
- `internal/tui/app/root_test.go`
- `cmd/aipaper-cli/main.go` 分流调整
- 启动分流测试 helper 或等价测试

## 模块代码行数预估

- 单文件目标 < 500 行。
- RootModel 骨架和 program 封装分文件，避免入口文件膨胀。
