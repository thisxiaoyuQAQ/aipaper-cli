## 测试报告 - 模块 20：用户文档与 README 快速入口

### 测试概要

- **测试对象**：docs/user-guide.md、README.md
- **测试日期**：2026-06-09
- **测试方法**：文档一致性自动化验证
- **总用例数**：41
- **通过**：41
- **失败**：0（修复后）
- **跳过**：0

---

## 测试用例明细

### 1. CLI 命令一致性（4/4 通过）

| 命令 | 文档位置 | 代码位置 | 状态 |
|---|---|---|---|
| `init` | docs/user-guide.md:58 | internal/cli/cli.go:40 | ✅ 通过 |
| `status` | docs/user-guide.md:59 | internal/cli/cli.go:42 | ✅ 通过 |
| `recover` | docs/user-guide.md:60 | internal/cli/cli.go:44 | ✅ 通过 |
| `config` | docs/user-guide.md:61 | internal/cli/cli.go:46 | ✅ 通过 |

### 2. 导出文件路径一致性（5/5 通过）

| 文件路径 | 文档位置 | 代码位置 | 状态 |
|---|---|---|---|
| `final/paper.md` | docs/user-guide.md:217 | internal/export/export.go:19 | ✅ 通过 |
| `final/paper.docx` | docs/user-guide.md:218 | internal/export/export.go:20 | ✅ 通过 |
| `final/references.md` | docs/user-guide.md:219 | internal/export/export.go:21 | ✅ 通过 |
| `final/citation-trace.json` | docs/user-guide.md:220 | internal/export/export.go:22 | ✅ 通过 |
| `final/report.md` | docs/user-guide.md:221 | internal/export/export.go:23 | ✅ 通过 |

### 3. 环境变量文档完整性（22/22 通过）

所有 `.env.example` 中的环境变量均已在 `docs/user-guide.md` 附录中记录：

| 变量名 | 状态 |
|---|---|
| `OPENROUTER_API_KEY` | ✅ 已记录 |
| `ANTHROPIC_API_KEY` | ✅ 已记录 |
| `GEMINI_API_KEY` | ✅ 已记录 |
| `OPENAI_API_KEY` | ✅ 已记录 |
| `DEEPSEEK_API_KEY` | ✅ 已记录 |
| `QWEN_API_KEY` | ✅ 已记录 |
| `GLM_API_KEY` | ✅ 已记录 |
| `GROK_API_KEY` | ✅ 已记录 |
| `OLLAMA_BASE_URL` | ✅ 已记录 |
| `ANTHROPIC_BASE_URL` | ✅ 已记录（修复后） |
| `CUSTOM_LLM_PROVIDER` | ✅ 已记录（修复后） |
| `CUSTOM_LLM_TYPE` | ✅ 已记录（修复后） |
| `CUSTOM_LLM_BASE_URL` | ✅ 已记录 |
| `CUSTOM_LLM_MODEL` | ✅ 已记录（修复后） |
| `CUSTOM_LLM_API_KEY` | ✅ 已记录 |
| `SERPAPI_KEY` | ✅ 已记录 |
| `TAVILY_KEY` | ✅ 已记录 |
| `EXA_KEY` | ✅ 已记录 |
| `GOOGLE_SCHOLAR_PROXY_URL` | ✅ 已记录 |
| `AIPAPER_DEFAULT_LANGUAGE` | ✅ 已记录 |
| `AIPAPER_CITATION_STYLE` | ✅ 已记录 |
| `AIPAPER_OUTPUT_DIR` | ✅ 已记录 |

### 4. README 链接有效性（4/4 通过）

| 链接 | 目标文件 | 状态 |
|---|---|---|
| `docs/user-guide.md` | 存在 | ✅ 通过 |
| `docs/需求与架构.md` | 存在 | ✅ 通过 |
| `docs/interfaces/_index.md` | 存在 | ✅ 通过 |
| `docs/开发进度.md` | 存在 | ✅ 通过 |

### 5. 默认模型一致性（3/3 通过）

| 模型 | 文档位置 | 接口定义 | 状态 |
|---|---|---|---|
| `gpt-5.5` | docs/user-guide.md:72 | docs/interfaces/tui.md:76 | ✅ 通过 |
| `claude-opus-4-8` | docs/user-guide.md:73 | docs/interfaces/tui.md:76 | ✅ 通过 |
| `llama3` | docs/user-guide.md:74 | docs/interfaces/tui.md:76 | ✅ 通过 |

### 6. 非目标能力检查（3/3 通过）

| 术语 | 检查项 | 状态 |
|---|---|---|
| Web UI | 未在文档中承诺 | ✅ 通过 |
| 自动 API 测试 | 未在文档中承诺 | ✅ 通过 |
| 手动选择部分材料 | 未在文档中承诺 | ✅ 通过 |

---

## 修复记录

### 修复轮次 1：补充缺失的环境变量文档

**问题**：初始测试发现 4 个环境变量未在用户文档中记录
- `ANTHROPIC_BASE_URL`
- `CUSTOM_LLM_PROVIDER`
- `CUSTOM_LLM_TYPE`
- `CUSTOM_LLM_MODEL`

**修复方式**：在 `docs/user-guide.md` 附录的环境变量表中补充这 4 个变量及其说明

**修复验证**：重新检查后全部通过

---

## 覆盖率

### 文档结构覆盖

- [x] 第 1 章：这是什么（项目概述、核心能力）
- [x] 第 2 章：安装与启动（环境、构建、TUI/CLI 启动）
- [x] 第 3 章：首次配置 LLM（ConfigWizard 流程、四种模板、API Key 安全）
- [x] 第 4 章：准备材料（materials/ 目录、格式支持）
- [x] 第 5 章：生成文章（TUI 全流程说明）
- [x] 第 6 章：输出文件说明（final/ 产物和中间产物）
- [x] 第 7 章：中断与恢复（Ctrl+C 停止、RecoverPrompt 恢复）
- [x] 第 8 章：常见问题（API Key 错误、无材料、搜索失败、context 过长、docx 失败、Windows 双击闪退）
- [x] 第 9 章：高级命令（init/status/recover/config 详细说明）
- [x] 附录：环境变量参考（完整列表 22 个）

### Vault 要求覆盖

- [x] 文档命令与实现一致
- [x] 输出文件名与实际 export Result 一致
- [x] `.env.example` 中出现的变量在文档中解释
- [x] README 链接有效

---

## 结论

- [x] ✅ **全部通过**

### 测试通过依据

1. **CLI 命令验证**：全部 4 个命令（init、status、recover、config）在文档和代码中完全一致
2. **导出文件路径验证**：全部 5 个 final/ 产物路径与 export.go 常量完全一致
3. **环境变量完整性**：全部 22 个 .env.example 变量均在用户文档中记录并解释
4. **链接有效性**：README 中引用的全部 4 个文档文件均存在
5. **默认模型一致性**：全部 3 个 ConfigWizard 默认模型与 TUI 接口定义一致
6. **非目标能力检查**：文档未承诺任何非目标能力（Web UI、自动 API 测试、手动选择材料）

### 质量指标

- 文档行数：user-guide.md 381 行（含 4 行修复）、README.md 120 行
- 一致性测试用例：41 个，全部通过
- 修复轮次：1 轮（环境变量补充）
- 审查发现问题：0 个（审查 Agent 已通过）

### 下一步

可进入开发进度更新和 Git commit 阶段。
