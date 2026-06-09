## 交接单

- **来源角色**：编写 Agent
- **目标角色**：审查 Agent
- **所属任务**：vault/17-WritingProgress四区布局与Runtime事件桥.md
- **涉及文件**：
  - `internal/tui/writing/events.go`（新增）
  - `internal/tui/writing/model.go`（新增）
  - `internal/tui/writing/view.go`（新增）
  - `internal/tui/writing/model_test.go`（新增）
  - `internal/tui/app/root.go`（修改）
  - `internal/tui/app/root_writing_test.go`（新增）

- **变更摘要**：
  1. 新增 `internal/tui/writing` 包，实现 WritingProgress 四区布局屏幕
  2. 定义 `RuntimeEvent` 类型和 `BridgeRunEvent` 函数，隔离 agentcore/litellm 原始事件
  3. 实现 WritingProgress Model，支持消费 12 种运行时事件（step started/done/failed、role log、content delta、usage update、chapter status、quality review、checkpoint saved、export artifact、runtime done/error）
  4. 实现四区布局 View：左侧指标、中上日志、中下流式正文、右侧章节进度
  5. 支持窄终端降级为纵向堆叠布局
  6. 在 RootModel 中接入 WritingProgress 屏幕：添加 Writing 字段、newWritingModel 和 updateWriting 函数
  7. 支持从 References 屏幕成功确认后转场到 WritingProgress
  8. 支持从 RecoverPrompt 恢复场景转场到 WritingProgress（携带 RecoveryPrompt）
  9. 完成 mock RuntimeEvent 测试，覆盖 streaming delta、usage、章节状态、进度计算、错误处理

- **下游 Agent 需额外读取**：
  - `项目备忘录.skill`（必读）
  - `docs/TUI全流程增量需求.md`（WritingProgress 需求）
  - `docs/TUI全流程增量架构设计.md`（架构约束）
  - `docs/interfaces/tui.md`（RuntimeEvent 契约）
  - `docs/interfaces/agent.md`（Agent 协作契约）
  - `docs/interfaces/artifacts.md`（章节状态机）
  - `internal/app/agent_runtime.go`（Host runtime 边界）
  - `internal/agent/events.go`（事件投影逻辑）

- **已知风险/待确认项**：
  1. **真实 AgentRuntime 接入尚未完成**：当前 WritingProgress 的 `Init()` 返回 nil，未启动真实 LLM runtime。vault 文件明确要求"必须把真实 Architect/Writer/Editor runner 接入 Host 可启动路径作为验收项"，这部分需要在后续步骤或独立任务中完成。
  2. **RuntimeEvent 桥接仅处理基础事件**：当前 `BridgeRunEvent` 只处理 tool_exec_start/end、agent_end、error 等基础事件类型，未处理 content_delta、usage_update、chapter_status、quality_review 等专用事件（这些需要从 Host 或 Coordinator 通过自定义事件投影实现）。
  3. **usage 缺失时显示 `--`**：已实现，但未测试真实 provider 缺 usage 场景。
  4. **小终端降级布局**：已实现 renderNarrowLayout，但未在真实小终端中验证渲染效果。
  5. **Ctrl+C 安全停止**：Model 中已实现 canceled 标志，但未实现向 runtime 发送停止信号的机制（需要在 Host 层实现）。
  6. **ExportSummary 转场**：当前 WritingProgress Done 后直接 quit，模块 19 实现 ExportSummary 后需修改转场逻辑。

- **测试覆盖**：
  - ✅ WritingProgress Model 创建和初始化
  - ✅ RuntimeEvent 处理（step started/done/failed、role log）
  - ✅ ContentDelta 追加和章节切换
  - ✅ UsageUpdate 更新 token/cost/model
  - ✅ ChapterStatus 更新和进度计算
  - ✅ RuntimeDone/RuntimeError 终止状态
  - ✅ 键盘输入处理（Ctrl+C、Space、↑/↓）
  - ✅ BridgeRunEvent 事件映射
  - ✅ RootModel WritingProgress 集成
  - ✅ 辅助函数（estimateWordCount、splitLines、formatDuration）
  - ✅ 全部构建通过（`go build ./...`）
  - ✅ 全部测试通过（`go test ./internal/tui/writing/...`、`go test ./internal/tui/app/...`）

- **未测试场景**（需在后续集成测试中覆盖）：
  - 真实 AgentRuntime 事件序列驱动 UI 更新
  - 真实 streaming delta 从 LLM 到 TUI 的端到端流程
  - 真实 usage 数据格式和缺失字段处理
  - 窄终端（<60 字符）的实际渲染效果
  - Ctrl+C 向 runtime 发送停止信号
  - WritingProgress → ExportSummary 转场（模块 19 依赖）
