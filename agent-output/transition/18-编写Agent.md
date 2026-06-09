## 交接单

- **来源角色**：编写 Agent
- **目标角色**：审查 Agent
- **所属任务**：vault/18-暂停退出与安全恢复.md
- **涉及文件**：
  - internal/tui/app/recover_prompt.go（已存在，未修改）
  - internal/tui/writing/model.go（修改）
  - internal/tui/writing/events.go（新增消息类型）
  - internal/tui/app/root.go（修改）
  - internal/tui/app/recover_prompt_test.go（新增）
  - internal/tui/writing/stop_test.go（新增）
  - internal/tui/app/exit_confirm_test.go（新增）
  - internal/tui/writing/model_test.go（修改一个测试）

- **变更摘要**：
  1. **WritingProgress 停止功能增强**：
     - 新增 `stopping`、`stopRequested` 状态字段
     - Ctrl+C 时进入 stopping 状态而非立即取消
     - 收到 `EventCheckpointSaved` 事件且 `stopRequested=true` 时才标记为 `canceled`
     - 新增 `Stopping()` 和 `StopRequested()` 方法
     - 新增 `RuntimeStopRequestedMsg` 消息类型

  2. **RootModel 退出确认**：
     - 新增 `exitConfirm` 和 `exitConfirmScreen` 字段
     - 在 Update 中处理退出确认逻辑（y/n/esc）
     - 新增 `shouldConfirmExit()` 方法判断是否需要确认（当前所有屏幕都返回 false，为未来扩展预留）
     - 新增 `currentScreenView()` 辅助方法
     - View 中显示退出确认提示
     - 屏幕转场时清除 `exitConfirm` 状态
     - 处理 `RuntimeStopRequestedMsg` 消息并转发给 Writing model

  3. **RecoverPrompt 功能**：
     - 已存在的 `recover_prompt.go` 实现了恢复提示的核心功能
     - 支持继续写作（c/enter）、重新开始（r + y 确认）、退出（q/ctrl+c）
     - 重新开始需要二次确认，显示明确提示不会删除现有文件

  4. **测试覆盖**：
     - `recover_prompt_test.go`：11 个测试覆盖 RecoverPrompt 的所有交互路径
     - `stop_test.go`：10 个测试覆盖 WritingProgress 的停止功能
     - `exit_confirm_test.go`：11 个测试覆盖 RootModel 的退出确认功能
     - 修复 `model_test.go` 中的一个测试以适应新的停止机制

- **下游 Agent 需额外读取**：
  - 项目备忘录.skill（必读）
  - docs/TUI全流程增量需求.md（3.9 节：暂停、恢复与退出）
  - docs/interfaces/checkpoint.md（恢复机制）
  - internal/checkpoint/checkpoint.go（已在勿动清单，仅阅读不修改）
  - internal/app/recover.go（已在勿动清单，仅阅读不修改）

- **已知风险/待确认项**：
  1. `shouldConfirmExit()` 当前对所有屏幕都返回 false，未来如果需要在特定屏幕（如 Requirements 或 References）检测未保存更改，需要在相应的 model 中新增 `HasChanges()` 方法
  2. WritingProgress 的真实 runtime 接入尚未完成（模块 17 产出为 mock 测试），实际的停止信号传递到 AgentRuntime 的机制需要在真实 runtime 接入时实现
  3. `RuntimeStopRequestedMsg` 当前在 RootModel 中接收并转发，但实际的 runtime stop 调用需要在真实 runtime 接入时补充
  4. 退出确认提示样式较简单，未使用 lipgloss 样式化（与 RecoverPrompt 保持一致即可）

- **设计决策说明**：
  1. **两阶段停止机制**：Ctrl+C 先请求停止（stopping），等待 checkpoint 保存完成后才标记为 canceled，确保进度不丢失
  2. **退出确认预留扩展点**：`shouldConfirmExit()` 当前简化处理（全部返回 false），为未来需求预留接口，不影响当前功能
  3. **RecoverPrompt 已存在**：模块 12 已实现 RecoverPrompt 的核心功能，本模块未修改该文件，仅新增测试覆盖
  4. **测试策略**：对所有关键状态转换和用户交互路径进行单元测试，不依赖真实 runtime，使用状态断言而非私有字段访问

- **构建验证**：
  - ✅ `go build ./...` 成功
  - ✅ `go test ./internal/tui/app/...` 全部通过
  - ✅ `go test ./internal/tui/writing/...` 全部通过
