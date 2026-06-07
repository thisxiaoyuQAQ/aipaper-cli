# 05-Agent运行时与Coordinator

## 模块做啥（1 行）

接入 agentcore 和 litellm，建立 Host 工具注册、事件投影、Coordinator 主流程和子 Agent 调用边界。

## 依赖谁（1 行）

- 必须先完成：vault/01-基础脚手架与配置Store.md、vault/04-文献确认TUI.md、vault/07-StepCheckpoint恢复.md
- 可并行：vault/06-写作产物与质量门控.md 的契约细化

## 需要先读哪几个文件（2~5 个）

- 项目备忘录.md
- docs/需求与架构.md「模块职责」「系统结构」
- origindocs/agentcore.md
- origindocs/litellm.md
- internal/config/config.go

## 接口与类型

- Host 输入：合并后的 `config.Config`、Store、恢复事实。
- Coordinator 可用工具组：requirements、materials、search、references、artifacts、checkpoint、export。
- 子 Agent：Architect、Writer、Editor，各自隔离 context，通过 Store artifacts 协作。
- 第一版实际工具名：`requirements_read`、`progress_read`、`references_confirmed_read`、`checkpoint_validate_latest`、`writer_run`。
- 工具统一返回 `{ok:true,data}` 或 `{ok:false,error:{code,message,retryable,details}}`。
- 当前错误码：`AGENT_INVALID_JSON`、`REFERENCE_NONE_CONFIRMED`、`AGENT_TOOL_FAILED`。
- 运行时入口：`app.NewAgentRuntime`，测试可注入 mock `agentcore.ChatModel` 和 mock Writer runner。

## 实现要点

- Host 只加载配置、创建 model/Agent、注册工具、投影事件、处理中断和恢复。
- Coordinator 是唯一流程决策者；Host 不维护硬编码 scheduler。
- 工具只返回事实 JSON，不夹带“下一步应该做什么”。
- Coordinator 系统 prompt 必须显式包含引用硬规则、质量门控、checkpoint 规则和不可重复操作。
- 从 latest checkpoint 恢复时，Host 生成恢复 prompt，列出已完成 step、下一预期 step、可读 artifact、禁止重复的写入。
- 事件流需要能给 TUI 展示当前 agent、工具调用、章节、重写轮次和错误。

## 测试要点

- 使用 mock LLM / 固定响应测试 Coordinator 状态机。
- 验证无 confirmed references 时不会调用 Writer。
- 验证 agent 输出非法 JSON 时返回结构化写作错误。
- 验证 Host 不根据章节评分写死重写判断，重写决策来自 Coordinator 对 Editor 事实的处理。

## 产出清单

- internal/agent/
- internal/app/agent_runtime.go
- docs/interfaces/agent.md
- 对应 `*_test.go`

## 行数预估

- prompt、工具桥接、运行时分别拆文件；单文件目标 < 500 行。
