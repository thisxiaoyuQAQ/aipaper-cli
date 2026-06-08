# TUI 全流程增量需求

> 来源：`docs/superpowers/specs/2026-06-08-tui-full-generation-user-guide-design.md`  
> 范围：在已验收 MVP（01-09）基础上，增量实现 Windows 双击 / 无参数启动的完整 TUI 生成流程与最终用户文档。  
> 非目标：不重写已稳定的 `internal/checkpoint`、`internal/agent`、`internal/artifacts`、`internal/export`、`internal/tui/requirements`、`internal/tui/references` 规则。

## 1. 项目背景

当前 `aipaper-cli` 已完成 Store/config、材料解析、学术搜索、文献确认 TUI model、Agent runtime、写作产物质量门控、checkpoint 恢复、最终导出和 E2E 冒烟测试。下一阶段目标是把这些能力编排成最终用户可直接使用的 TUI：Windows 用户双击 `aipaper-cli.exe` 或终端无参数运行即可从配置开始，一路完成需求填写、材料扫描、文献确认、真实 LLM 写作、导出与恢复。

## 2. 用户故事

1. 作为首次使用的 Windows 用户，我希望双击 `aipaper-cli.exe` 后直接进入交互式界面，不需要先学习命令行参数。
2. 作为首次配置 LLM 的用户，我希望通过 OpenAI / Anthropic / Ollama / Custom 模板填写最少必要字段，并生成项目级 `aipaper.json`。
3. 作为综述写作者，我希望在 TUI 内填写写作需求，随后自动扫描 `materials/` 目录中的全部材料。
4. 作为需要引用可控的用户，我希望候选文献必须经过确认后才允许进入写作。
5. 作为等待生成的用户，我希望在写作过程中看到日志、流式正文、章节进度、token/cost 等实时状态。
6. 作为可能中断任务的用户，我希望 Ctrl+C 能保存进度，下次启动可继续。
7. 作为最终用户，我希望看到清晰的导出文件说明、常见问题和高级命令文档。

## 3. 功能需求

### 3.1 默认启动与 CLI 兼容

- 无参数运行或双击 `aipaper-cli.exe` 时进入 TUI。
- 带参数运行时保留现有 `init/status/recover/config` 命令行为。
- TUI 启动失败时恢复终端状态，输出简短错误信息，不吞掉 panic。

### 3.2 ConfigWizard

- 首次运行或配置缺失时进入配置向导。
- 默认写项目配置 `./aipaper.json`，避免污染全局配置。
- 提供四类模板：OpenAI、Anthropic、Ollama、Custom。
- 三步交互：选择模板 → 填必要字段 → 展示摘要并保存。
- 不做 API 连通性测试。
- API Key 在 UI、日志、错误、report 中必须脱敏。
- API Key 保存优先使用环境变量引用（如 `env:OPENAI_API_KEY` / `env:ANTHROPIC_API_KEY`）；若用户选择直接写入项目配置，必须展示安全提示。
- 生成的配置必须复用现有 `config.Config`、`ProviderConfig`、`RoleConfig`。

### 3.3 Requirements 屏幕

- 复用 `internal/tui/requirements` model/view/validation。
- RootModel 负责桥接 Bubble Tea 事件、落盘 `requirements.json`、投影成功事件。
- 不重写字段校验规则。

### 3.4 MaterialsScan 屏幕

- Requirements 完成后扫描当前项目 `materials/` 目录。
- 目录不存在时创建目录，并提示用户放入材料。
- 扫描目录内全部文件，不提供手动选择部分文件。
- 支持格式沿用现有 `internal/materials`：PDF、Markdown、TXT、BibTeX；DOCX、URL、CSV 降级支持。
- 单文件失败不阻塞；全部失败时提示重新放入材料或跳过。
- BibTeX 候选必须保留并与后续搜索候选合并。

### 3.5 AcademicSearch / SearchProgress

- 材料扫描后根据 requirements 执行学术搜索。
- 单个 provider 失败不阻塞。
- 所有 provider 失败时提供重试、跳过搜索、返回材料步骤。
- 跳过搜索时仍可使用 BibTeX 或已有材料候选继续，但 References 屏需明确候选不足。

### 3.6 References 屏幕

- 复用 `internal/tui/references` model/view/key 冲突处理和 confirmed/rejected/BibTeX 写入规则。
- Writer 和 Editor 只能使用 `references/confirmed.json` 的硬规则不变。
- 未确认任何文献不得进入写作。

### 3.7 WritingProgress 四区布局

四区布局：左侧指标、中上日志、中下流式正文、右侧进度。

- 左侧指标：运行态、阶段、完成比例、字数、模型、context、tokens、cost、elapsed。
- 中上日志：Coordinator / Architect / Writer / Editor / Export 事件，支持自动滚动和手动滚动暂停。
- 中下内容：Writer streaming delta 实时追加，章节切换和 Editor 重写需可见。
- 右侧进度：阶段、章节状态、评分、重写次数、总进度条、引用一致性、最终字数。
- 窗口过窄时降级为纵向堆叠，不崩溃。

### 3.8 真实 LLM 运行时接入

- 复用 `internal/app/agent_runtime.go`、`internal/agent`、`internal/artifacts`、`internal/export` 边界。
- Host 负责 provider/model/role 解析、模型适配器创建、工具注册、事件转换、恢复 prompt 注入。
- Coordinator 仍是唯一流程决策者；Host 和 TUI 不硬编码写作顺序。
- Runtime 事件需覆盖 step、role log、streaming delta、usage、chapter status、review、checkpoint、export artifact、done/error。

### 3.9 暂停、恢复与退出

- 写作中 Ctrl+C 请求 runtime 在安全点停止，保存 checkpoint，展示“进度已保存，下次启动可继续”。
- 下次启动检测未完成 run 时显示恢复提示：继续写作、重新开始、退出。
- 重新开始需要二次确认，不得无提示清空或覆盖产物。
- 表单页或确认页 Ctrl+C 显示退出确认，未保存改动需提示。

### 3.10 ExportSummary 与 Done

- WritingProgress 完成后进入 ExportSummary。
- 复用 `export.LoadInput` / `export.ExportFinal`。
- 展示实际输出目录和文件列表。
- Docx 降级时明确提示 Markdown/report 已成功生成。
- BibTeX 输出路径需与实现一致：确认阶段为 `references/confirmed.bib`；如新增 final BibTeX，需同步测试和文档。

### 3.11 用户文档

- 新增 `docs/user-guide.md`。
- README 保持快速入口：项目简介、双击 / 无参数启动、简短流程图、链接到 user guide。
- 文档覆盖安装、首次配置、材料准备、生成流程、输出文件、中断恢复、FAQ、高级命令。

## 4. 配置项

| 配置 | 默认值 | 取值 / 说明 |
|---|---|---|
| OpenAI type | `openai` | ConfigWizard 模板 |
| OpenAI base_url | `https://api.openai.com/v1` | 可编辑 |
| OpenAI model | `gpt-5.5` | 默认模型 |
| Anthropic type | `anthropic` | ConfigWizard 模板 |
| Anthropic base_url | `https://api.anthropic.com` | 可编辑 |
| Anthropic model | `claude-opus-4-8` | 默认模型 |
| Ollama type | `ollama` | ConfigWizard 模板 |
| Ollama base_url | `http://localhost:11434` | 可编辑 |
| Ollama model | `llama3` | 可编辑 |
| Custom | 用户填写 | provider name/type/base_url/model/api_key |
| role 映射 | default provider/model | 至少覆盖 coordinator/architect/writer/editor，保证 runtime 可解析 |
| 配置保存位置 | `./aipaper.json` | TUI 第一版默认项目级 |
| Store 根 | `output/aipaper/` | 沿用既有约定 |
| 材料目录 | `./materials` | 不存在时 TUI 创建并提示 |

## 5. 数据范围与存储策略

- 材料文件数量：按个人项目规模设计，几十到数百个文件可用；超大目录需在 UI 中明确扫描进度。
- 单文件失败不阻塞整体扫描，失败原因记录在 manifest 并在 Details 展示。
- 写作正文、review、checkpoint、final artifacts 继续落盘到 `output/aipaper/`。
- TUI 状态不替代 Store；恢复和最终产物以 Store 文件为权威。
- API key 不写入事件、日志、report、错误详情；配置摘要只显示脱敏值。

## 6. 性能与体验目标

| 场景 | 目标 |
|---|---|
| 无参数启动到首屏 | 本地配置探测后尽快显示，避免长时间空白 |
| 表单输入响应 | 键入即时响应，无明显卡顿 |
| 材料扫描 | 后台执行，显示当前文件和统计 |
| 搜索 | provider 失败不阻塞其他 provider；展示可重试错误 |
| Streaming | Writer delta 尽快显示到内容区，不等整章完成 |
| 窄窗口 | 不崩溃，降级布局并提示放大窗口 |
| 缺 usage | token/cost 显示 `--`，不阻塞写作 |

## 7. 技术栈约束

- 语言：Go 1.25。
- TUI：Bubble Tea、Bubbles、Lip Gloss。
- Agent：`github.com/voocel/agentcore`。
- LLM：`github.com/voocel/litellm`。
- Store：现有 `internal/store` 原子写入。
- Runtime：现有 `internal/app/agent_runtime.go` 和 `internal/agent` 边界。

## 8. 非功能需求

- 安全：API key 脱敏；不得写入 run events、report、错误详情。
- 可恢复：关键步骤以 checkpoint / progress / artifact hash 为依据。
- 可测试：新增屏幕 model 和 root state probe 均应可用 mock 测试，不依赖真实 LLM。
- 可观测：WritingProgress 展示结构化事件；错误需可读且可恢复路径明确。
- 兼容：带参数 CLI 行为不变；无参数才进入 TUI。

## 9. 环境要求

- 开发：Go 1.25+。
- 运行：至少一个 LLM provider 或本地 Ollama。
- 可选环境变量：见 `.env.example`，包括 `OPENAI_API_KEY`、`ANTHROPIC_API_KEY`、`OLLAMA_BASE_URL`、`CUSTOM_LLM_*`、搜索增强 provider key。
- Windows：需实际验证双击控制台程序的工作目录、窗口保持和错误兜底。

## 10. 验收标准

- `go test ./...` 通过。
- 无参数运行进入 TUI；有参数命令行为不变。
- ConfigWizard 模板默认值正确，保存后 `config.Load` 和 role runtime 解析可用。
- Requirements、References 复用既有 model 规则。
- MaterialsScan 不丢失 BibTeX 候选。
- SearchProgress 能处理 provider 单点失败和全失败路径。
- WritingProgress 能消费 mock runtime events，并实时显示 streaming delta。
- Ctrl+C 写作中进入安全停止 / checkpoint 恢复路径。
- ExportSummary 展示实际存在的输出文件。
- `docs/user-guide.md` 与实现路径和命令一致。
