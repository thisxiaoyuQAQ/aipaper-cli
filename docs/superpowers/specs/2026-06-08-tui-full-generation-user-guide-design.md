# TUI 全流程生成与用户文档设计

## 范围

本设计覆盖 aipaper-cli 下一阶段工作：让 Windows 用户双击 `aipaper-cli.exe` 即可进入 TUI，在 TUI 内完成 Provider 配置、写作需求填写、材料扫描、文献确认、真实 LLM 写作、最终导出，并补充面向最终用户的使用文档。

本阶段在已有 MVP（01-09 模块已验收）上增量实现，不重写已经稳定的 `internal/checkpoint`、`internal/agent`、`internal/artifacts`、`internal/export`、`internal/tui/requirements` 和 `internal/tui/references` 规则。

## 已确认需求

- 无参数运行或双击 `aipaper-cli.exe` 时直接进入 TUI。
- 保留现有 CLI 子命令；只有带参数时才走 `init/status/recover/config` 等命令。
- 首次运行自动进入 Provider 配置向导，生成现有配置体系使用的 JSON 配置文件。
- 配置向导采用“预设模板 + 必要字段 + 可选高级项”的混合体验。
- Provider 模板包含 OpenAI、Anthropic、本地 Ollama、Custom 四类。
- OpenAI 默认模型为 `gpt-5.5`。
- Anthropic 默认模型为 `claude-opus-4-8`。
- 不做 API 测试；保存配置后直接进入需求填写。
- TUI 写作进度屏采用四区布局：左侧指标、中上日志、中下流式生成内容、右侧进度。
- 生成内容必须流式实时显示。
- 用户文档需要覆盖安装、首次配置、材料准备、生成流程、输出文件、中断恢复、常见问题和高级命令。

## 方案选择

采用“单一 Bubble Tea Program + RootModel 状态机切换屏幕”方案。

- `cmd/aipaper-cli/main.go` 在无参数时启动交互式 TUI。
- `internal/cli` 保留现有命令模式；带参数时继续调用 `cli.Run`。
- `internal/tui` 新增根模型和屏幕编排层。
- RootModel 持有全局状态（workDir、store、config、当前 run 状态）和当前屏幕 model。
- 各屏幕通过显式 transition message 切换，不让子屏幕直接修改全局流程。

选择该方案的原因：用户体验最直接，双击后一路向下完成生成；同时可以复用现有 requirements/references model，避免重写已稳定的校验和确认规则。

不采用多 Program 链式启动，因为每步退出再进入会割裂体验。不采用主菜单优先方案，因为当前目标是降低首次使用门槛；未来可以在 RootModel 前增加“继续 / 新建 / 配置管理”的启动页。

## 屏幕流程

TUI 主流程如下：

```text
ConfigWizard
  -> Requirements
  -> MaterialsScan
  -> AcademicSearch/SearchProgress
  -> References
  -> WritingProgress
  -> ExportSummary
  -> Done
```

启动时 RootModel 自动检测当前状态并进入合适屏幕：

1. 配置缺失、provider 为空或 role 映射缺失：进入 ConfigWizard。
2. 存在未完成 checkpoint：显示恢复提示，用户选择继续或重新开始。
3. 缺 requirements：进入 Requirements。
4. 缺材料解析结果或用户请求重扫：进入 MaterialsScan。
5. 缺 confirmed references：进入 References；若候选文献尚未生成，先进入 SearchProgress。
6. 缺完整写作产物：进入 WritingProgress。
7. 缺最终导出：进入 ExportSummary 并执行导出。
8. 全部完成：显示完成摘要。

屏幕切换使用明确消息：

```go
type ScreenTransitionMsg struct {
    Next Screen
    Data any
}
```

RootModel 负责接收 transition 并初始化下一屏所需数据。

## 默认启动与 CLI 兼容

`main.go` 的行为调整为：

```text
len(os.Args) == 1  -> 启动 TUI
len(os.Args) > 1   -> 调用 cli.Run，保留现有命令
```

这样满足：

- Windows 用户可双击 exe 进入 TUI。
- 终端用户可运行 `./aipaper-cli` 或 `./aipaper-cli.exe` 进入 TUI。
- 高级用户和测试仍可使用 `init/status/recover/config`。

TUI 启动失败时应恢复终端状态，并输出简短错误信息，不吞掉 panic。

## ConfigWizard 设计

### 触发条件

启动时加载现有 `internal/config` 配置体系：

- 全局路径：`~/.aipaper/config.json`
- 项目路径：`aipaper.json`
- 可选额外 config path 仍由 CLI 子命令处理

无参数 TUI 模式优先写项目配置 `aipaper.json`，避免无意污染用户全局配置。后续可在配置向导中提供“保存到全局 / 当前项目”的高级选项；第一版默认当前项目。

若配置缺少以下任一项，则进入向导：

- 至少一个 provider
- `architect`、`writer`、`editor` 三个 role 映射
- role 指向的 provider 存在
- role 或全局模型可解析

### 模板

配置向导提供四个模板：

1. OpenAI
   - type: `openai`
   - base_url: `https://api.openai.com/v1`
   - model: `gpt-5.5`
   - 必填：API Key
2. Anthropic
   - type: `anthropic`
   - base_url: `https://api.anthropic.com`
   - model: `claude-opus-4-8`
   - 必填：API Key
3. Ollama
   - type: `ollama`
   - base_url: `http://localhost:11434`
   - model: `llama3`
   - API Key 可为空
4. Custom
   - type、provider name、base_url、model、api_key 均由用户填写

保存时使用现有 `config.Config`、`ProviderConfig`、`RoleConfig` 结构，不引入第二套 provider JSON 格式。

示例结构：

```json
{
  "provider": "default",
  "model": "claude-opus-4-8",
  "providers": {
    "default": {
      "type": "anthropic",
      "api_key": "sk-ant-...",
      "base_url": "https://api.anthropic.com",
      "models": ["claude-opus-4-8"]
    }
  },
  "roles": {
    "architect": {"provider": "default", "model": "claude-opus-4-8"},
    "writer": {"provider": "default", "model": "claude-opus-4-8"},
    "editor": {"provider": "default", "model": "claude-opus-4-8"}
  }
}
```

### 交互

配置向导分三步：

1. 选择模板。
2. 填写必要字段。
3. 显示配置摘要并保存。

不进行 API 测试。保存失败时停留在当前屏幕，提供重试、返回修改、退出。

API Key 在 UI、日志和错误中必须脱敏显示，只显示前缀和尾部少量字符。

### Claude 兼容注意事项

Anthropic 模板默认 `claude-opus-4-8`。配置向导不主动写入 temperature、top_p、top_k 等采样参数，避免与 Claude Opus 4.8 请求面不兼容。若现有 `RoleConfig.Temp` 有默认值，runtime 在调用 Claude Opus 4.8 时应避免发送不兼容参数，或由 litellm 适配层处理。

长输出走 streaming，以支持内容实时显示并降低长请求超时风险。

## Requirements 屏幕

复用现有 `internal/tui/requirements` model、view、validation，不重写需求字段校验规则。

RootModel 只负责：

- 将完成后的需求写入现有 Store 约定路径。
- 将成功事件投影为 transition。
- 将失败写入错误状态并提示用户重试。

若后续需要“返回修改需求”，由 RootModel 重新加载已保存 requirements 初始化该 model。

## MaterialsScan 屏幕

### 行为

Requirements 完成后自动扫描当前项目的 `materials/` 目录。若目录不存在，则创建目录并提示用户放入材料。

用户确认：放入目录中的文件全部扫描，不提供手动选择部分文件的能力。

支持格式沿用现有 materials 模块：PDF、Markdown、TXT、BibTeX；DOCX、URL、CSV 走降级支持。

### 状态

MaterialsScan 包含：

- Empty：没有找到材料。
- Scanning：后台扫描中，显示当前文件和进度。
- Done：展示成功、降级、失败数量。
- Details：可查看每个 material 的简要元信息和失败原因。

单个文件失败不阻塞流程。全部失败时提示用户重新放入材料或跳过。

### 实现边界

屏幕只调用现有 `internal/materials` 管道，不重复实现解析规则。扫描结果写入现有 manifest/extracted/parsed 产物路径。

## AcademicSearch / SearchProgress

材料扫描后执行学术搜索，生成候选文献供 References 屏幕确认。

如果搜索耗时短，可以作为 MaterialsScan 后的短暂进度态；如果实现中需要明确进度或错误展示，则独立成 SearchProgress 屏幕。

错误处理：

- 单个搜索 provider 失败不阻塞。
- 所有 provider 失败时，用户可重试、跳过搜索或返回材料步骤。
- 跳过搜索时仍可使用 BibTeX 或已有材料上下文继续，但 References 屏应明确候选不足。

## References 屏幕

复用现有 `internal/tui/references` model、view、key 冲突处理、confirmed/rejected/BibTeX 写入规则。

RootModel 只负责初始化候选列表、接收确认完成事件、切换到 WritingProgress。

Writer 和 Editor 仍必须只使用 `references/confirmed.json`，保持现有 artifacts 校验硬规则。

## WritingProgress 四区布局

### 总体布局

写作进度屏采用 dashboard 风格四区布局：

```text
┌──────────────┬────────────────────────────────────┬──────────────┐
│ 左侧指标      │ 中上：实时日志                       │ 右侧进度      │
│              │                                    │              │
│ 运行态        │ [14:32] Coordinator ...            │ 阶段 [4/5]    │
│ 阶段          │ [14:33] Architect ...              │ 章节列表      │
│ 完成比例      │ [14:35] Writer ...                 │ 总进度条      │
│ 字数          │                                    │ 引用一致性    │
│ 模型          ├────────────────────────────────────┤              │
│ Context       │ 中下：生成内容流式预览               │              │
│ Tokens        │                                    │              │
│ Cost          │ ## 当前章节                          │              │
│ Elapsed       │ 正在生成的正文实时追加...             │              │
└──────────────┴────────────────────────────────────┴──────────────┘
```

### 左侧指标

左侧显示：

- 运行态：Initializing / Running / Paused / Failed / Done
- 阶段：Architect / Writer / Editor / Export
- 已完成比例：例如 `3/5 章`、`60%`
- 当前总字数
- 当前模型
- Context：当前上下文 tokens / 模型最大 context
- Token 消耗：输入、输出、cache read、cache creation（有则显示）
- Cost 估算：provider 可计算时显示，否则 `--`
- Elapsed：运行耗时

若 provider 或 litellm 未返回 usage，则 token/cost 显示 `--`，不阻塞写作。

### 中上日志

中上日志区显示 Coordinator、Architect、Writer、Editor、Export 的事件：

- 时间戳
- 来源角色
- 简短消息

日志默认自动滚动到底部。用户手动滚动后暂停自动滚动，按 End 或切回最新恢复。

### 中下内容

中下区实时流式显示当前正在生成的内容：

- Writer streaming delta 追加到当前章节预览。
- 切换章节时更新标题和内容。
- Editor 触发重写时标注 rewrite round，并在内容区显示重写后的最新版本。
- 内容区支持滚动；Tab 在日志区和内容区之间切换焦点。

### 右侧进度

右侧显示：

- 当前主阶段 `[4/5] 写作`
- 章节列表和状态：待写作、写作中、评审中、完成、重写中、人工复核
- 每章评分和重写次数
- 总进度条
- 引用一致性和最终字数

### 响应式布局

- 左右栏固定宽度，中间栏自适应。
- 终端宽度不足时降级为纵向堆叠，并提示用户放大窗口。
- 不因窗口过小崩溃。

## 真实 LLM 运行时接入

本阶段复用现有 `internal/app/agent_runtime.go`、`internal/agent`、`internal/artifacts`、`internal/export` 边界。

Host 负责：

- 根据 `config.Config` 解析 provider/model/role。
- 创建 litellm / agentcore 模型适配器。
- 注册现有事实工具和写作产物工具。
- 将 runtime 事件转换为 TUI 可消费事件。
- 把 checkpoint 恢复 prompt 注入 Coordinator。

Coordinator 仍是唯一流程决策者。Host 不根据 UI 状态硬编码写作顺序，也不绕过 Coordinator 直接调用 Writer/Editor。

事件通道需要覆盖：

- step start / done / failed
- role log
- streaming content delta
- usage update
- chapter status update
- quality review result
- checkpoint saved
- export artifact path

TUI 通过 channel 消费这些事件并更新四区布局。

## 暂停、恢复与退出

写作中按 Ctrl+C 不直接强杀进程，而是：

1. 请求 runtime 在安全点停止。
2. 保存 checkpoint。
3. 显示“进度已保存，下次启动可继续”。
4. 退出 TUI。

下次双击 exe 时检测到未完成 run，显示恢复提示：

1. 继续写作。
2. 重新开始（需二次确认，会清空或归档已有 run 产物）。
3. 退出。

表单页或确认页按 Ctrl+C 显示退出确认；未保存改动应提示用户。

## 错误处理

所有可恢复错误都在 TUI 中展示，不直接崩溃。

- 配置保存失败：提示路径和原因，允许重试或返回修改。
- API 401/403：提示 API Key 或权限可能错误，允许返回 ConfigWizard 修改。
- API 429/529/5xx：提示限流或服务异常，保存 checkpoint，允许稍后恢复。
- context 过长：提示材料或上下文过大，不允许静默截断；应提供返回材料步骤、减少输入或使用摘要策略的说明。
- 搜索失败：可重试、跳过或返回材料步骤。
- 导出失败：保留写作产物，允许重试导出。

API Key 不进入日志、事件文件、report 或错误详情。

## ExportSummary

WritingProgress 完成后进入 ExportSummary。该屏幕调用现有 `internal/export` 的 `LoadInput` / `ExportFinal`，生成：

- `paper.md`
- `paper.docx`
- `references.bib`
- `citation_trace.json`
- `report.md`

若 docx 导出降级，明确提示用户 Markdown 已成功生成，docx 需要依赖或格式支持补充。

导出完成后展示输出目录和文件列表，提示用户按 Enter 打开输出路径（若实现可安全支持）或按任意键退出。

## 用户文档

新增面向最终用户的 `docs/user-guide.md`，README 保持快速入口。

### docs/user-guide.md 内容

1. 这是什么：aipaper-cli 的用途和适用场景。
2. 安装与启动：Windows 双击 exe、终端运行 `./aipaper-cli.exe`。
3. 首次配置 LLM：OpenAI / Anthropic / Ollama / Custom 模板和默认模型。
4. 准备材料：`materials/` 目录、支持格式、失败/降级说明。
5. 生成文章：需求填写、材料扫描、文献确认、AI 写作、导出。
6. 输出文件说明：paper.md、paper.docx、references.bib、citation_trace.json、report.md。
7. 中断与恢复：Ctrl+C 暂停、下次启动恢复。
8. 常见问题：API key 错误、无材料、搜索失败、context 过长、docx 降级。
9. 高级命令：init/status/recover/config 的用途。

### README 更新方向

README 只保留：

- 项目简介。
- 快速开始：双击 `aipaper-cli.exe`。
- 简短流程图。
- 链接到 `docs/user-guide.md`。

README 当前是未跟踪文件，实施阶段修改前需确认是否由用户或其他流程生成，避免误覆盖。

## 测试计划

新增或调整测试应覆盖：

- 无参数入口选择 TUI，有参数入口继续走 CLI。
- 配置向导生成的 `config.Config` 可通过现有 Validate 和 runtime 解析。
- OpenAI / Anthropic / Ollama / Custom 模板的默认值正确。
- API Key 脱敏显示，不进入日志。
- RootModel 根据状态进入正确屏幕。
- MaterialsScan 对空目录、成功、降级、失败文件的状态展示。
- WritingProgress 能消费 mock runtime events 并更新四区数据。
- Streaming delta 实时追加到内容预览。
- Usage 缺失时 token/cost 显示 `--` 且不中断。
- Ctrl+C 写作中触发 checkpoint 暂停流程。
- ExportSummary 调用现有 export 接口并展示路径。
- 用户文档中的命令和路径与实现一致。

全量验证仍要求 `go test ./...` 通过；TUI 可用性可补充最小手动验收步骤。

## 非目标

本阶段不做：

- 手动选择部分材料文件；`materials/` 中的文件全部扫描。
- API 保存后立即测试 provider 连通性。
- Web UI 或浏览器界面。
- 重新设计 Writer/Editor 质量门控规则。
- 新增与当前写作闭环无关的导出格式。
- 自动打开外部网页申请 API Key。

## 风险与约束

- Windows 双击控制台程序的窗口行为需要实际验证；若双击后窗口闪退，需要保证 TUI 启动前有错误兜底或文档说明。
- 不同 provider usage 字段不一致，左侧 token/cost 指标必须允许未知值。
- Claude Opus 4.8 不应默认发送不兼容采样参数。
- Streaming 和 agentcore/litellm 事件映射需要小接口隔离，避免 TUI 直接依赖第三方细节。
- 现有未跟踪 `README.md`、`aipaper-cli`、`aipaper-cli.exe` 不属于本设计落档变更，实施前应确认来源。
