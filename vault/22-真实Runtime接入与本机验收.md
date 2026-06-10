# 22-真实Runtime接入与本机验收

## 任务目标

在 10-21 自动化 TUI 增量验收完成后，补齐真实运行环境边界：将 WritingProgress 与 Host/AgentRuntime 的真实执行路径打通，接入真实 runtime stop adapter，并完成 Windows 本机双击与真实 provider smoke 验收。

## 背景

全量审计发现 17/18/21 的“已完成”主要覆盖 TUI model、mock runtime、构建与非交互 smoke：

- 17 已完成四区布局、RuntimeEvent model/view 桥接和 mock 事件测试，但真实 Architect/Writer/Editor runner 尚未从 TUI WritingProgress 启动。
- 18 已完成两阶段停止状态机和 RecoverPrompt 交互，但真实 runtime stop adapter 需要随真实 runtime 接入补齐。
- 21 已完成 mock 全流程、构建、文档一致性和非交互 CLI smoke；Windows 桌面双击交互与真实 provider smoke 仍依赖用户本机/API key 条件。

本任务不回退 17/18/21 的自动化验收结论，而是作为后续真实环境验收任务独立执行。

## 前置依赖

- 05-Agent运行时与Coordinator
- 07-StepCheckpoint恢复
- 17-WritingProgress四区布局与Runtime事件桥
- 18-暂停退出与安全恢复
- 21-TUI全流程测试与Windows双击验收

## 需要读取

- `项目备忘录.skill`
- `docs/开发进度.md`
- `docs/TUI全流程增量需求.md`
- `docs/TUI全流程增量架构设计.md`
- `docs/interfaces/tui.md`
- `agent-output/request/全量审计-决策请求.md`
- `internal/app/agent_runtime.go`
- `internal/tui/app/root.go`
- `internal/tui/writing/`
- `internal/e2e/tui_flow_test.go`

## 实现要求

### 1. WritingProgress 启动真实 runtime

- RootModel 进入 `ScreenWriting` 时，应能通过 Host/AgentRuntime 启动真实写作流程。
- WritingProgress 仍只负责展示和用户交互，不直接写死 Coordinator 决策规则。
- runtime 启动失败必须以用户可读错误展示，并保留返回/退出路径。
- 恢复路径 `RecoverPrompt continue` 必须将 `RecoveryPrompt` 注入真实 runtime 启动参数。

### 2. RuntimeEvent 真实投影

- 将真实 runtime/Coordinator 事件投影为 `internal/tui/writing.RuntimeEvent`。
- 至少覆盖：
  - step started/done/failed
  - role log
  - content delta / chapter progress
  - usage update
  - checkpoint saved
  - runtime done/error
- 不得向 UI event fields 传递完整 API key、Authorization header、provider raw request。

### 3. 真实 stop adapter

- WritingProgress 中 Ctrl+C 发出的 stop 请求必须传递给真实 runtime。
- runtime 收到停止请求后，应尽力在安全 checkpoint 后停止。
- checkpoint saved 后 UI 进入可恢复的 canceled 状态。
- 下一次启动应通过 StateProbe/RecoverPrompt 回到可继续路径。

### 4. 真实 provider smoke

- 在具备 API key 时，执行最小真实 provider smoke：
  - 配置可被 `runtimeapp.ResolveRoleRuntime` 解析。
  - TUI 写作流程至少能启动并产生可观测 runtime event。
  - 不要求完整长文生成；可使用短输入和低 token 限制。
- 无 API key 时必须跳过并在报告中记录跳过原因。

### 5. Windows 本机双击验收

- 构建 `aipaper-cli.exe`。
- 在 Windows 本机双击或等效 Explorer 启动场景中确认：
  - 用户能看到可读入口或错误提示。
  - 不会黑窗闪退且无提示。
  - 配置缺失时进入 ConfigWizard 或显示明确配置指引。
- 如果自动化无法覆盖双击，必须输出手动验收清单并由用户确认。

## 测试要求

- 单元测试：
  - RootModel 写作启动注入 runtime starter。
  - RecoverPrompt continue 注入 recovery prompt。
  - Ctrl+C stop 请求调用 runtime stop adapter。
  - runtime event 投影不泄露 secret。
- 集成/冒烟测试：
  - mock runtime 全流程继续通过。
  - 真实 provider smoke 在无 API key 时跳过并记录。
  - Windows exe 构建通过。

## 验收标准

- `go test ./...` 通过。
- `go build ./cmd/aipaper-cli` 通过。
- `go build -o aipaper-cli.exe ./cmd/aipaper-cli` 通过。
- WritingProgress 可启动真实 Host/AgentRuntime 路径，不再仅依赖 mock event。
- Ctrl+C stop 请求可到达真实 runtime stop adapter。
- 恢复路径可使用真实 recovery prompt 继续。
- `docs/全量验收报告.md` 记录真实 provider smoke 和 Windows 双击验收结果或跳过原因。
- 若 Windows 双击需用户手动确认，必须在 `agent-output/request/` 输出确认请求。

## 已知风险/边界

- 真实 provider smoke 可能产生外部 API 调用和费用，执行前必须确认用户授权或检测到明确测试环境。
- Windows 双击交互可能无法在非交互 CI/终端中完全自动化，允许拆为手动验收清单。
- 不得为通过 smoke 放宽 Writer/Editor 只能使用 confirmed references 的硬规则。
