## 审查报告 - 模块 20：用户文档与 README 快速入口

### 审查概要

- **审查对象**：docs/user-guide.md（新增）、README.md（更新）
- **审查日期**：2026-06-09
- **审查结论**：✅ 审查通过

---

## 1. 安全性

- [x] 无输入参数校验问题（文档类任务，不涉及）
- [x] 无 SQL/NoSQL 注入风险（文档类任务，不涉及）
- [x] 无 XSS 风险（文档类任务，不涉及）
- [x] 无权限校验问题（文档类任务，不涉及）
- [x] 敏感数据脱敏：文档中明确说明 API Key 自动脱敏（如 `sk-...abcd`），所有日志、摘要和报告中均脱敏
- [x] 无密钥/凭证硬编码：推荐使用 `env:OPENAI_API_KEY` 环境变量引用

## 2. 并发与数据一致性

- [x] 不涉及（文档类任务）

## 3. 资源管理

- [x] 不涉及（文档类任务）

## 4. 性能

- [x] 不涉及（文档类任务）

## 5. 错误处理

- [x] 不涉及（文档类任务）

## 6. 项目约定一致性

### 6.1 核心约定检查

- [x] **输出文件名与 export 包一致**：
  - 文档：`final/paper.md`、`final/paper.docx`、`final/references.md`、`final/citation-trace.json`、`final/report.md`
  - 代码：`internal/export/export.go:19-23` 定义完全一致

- [x] **CLI 命令与实现一致**：
  - 文档：`init`、`status`、`recover`、`config`
  - 代码：`internal/cli/cli.go:40-46` 实现完全一致

- [x] **环境变量与 .env.example 一致**：
  - 文档附录列出：`OPENAI_API_KEY`、`ANTHROPIC_API_KEY`、`OPENROUTER_API_KEY`、`GEMINI_API_KEY`、`DEEPSEEK_API_KEY`、`QWEN_API_KEY`、`GLM_API_KEY`、`GROK_API_KEY`、`OLLAMA_BASE_URL`、`CUSTOM_LLM_BASE_URL`、`CUSTOM_LLM_API_KEY`、`SERPAPI_KEY`、`TAVILY_KEY`、`EXA_KEY`、`GOOGLE_SCHOLAR_PROXY_URL`、`AIPAPER_DEFAULT_LANGUAGE`、`AIPAPER_CITATION_STYLE`、`AIPAPER_OUTPUT_DIR`
  - `.env.example` 包含全部上述变量

- [x] **ConfigWizard 默认模板与 TUI 接口一致**：
  - 文档：OpenAI `gpt-5.5`、Anthropic `claude-opus-4-8`、Ollama `llama3`
  - 接口：`docs/interfaces/tui.md:76` 定义完全一致

- [x] **TUI 屏幕转场流程与架构设计一致**：
  - 文档：ConfigWizard → Requirements → MaterialsScan → SearchProgress → References → WritingProgress → ExportSummary → Done
  - 架构：`docs/TUI全流程增量架构设计.md:26-43` 状态机完全一致

- [x] **Store 输出目录路径一致**：
  - 文档：`output/aipaper/`
  - 项目备忘录：`output/aipaper/`

### 6.2 链接有效性

- [x] README → `docs/user-guide.md`（相对路径，已确认文件存在）
- [x] README → `docs/需求与架构.md`（已确认文件存在）
- [x] README → `docs/interfaces/_index.md`（已确认文件存在）
- [x] README → `docs/开发进度.md`（已确认文件存在）

### 6.3 未实现功能承诺检查

- [x] ✅ 文档未承诺 Web UI
- [x] ✅ 文档未承诺自动 API 连通性测试
- [x] ✅ 文档未承诺手动选择部分材料
- [x] ✅ 项目边界说明与 vault 要求一致（README:108-115 和用户文档未过度承诺）

## 7. 代码质量

- [x] 文档结构清晰，章节编号完整（1-9 + 附录）
- [x] 用户指南共 377 行，结构合理，未过度展开实现细节
- [x] README 共 120 行，简洁明了，链接到完整文档
- [x] 边界条件处理：FAQ 覆盖 API Key 错误、无材料、搜索失败、context 过长、docx 失败、Windows 双击闪退
- [x] 命名一致：所有命令、文件路径、屏幕名称与代码实现一致
- [x] 无魔法值：环境变量名、默认 URL、模型名均有明确说明

---

## 发现的问题

**无严重问题、警告或建议项。**

---

## 结论

- [x] ✅ **审查通过**

### 审查通过理由

1. **文档完整性**：用户指南包含 vault 要求的全部 9 个章节（这是什么、安装与启动、首次配置 LLM、准备材料、生成文章、输出文件说明、中断与恢复、常见问题、高级命令）+ 环境变量附录
2. **一致性验证**：所有命令、文件路径、默认模型、环境变量均与代码实现和接口定义一致
3. **链接有效性**：README 中的所有文档链接均已验证存在
4. **项目边界清晰**：未承诺非目标能力（Web UI、自动 API 测试、手动选择材料），项目边界说明与 vault 一致
5. **FAQ 覆盖完整**：包含 vault 要求的全部常见问题
6. **安全合规**：API Key 脱敏说明清晰，推荐环境变量引用
7. **用户友好**：Windows 双击、终端运行、CLI 命令等多种使用方式均有说明

### 下一步

可进入测试 Agent 阶段，验证文档一致性。
