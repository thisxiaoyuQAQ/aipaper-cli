# 写作页启动后不推进的诊断增强设计

## 范围

本设计修复一个具体 bug：真实运行（双击 `aipaper-cli.exe` 或无参数启动）下，用户走完配置→需求→材料→搜索→文献确认、进入 `WritingProgress`（写作页）后，TUI 能正常渲染四区布局，但**界面完全不动**——无日志、无 token、无章节状态变化，runtime 似乎根本没有推进。

本设计**只增强可观测性与最小兜底**，不重构启动流程、不改 agentcore、不改事件映射。目标受众是本地编码 → 自审 → 测试 → 提交，不覆盖 CI/CD 与生产部署。

## 复现与现象

- 触发路径：无参数运行 / 双击 exe，按向导一路到文献确认并回车，进入 `WritingProgress`。
- 现象：写作页渲染正常，但长时间停留在初始状态（`Initializing` 一类文案），日志区为空，token/进度/章节均无更新。
- 已排除：不是 mock 路径的问题。`TestRootModelPumpsRuntimeEventsIntoWritingModel` 等用例能跑通，因为测试直接往 `rt.msgs` 塞事件；真实路径事件需由后端协程产生。

## 根因拓扑

写作页的所有动态都来自 `RuntimeEventMsg`，由 `RootModel.Update` 收到后重新挂 `runtime.NextEventCmd()` 泵。事件源头链路为：

```text
WritingProgress(写作页)  ←  RuntimeEventMsg  ←  root.Update 重挂 NextEventCmd 泵
        ↑ 停在这里，无事件进入
root.runtime.NextEventCmd  ←  rt.msgs 通道
        ↑ 通道为空
rt.msgs  ←  sendEvent(后端 sink)
        ↑ 无人写入
rt.run 协程: agent.Prompt(kickoff) -> agentcore goroutine -> sink(events)
```

"能进写作页但不动"在拓扑上唯一对应 **`rt.msgs` 通道长期收不到任何事件**。可能卡住的环节（从前往后）：

| # | 环节 | 失败征兆 | 当前可见性 |
|---|------|---------|-----------|
| A | `StartWritingRuntime` 返回 error | 回 `writingRuntimeStartedMsg{err}`，写作页显示错误 | 已处理（`root.go` Update 分支） |
| B | `rt.run` 协程在 `agent.Prompt` 前后 panic / 未启动 | 通道关闭，TUI 永久卡死且无任何提示 | 静默 |
| C | `agent.Prompt` 成功但 agentcore goroutine 静默卡住（真实 LLM 调用挂起、网络无响应、工具卡死） | `WaitForIdle()` 永不返回，无任何事件 | 完全静默 |
| D | coordinator 跑完一轮但未产生 tool 事件即 `agent_end` | round 循环反复 `Continue` 空转，直至 30 轮超时（极慢） | 无进度可见 |
| E | `sendEvent` 在通道满 512 时 drop（`default:` 分支） | 事件被丢，但通常先有部分显示 | 边缘 |

最可能是 C 或 D——真实 LLM / 工具调用阶段卡住，而当前代码在这两个环节**没有任何可观测性**：无日志、无心跳、无"已等待 N 秒"提示。用户只看到静态布局。

## 已确认需求

- 进入写作页后，无论卡在 A/B/C/D/E 哪个环节，TUI 都应给出明确信号，而非静默停留。
- 启动链路的每个关键节点都要可落盘记录，便于真实复现后直接定位卡点。
- 不依赖真实网络即可回归验证（用注入的 fake starter / fake runtime）。
- 不重构启动流程为状态机；不改 agentcore；不改 `ProjectEvent` / `BridgeRunEvent` 的事件映射（已验证正确）。

## 方案选择

采用"后端可观测性 + TUI 兜底提示 + 落盘日志"的增强诊断方案。

- 后端在启动、`Prompt` 成功、每轮 idle、abort、done/error 节点发可读事件与日志。
- 后端加一个心跳 watchdog，即使 agentcore 卡死（C）也能按周期发心跳事件，让 TUI 保持"活着"的迹象并暴露已等待时长。
- TUI 初始文案更明确，并显示"距上次事件已 Ns"的无更新提示。
- 关键节点同步写入可注入的结构化日志，默认落 `output/aipaper/runtime.log`。

选择该方案的原因：直接针对"黑盒静默"这个核心症状，改动集中在 `internal/tui/app` 与 `internal/tui/writing`，不触碰已稳定的事件映射与 agent runtime 边界。

不采用"重构启动流程为状态机"方案，因为当前 bug 的本质是可观测性缺失而非结构错误，重构会扩大风险面。不给真实 LLM 调用加超时，因为网络/模型侧的根因超出"修复响应"范围；watchdog 让卡顿可见即可，由用户决定中止。

## 组件改动

### 后端：`internal/tui/app/runtime_launcher.go`

- `StartWritingRuntime` 在构建好 `rt` 且 `go rt.run` 启动**之前**，向 `rt.msgs` 发一条 `role_log` 事件：「启动写作运行时（model=…，resuming=…）」。确认通道就绪与即将起协程，排除 B（协程未启动）。
- `rt.run` 中 `agent.Prompt(kickoff)` 返回 `nil` 后立即发 `role_log`：「coordinator 已启动」。区分"Prompt 调用未成功返回"这一 C 的子情况。
- 新增心跳 watchdog：独立 goroutine，按固定周期（默认 15s）发 `role_log`：「写作中… 已等待 Ns（round=K）」。`rt.Stop` 与 `run` 退出时停止心跳。即便 agentcore 卡死（C），TUI 也有活信号并暴露卡了多久。
- `WaitForIdle` 前后与每轮 round 的开始/结束、abort、done/error 均通过可选 `diagLogger` 记录一行。

### 新增：`internal/tui/app/runtime_diag.go`（小文件）

- 定义可选、可注入的结构化日志器（默认落 `output/aipaper/runtime.log`，失败回退 stderr）。
- 提供构造函数，launcher 与测试均可注入；测试用 `bytes.Buffer`。
- 只记录启动链路关键节点，不记录每条流式 delta（避免噪音）。

### TUI：`internal/tui/writing/model.go`

- `NewModel` 初始 `phase` 由 `Initializing` 改为「正在启动写作运行时…」；收到首条 `role_log` 后由现有逻辑自然切换为实际 phase。
- 记录"最后一次 `handleRuntimeEvent` 的时间"。`View` 渲染一行：当距上次事件超过阈值（默认 20s）且尚未 done/error 时，显示「已 Ns 无更新，可能正在等待模型响应；Ctrl+C 可保存进度后退出」。该提示与 watchdog 互补：watchdog 保证卡死时仍有心跳，本提示兜底 watchdog 以外（如通道满 drop、事件被过滤）的状态。

### 测试：`internal/tui/app/runtime_launcher_test.go` + writing

- `TestWritingRuntimeEmitsStartLog`：成功启动后 `rt.msgs` 必含一条启动 `role_log`（用注入 fake starter，不依赖网络）。
- `TestWritingRuntimeHeartbeatWhileIdle`：用 fake agent 让 `WaitForIdle` 可控阻塞，断言 watchdog 按周期发心跳事件、`Stop` 后心跳停止。
- writing model 新增纯模型测试：长时间无事件时渲染无更新提示行。

## 数据流

```text
StartWritingRuntime
  ├─ config.Load / role runners / NewAgentRuntime （失败 -> writingRuntimeStartedMsg{err} -> 写作页显示错误）
  ├─ rt.msgs <- role_log「启动写作运行时…」
  ├─ go rt.run(workDir, recoveryPrompt)
  │     ├─ agent.Prompt(kickoff)            -> 失败: RuntimeDoneMsg{Error}
  │     ├─ rt.msgs <- role_log「coordinator 已启动」
  │     └─ for round:
  │          WaitForIdle()   ← 心跳 watchdog 并行发 role_log「已等待 Ns（round=K）」
  │          load progress / endReason switch / Continue or done
  └─ 返回 rt -> writingRuntimeStartedMsg -> root 存 rt, 挂 NextEventCmd 泵
                                   ↓
            后端 sink -> sendEvent -> rt.msgs -> RuntimeEventMsg -> Writing.Update + 重挂泵
                                   ↓
            View: 四区 + 「已 Ns 无更新」提示（超过阈值且未完成）
```

## 错误处理

- 启动失败（A）：沿用现有 `writingRuntimeStartedMsg{err}` → 写作页错误文案，不新增逻辑。
- `Prompt` 失败（B 子情况）：现有 `RuntimeDoneMsg{Error}` 保留；新增"coordinator 已启动"前的 `role_log` 让"Prompt 未成功"可被日志区分。
- agentcore 静默卡住（C）：watchdog 心跳 + TUI 无更新提示共同暴露；用户用 Ctrl+C 触发两阶段 stop（已有逻辑）保存 checkpoint 后退出。
- 空转超时（D）：watchdog 日志显示 round 持续递增却无 tool 事件，定位为模型空转；到达 `maxContinuationRounds` 时沿用现有 `RuntimeDoneMsg{Error}`。
- 通道满 drop（E）：现有 `sendEvent` 的 `default:` drop 保留（丢显示事件优于阻塞 agent 环），由 TUI 无更新提示兜底。
- diagLogger 写入失败：回退 stderr，不阻断主流程。

## 测试

- 单元（不依赖网络）：
  - `TestWritingRuntimeEmitsStartLog`、`TestWritingRuntimeHeartbeatWhileIdle`、`TestWritingRuntimeStopsHeartbeatOnExit`。
  - writing model「无更新提示」纯模型测试。
  - diagLogger 注入 `bytes.Buffer` 的记录断言。
- 手工验收（真实运行，由用户在 Windows 双击复现）：
  1. 走完整向导进入写作页，观察是否出现启动 `role_log` 与（必要时）心跳。
  2. 若仍卡住，查看 `output/aipaper/runtime.log` 最后一条节点，据此判断卡在 Prompt / WaitForIdle / 空转 round。
  3. Ctrl+C 确认能保存 checkpoint 并友好退出。

## 验收标准

- 真实运行进入写作页后，必然先看到一条启动 `role_log`（确认后端协程已起）。
- agentcore 卡死时，TUI 不再静默——出现心跳 `role_log` 与"已 Ns 无更新"提示之一或两者。
- `output/aipaper/runtime.log` 记录启动链路关键节点，足以在复现后直接定位卡点。
- 新增/既有单元测试全绿；不改变 mock 路径下的事件泵行为（`TestRootModelPumpsRuntimeEventsIntoWritingModel` 仍通过）。
- 不触碰 `ProjectEvent` / `BridgeRunEvent` 映射、不改 agent runtime 边界、不改 agentcore。

## 不做

- 不重构启动流程为状态机。
- 不给真实 LLM 调用加超时或重试策略。
- 不改 `internal/agent`、`internal/app`（agent runtime / coordinator / role runner）的行为。
- 不记录每条流式 delta，避免日志噪音。
