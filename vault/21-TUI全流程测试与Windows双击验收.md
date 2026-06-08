# 21-TUI全流程测试与Windows双击验收

## 模块概述

补齐 TUI 全流程测试和 Windows 双击验收，确保无参数 TUI、带参数 CLI、配置、材料、搜索、文献确认、写作进度、暂停恢复、导出和文档全部一致。

## 前置依赖

- 依赖模块：10-无参数TUI启动与RootModel骨架 至 20-用户文档与README快速入口
- 可并行模块：无

## 最小上下文清单

- 项目备忘录.md
- docs/TUI全流程详细开发流程.md
- docs/开发进度.md
- docs/全量验收报告.md
- internal/e2e/review_mini_test.go
- 所有新增 `internal/tui/*` 测试
- README.md
- docs/user-guide.md

## 接口与类型定义

测试目标覆盖：

```text
无参数入口 -> ConfigWizard -> Requirements -> MaterialsScan -> SearchProgress -> References -> WritingProgress(mock runtime) -> ExportSummary -> Done
```

全量验证命令：

```bash
go test ./...
go build ./cmd/aipaper-cli
```

Windows 构建：

```powershell
go build -o aipaper-cli.exe ./cmd/aipaper-cli
.\aipaper-cli.exe
```

## 实现要求

- 新增或完善 TUI model/root/state probe 单元测试。
- 新增 mock runtime 全流程测试，不依赖真实 LLM 和真实网络。
- 验证无参数启动和有参数 CLI 均正常。
- 验证 ConfigWizard 保存配置后可被 runtime role 解析。
- 验证材料 BibTeX 候选不被搜索覆盖。
- 验证 References 确认是写作硬门槛。
- 验证 WritingProgress streaming delta、usage 缺失、错误和完成状态。
- 验证 Ctrl+C 安全停止和 checkpoint 恢复路径。
- 验证 ExportSummary 输出路径与文档一致。
- 进行 Windows 双击 / 终端启动手动验收，并记录结果；若双击窗口行为受系统限制，需在 user guide 中说明。
- 增加真实 provider 最小手动 smoke：最小 requirements + 小材料 + 1 个确认引用 + 生成/导出；若没有 API key 可跳过，但必须记录跳过原因。

## 测试要求

- `go test ./...` 通过。
- `go build ./cmd/aipaper-cli` 通过。
- Windows 手动验收清单完成：
  - 双击 exe 进入 TUI 或展示可读错误；
  - 终端运行无参数进入 TUI；
  - 带参数 `--help` / `status` 等命令可用；
  - 窄窗口不崩溃；
  - Ctrl+C 恢复提示可见。
- 文档一致性检查通过。

## 任务清单（预期产出）

- `internal/e2e/tui_flow_test.go` 或等价测试
- 新增 TUI 相关单元测试补齐
- 更新 `docs/开发进度.md` TUI 增量任务状态
- 更新或新增全量验收报告
- Windows 双击手动验收记录

## 模块代码行数预估

- E2E 测试拆 helper；单文件过长时拆 `testutil`。
