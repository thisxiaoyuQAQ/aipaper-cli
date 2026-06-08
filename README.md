# aipaper-cli

`aipaper-cli` 是一个面向学术综述 / 文献综述写作的 AI Agent CLI。它围绕“需求收集 → 材料解析 → 学术搜索 → 人工确认文献 → Agent 写作与评审 → checkpoint 恢复 → 最终导出”构建本地工作流，目标是帮助用户把已有材料和联网检索结果整理成可追溯、可复核的综述草稿。

> 当前版本重点完成了底层能力与可测试闭环：Store/config、材料解析、学术搜索、文献确认 TUI model、Agent runtime、写作产物质量门控、checkpoint 恢复、最终导出和 E2E 冒烟测试。CLI 暴露的稳定命令见下文。

## 功能特性

- **本地项目 Store**：默认输出到 `output/aipaper/`，所有运行状态、材料、文献、草稿和导出文件都在本地落盘。
- **多来源配置**：支持全局配置、项目配置和命令行显式配置叠加。
- **材料解析**：支持 Markdown、TXT、BibTeX、PDF；对 DOCX、URL、CSV 提供基础 / 降级解析。
- **学术搜索**：内置 Semantic Scholar、Crossref、arXiv、PubMed；可扩展 SerpAPI、Tavily、Exa、Google Scholar 代理和自定义源。
- **人工确认文献**：候选文献必须经用户确认后才会进入 `references/confirmed.json`，未确认文献不能被最终引用。
- **Agent 写作流程**：Coordinator 调度 Architect、Writer、Editor，生成大纲、章节草稿、论断、引用映射与评审结果。
- **质量门控**：章节需要满足分数阈值、引用一致性和 unsupported claim 检查后才能进入最终导出。
- **Step checkpoint 恢复**：关键步骤成功后写入 checkpoint，可用于崩溃后的恢复校验。
- **最终导出**：生成 `paper.md`、`paper.docx`、`references.md`、`citation-trace.json`、`report.md`。

## 环境要求

- Go `1.25.0` 或更高版本
- 可选：至少一个 LLM Provider API Key，用于真实 Agent 写作流程
- 可选：学术搜索增强服务 API Key，如 SerpAPI、Tavily、Exa

## 快速开始

### 1. 克隆项目

```bash
git clone https://github.com/thisxiaoyuQAQ/aipaper-cli.git
cd aipaper-cli
```

### 2. 安装依赖 / 验证构建

```bash
go mod download
go build ./...
go test ./...
```

### 3. 查看帮助

```bash
go run ./cmd/aipaper-cli --help
```

当前 CLI 命令：

```text
aipaper-cli init [--workdir DIR] [--config FILE]
aipaper-cli status [--workdir DIR]
aipaper-cli recover [--workdir DIR]
aipaper-cli config [--workdir DIR] [--config FILE]
```

如果你想先构建二进制：

```bash
go build -o aipaper-cli ./cmd/aipaper-cli
./aipaper-cli --help
```

Windows PowerShell 下可运行：

```powershell
go build -o aipaper-cli.exe ./cmd/aipaper-cli
.\aipaper-cli.exe --help
```

## 配置教程

`aipaper-cli` 会按以下顺序加载配置，后加载的配置覆盖先加载的配置：

1. 全局配置：`~/.aipaper/config.json`
2. 项目配置：`./aipaper.json`
3. 显式配置：`--config FILE`

### 配置文件示例

在项目根目录创建 `aipaper.json`：

```json
{
  "provider": "openrouter",
  "model": "google/gemini-2.5-pro",
  "default_language": "zh-CN",
  "citation_style": "gbt7714",
  "providers": {
    "openrouter": {
      "type": "openrouter",
      "api_key": "env:OPENROUTER_API_KEY",
      "base_url": "https://openrouter.ai/api/v1"
    }
  },
  "roles": {
    "editor": {
      "provider": "openrouter",
      "model": "anthropic/claude-sonnet-4",
      "max_turns": 4,
      "temperature": 0.2
    }
  }
}
```

说明：

- `provider` 和 `model` 需要成对出现。
- `providers` 是 provider 配置表，`provider` 的值必须能在其中找到。
- `api_key` 支持 `env:环境变量名` 写法，运行时会从环境变量读取密钥。
- `roles` 可为 Coordinator、Architect、Writer、Editor 等角色设置不同模型或参数。
- `config` 命令输出配置时会自动隐藏非空 `api_key`。

### 环境变量

项目提供 `.env.example` 作为参考：

```bash
OPENROUTER_API_KEY=
ANTHROPIC_API_KEY=
GEMINI_API_KEY=
OPENAI_API_KEY=
DEEPSEEK_API_KEY=
QWEN_API_KEY=
GLM_API_KEY=
GROK_API_KEY=

OLLAMA_BASE_URL=http://localhost:11434
CUSTOM_LLM_BASE_URL=
CUSTOM_LLM_API_KEY=

SERPAPI_KEY=
TAVILY_KEY=
EXA_KEY=
GOOGLE_SCHOLAR_PROXY_URL=

AIPAPER_DEFAULT_LANGUAGE=zh-CN
AIPAPER_CITATION_STYLE=gbt7714
AIPAPER_OUTPUT_DIR=output/aipaper
```

如果配置中写了：

```json
"api_key": "env:OPENROUTER_API_KEY"
```

则运行前需要设置对应环境变量，例如：

```bash
export OPENROUTER_API_KEY="your-api-key"
```

PowerShell：

```powershell
$env:OPENROUTER_API_KEY="your-api-key"
```

## 基础使用教程

### 初始化工作目录

```bash
go run ./cmd/aipaper-cli init --workdir .
```

执行后会创建或复用本地 Store：

```text
output/aipaper/
  run.json
  progress.json
  checkpoints/
  materials/
  references/
  outline/
  drafts/
  reviews/
  final/
```

命令会输出 Store 路径，以及本次新建或已存在的文件。

### 使用显式配置初始化

```bash
go run ./cmd/aipaper-cli init --workdir . --config ./aipaper.json
```

当你有多个实验配置时，可以用 `--config` 指定额外配置文件。

### 查看当前状态

```bash
go run ./cmd/aipaper-cli status --workdir .
```

如果尚未初始化，会输出：

```text
status: not initialized
store: output/aipaper
```

如果已初始化，会输出 `progress.json` 的格式化 JSON，例如当前阶段、step、章节进度和更新时间。

### 查看合并后的配置

```bash
go run ./cmd/aipaper-cli config --workdir . --config ./aipaper.json
```

输出内容包含：

- `loaded`：实际加载到的配置文件列表
- `config`：合并后的配置对象

敏感字段会被处理为：

```json
"api_key": "redacted"
```

### 校验 checkpoint 恢复状态

```bash
go run ./cmd/aipaper-cli recover --workdir .
```

该命令会读取 `checkpoints/latest.json`，校验 checkpoint 指向的产物路径和哈希是否一致，并输出恢复检查结果。若不可恢复，命令返回非零退出码。

## 推荐工作流

一个典型的使用流程如下：

1. **准备材料**
   - 将论文、笔记、BibTeX、Markdown、TXT 等材料放到项目材料目录。
   - 推荐提前整理文件名，方便后续追踪来源。

2. **配置模型与搜索服务**
   - 在 `aipaper.json` 中配置默认 provider、model、引用格式和语言。
   - 用环境变量保存 API Key，避免把密钥写进仓库。

3. **初始化 Store**
   - 运行 `init` 创建 `output/aipaper/` 布局。

4. **收集需求与解析材料**
   - 需求结构包括主题、研究问题、综述范围、语言、引用格式、目标字数、材料目录、是否允许联网搜索等。
   - 材料解析结果会进入 `materials/manifest.json`、`materials/extracted/` 和 `materials/parsed/`。

5. **搜索并确认文献**
   - 系统生成候选文献后，用户需要人工确认。
   - 确认结果写入 `references/confirmed.json` 和 `references/confirmed.bib`。

6. **Agent 写作与评审**
   - Architect 生成大纲。
   - Writer 生成章节草稿、claims 和 citation map。
   - Editor 评审章节质量，必要时触发重写或人工复核。

7. **导出最终产物**
   - 通过最终导出流程生成 `final/` 下的论文草稿、引用列表、引用追踪和报告。

## 输出目录说明

默认 Store 根目录为：

```text
output/aipaper/
```

核心文件：

| 路径 | 说明 |
|---|---|
| `run.json` | 本次运行的元信息、provider、model、事件和成本估计 |
| `progress.json` | 当前阶段、step、章节状态和更新时间 |
| `requirements.json` | 结构化写作需求 |
| `checkpoints/latest.json` | 最新 checkpoint 指针 |
| `checkpoints/step-*.json` | 每一步成功产物的 checkpoint |
| `materials/manifest.json` | 材料扫描与解析清单 |
| `materials/extracted/` | 提取出的文本内容 |
| `materials/parsed/` | 解析后的结构化材料 |
| `references/candidates.json` | 候选文献 JSON |
| `references/candidates.md` | 候选文献 Markdown 预览 |
| `references/confirmed.json` | 用户确认后的可引用文献 |
| `references/confirmed.bib` | 用户确认后的 BibTeX |
| `references/rejected.json` | 被拒绝的候选文献 |
| `outline/outline.json` | 结构化大纲 |
| `outline/outline.md` | 大纲 Markdown |
| `drafts/` | 章节草稿、claims、citation map 和 writer notes |
| `reviews/` | Editor 评审结果 |
| `final/paper.md` | 最终 Markdown 草稿 |
| `final/paper.docx` | 最终 Word 文档 |
| `final/references.md` | 最终参考文献列表 |
| `final/citation-trace.json` | 引用追踪文件 |
| `final/report.md` | 导出报告与质量摘要 |

## 引用与质量规则

`aipaper-cli` 对引用和质量门控有几个硬性约束：

- Writer 和 Editor 只能读取已确认文献，即 `references/confirmed.json`。
- 未确认的联网文献不能进入最终引用。
- 单章质量分数需要达到阈值。
- 引用一致性需要达到阈值。
- 不能存在高风险 unsupported claim。
- 超过重写轮次后，章节会进入人工复核状态。

最终导出的 `citation-trace.json` 会记录章节、段落、claim、reference key、来源类型和是否需要人工复核，方便用户检查每条引用的来源。

## 开发与测试

### 运行全部测试

```bash
go test ./...
```

### 编译检查

```bash
go build ./...
```

### 运行 CLI 单命令

```bash
go run ./cmd/aipaper-cli status --workdir .
```

### 主要代码结构

```text
cmd/aipaper-cli/          CLI 入口
internal/cli/             命令解析：init/status/recover/config
internal/app/             应用 bootstrap、Agent runtime、recover
internal/config/          配置加载、合并、校验、redact
internal/contracts/       结构化 JSON 契约
internal/store/           Store 路径、原子写入、哈希、布局创建
internal/checkpoint/      Step checkpoint 记录与恢复校验
internal/materials/       材料扫描、解析、manifest 写入
internal/search/          学术搜索 provider、标准化、去重
internal/references/      候选/确认/拒绝文献、BibTeX、reference key
internal/agent/           Coordinator、prompt 与工具桥接
internal/tui/             需求表单、文献确认等 TUI model
internal/artifacts/       写作产物与质量门控
internal/export/          Markdown、Docx、引用追踪和报告导出
fixtures/review-mini/     E2E 冒烟测试夹具
```

## 常见问题

### `init` 报错：`provider is required when model is set`

配置中只写了 `model`，没有写 `provider`。请同时配置：

```json
{
  "provider": "openrouter",
  "model": "google/gemini-2.5-pro"
}
```

### `init` 报错：`provider "xxx" is not configured`

`provider` 指向的 key 不存在于 `providers` 表。请检查配置：

```json
{
  "provider": "openrouter",
  "providers": {
    "openrouter": {
      "type": "openrouter",
      "api_key": "env:OPENROUTER_API_KEY"
    }
  }
}
```

### `recover` 返回非零退出码

说明最新 checkpoint 不可恢复，常见原因包括：

- `checkpoints/latest.json` 不存在
- checkpoint 指向的 artifact 被删除
- artifact 内容被手动修改，哈希不匹配
- checkpoint 中的路径不符合 Store 根目录约束

建议先查看命令输出的 JSON，再检查 `output/aipaper/checkpoints/` 和相关 artifact。

### `config` 输出没有加载项目配置

确认当前 `--workdir` 下是否存在 `aipaper.json`，或用 `--config FILE` 显式指定配置文件。

### 如何避免泄露 API Key？

- 不要把真实 key 直接提交到 `aipaper.json`。
- 推荐使用 `env:KEY_NAME` 写法。
- `.env.example` 只保留变量名，不填写真实值。
- `aipaper-cli config` 输出会 redact 非空 `api_key`，但仍建议不要在日志中打印原始环境变量。

## 项目边界

当前 MVP 不承诺：

- 生成可直接投稿的最终论文。
- 完美处理 GB/T 7714、APA 或 Word 高级排版。
- OCR 扫描 PDF、深度理解图片 / 表格 / 公式。
- 多用户后台、云同步或远程任务队列。
- 对联网文献真实性自动背书；文献仍需要用户确认和复核。

## 许可证

暂未在仓库中发现许可证文件。发布或分发前建议补充 `LICENSE`。
