# Agent Runtime 与 Coordinator 设计

## 范围

本设计覆盖 `05-Agent运行时与Coordinator` 的第一版实现。目标是建立可测试的 Host/Coordinator 运行时骨架，接入 `agentcore` 与 `litellm`，但不实现完整正文生成、质量评分和最终导出；这些留给后续 `06` 与 `08`。

## 方案选择

采用“可测试运行时骨架”方案：

- Host 负责配置加载后的模型创建、Agent 构造、工具注册、事件投影、恢复 prompt 注入。
- Coordinator 是唯一流程决策者，基于工具返回的事实 JSON 选择下一步。
- Writer / Architect / Editor 先作为隔离的角色边界和工具调用目标存在，具体产物写入在 `06` 细化。
- 所有 runtime 依赖都通过小接口包住，测试使用 mock model / scripted coordinator，不触发真实网络请求。

不采用完整写作循环，因为 `06` 的产物和质量门控还未落地；也不采用纯 mock，因为本模块明确要求接入 agentcore/litellm。

## 组件

### internal/agent

- 定义 Coordinator system prompt，显式包含引用硬规则、质量门控、checkpoint 规则、恢复不可重复操作。
- 定义角色名、事件视图、结构化错误和 Coordinator 输出解析。
- 提供测试用 scripted model，覆盖固定响应和非法 JSON。

### internal/app 或 internal/host

- 根据 `config.Config` 解析默认角色与 provider/model。
- 创建 litellm / agentcore 模型适配器。
- 构造 Coordinator Agent，注册工具桥接。
- 将 agentcore 事件投影为 TUI 可消费的运行事件。
- 恢复时复用 `app.Recover` 产出的 `RecoveryPrompt`。

### 工具桥接

第一版只注册已存在且稳定的事实工具：

- 读取 `requirements.json`、`progress.json`。
- 读取 `references/confirmed.json` 并返回 confirmed 数量。
- 校验 latest checkpoint。
- 后续 artifacts/export 工具只保留接口占位，不在 05 中写正文。

工具返回事实 JSON，不返回“下一步应该做什么”。

## 数据流

1. CLI / TUI 调用 Host。
2. Host 加载 config 与 Store，必要时生成恢复 prompt。
3. Host 创建 Coordinator Agent 并注册工具。
4. Coordinator 调用事实工具读取 Store。
5. Coordinator 基于事实决定下一步；Host 只执行工具调用并投影事件。
6. 事件进入 `run.json.events` 或 TUI 观察层。

## 错误处理

- 配置缺 provider/model 或 provider 未配置时返回配置错误。
- LLM / Agent 输出非法 JSON 时返回结构化写作错误 `AGENT_INVALID_JSON`。
- 无 confirmed references 时 Coordinator 不应调用 Writer；若模型试图调用，工具桥接返回事实错误，不替 Coordinator 做调度判断。
- Host 不根据 Editor 分数写死重写判断；它只把 Editor 事实交回 Coordinator。
- API key 不进入事件、日志或测试断言输出。

## 测试

- mock LLM 固定响应驱动 Coordinator 最小状态机。
- confirmed references 为空时，Writer 调用计数保持 0。
- 非法 JSON 输出返回 `AGENT_INVALID_JSON`。
- Editor review 未通过时，Host 不直接做重写决策，只投影事实事件。
- Host 构造测试不访问真实网络。
- 保持 `go test ./...` 全量通过。
