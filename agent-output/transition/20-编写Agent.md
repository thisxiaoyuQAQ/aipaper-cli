## 交接单

- **来源角色**：编写 Agent
- **目标角色**：审查 Agent
- **所属任务**：vault/20-用户文档与README快速入口.md
- **涉及文件**：
  - docs/user-guide.md（新增）
  - README.md（更新）

### 变更摘要

1. **新增 docs/user-guide.md**（完整用户文档，共 9 个章节）：
   - 1. 这是什么：项目概述和核心能力
   - 2. 安装与启动：环境要求、构建、TUI/CLI 启动方式
   - 3. 首次配置 LLM：ConfigWizard 流程、四种模板（OpenAI/Anthropic/Ollama/Custom）、API Key 安全
   - 4. 准备材料：materials/ 目录、支持的格式、BibTeX 自动提取
   - 5. 生成文章：TUI 全流程说明（ConfigWizard → Requirements → MaterialsScan → SearchProgress → References → WritingProgress → ExportSummary → Done）
   - 6. 输出文件说明：final/ 产物（paper.md/docx、references.md、citation-trace.json、report.md）和中间产物
   - 7. 中断与恢复：Ctrl+C 两阶段停止、RecoverPrompt 三选项、CLI recover 命令
   - 8. 常见问题：API Key 错误、无材料、搜索失败、context 过长、docx 失败、Windows 双击闪退
   - 9. 高级命令：init/status/recover/config 详细说明
   - 附录：环境变量参考表

2. **更新 README.md** 为快速入口：
   - 精简为功能特性、快速开始（构建、TUI 启动、CLI 命令）、流程概览图
   - 链接到 docs/user-guide.md 作为完整文档
   - 保留开发与测试、代码结构、项目边界说明

### 下游 Agent 需额外读取

- **项目备忘录.skill**（必读）
- **vault/20-用户文档与README快速入口.md**（任务要求）
- **docs/interfaces/tui.md**（TUI 屏幕枚举和转场契约）
- **docs/interfaces/export.md**（final/ 产物契约）
- **.env.example**（环境变量完整列表）
- **internal/export/export.go**（验证输出文件名和路径）

### 已知风险 / 待确认项

1. **文档一致性检查点**：
   - ConfigWizard 默认模板（OpenAI `gpt-5.5`、Anthropic `claude-opus-4-8`、Ollama `llama3`）需与 TUI 实现一致
   - 输出文件名（`final/paper.md`、`final/paper.docx`、`final/references.md`、`final/citation-trace.json`、`final/report.md`）需与 export 包常量一致
   - CLI 命令（`init`、`status`、`recover`、`config`）需与 internal/cli 实现一致
   - `.env.example` 中的环境变量需在用户文档中全部解释

2. **未实现的功能承诺检查**：
   - 文档中是否承诺了非目标能力（Web UI、自动 API 测试、手动选择部分材料）
   - 项目边界说明是否与 vault 文件要求一致

3. **链接有效性**：
   - README 中指向 `docs/user-guide.md` 的链接
   - 用户文档中提及的其他文档路径（如 docs/需求与架构.md、docs/interfaces/_index.md）

### 编写 Agent 任务完成检查

- [x] 交接单已输出
- [x] docs/user-guide.md 包含 vault 要求的全部 9 个章节
- [x] README 已精简为快速入口，链接到完整用户文档
- [x] 文档中默认模型与 ConfigWizard 一致（gpt-5.5、claude-opus-4-8、llama3）
- [x] 输出文件名与 internal/export 常量一致（paper.md、paper.docx、references.md、citation-trace.json、report.md）
- [x] FAQ 覆盖 vault 要求的问题（API key 错误、无材料、搜索失败、context 过长、docx 降级、Windows 双击窗口行为）
- [x] 文档未承诺非目标能力（Web UI、自动 API 测试、手动选择部分材料）
- [x] .env.example 中的环境变量在附录中全部列出并解释
- [x] README 修改前已确认来源（保留项目边界、代码结构等有用信息）
