# 20-用户文档与README快速入口

## 模块概述

新增最终用户文档 `docs/user-guide.md`，并更新 README 为快速入口，帮助 Windows 双击用户和终端用户完成安装、首次配置、材料准备、生成、导出和恢复。

## 前置依赖

- 依赖模块：10-无参数TUI启动与RootModel骨架 至 19-ExportSummary与完成页
- 可并行模块：无

## 最小上下文清单

- 项目备忘录.md
- docs/TUI全流程增量需求.md
- docs/TUI全流程增量架构设计.md
- README.md
- .env.example
- docs/interfaces/export.md
- internal/export/export.go

## 接口与类型定义

文档必须覆盖实际命令：

```text
aipaper-cli                 # 无参数进入 TUI
aipaper-cli.exe             # Windows 终端运行 / 双击进入 TUI
aipaper-cli init
aipaper-cli status
aipaper-cli recover
aipaper-cli config
```

实际输出路径以 export 和 references 契约为准。

## 实现要求

- 新增 `docs/user-guide.md`，包含：
  1. 这是什么；
  2. 安装与启动；
  3. 首次配置 LLM；
  4. 准备材料；
  5. 生成文章；
  6. 输出文件说明；
  7. 中断与恢复；
  8. 常见问题；
  9. 高级命令。
- README 只保留项目简介、快速开始、简短流程图、链接到 `docs/user-guide.md`。
- 修改 README 前确认其来源；当前 README 已存在且不是本轮新建，不要误删有用信息，可做精简迁移。
- 文档中的默认模型需与 ConfigWizard 一致：OpenAI `gpt-5.5`、Anthropic `claude-opus-4-8`、Ollama `llama3`。
- FAQ 覆盖 API key 错误、无材料、搜索失败、context 过长、docx 降级、Windows 双击窗口行为。
- 文档不得承诺非目标能力，如 Web UI、自动 API 测试、手动选择部分材料。

## 测试要求

- 文档命令与实现一致。
- 输出文件名与实际 export Result 一致。
- `.env.example` 中出现的变量在文档中解释。
- README 链接有效。

## 任务清单（预期产出）

- `docs/user-guide.md`
- README 快速入口更新
- 可选：文档一致性检查清单

## 模块代码行数预估

- 用户文档不强制 300 行以内，但应结构清晰，避免过度展开实现细节。
