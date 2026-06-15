# Paper-Cli 用户指南

## 1. 产品概述

`Paper-Cli` 是一个面向学术综述 / 文献综述写作的 AI Agent 命令行工具。它通过交互式 TUI 收集写作需求、解析用户材料、搜索学术文献、确认引用，并调度 Coordinator、Architect、Writer、Editor 多角色协作完成「大纲 → 草稿 → 评审 → 重写 → 导出」流程。

代码入口仍位于 `cmd/aipaper-cli`，配置文件默认名为 `aipaper.json`，运行产物默认写入 `output/aipaper/`。

核心能力：

- 交互式 TUI：无参数运行即可进入完整引导流程。
- 多来源材料解析：支持 PDF、Markdown、TXT、BibTeX，DOCX、URL、CSV 走降级解析路径。
- 学术搜索：内置 Semantic Scholar、Crossref、arXiv、PubMed。
- 文献人工确认：候选文献必须经用户确认后才能被引用。
- 多角色写作：Coordinator 调度 Architect、Writer、Editor 协作。
- 质量门控：claim、evidence、reference key 和 editor verdict 串联校验。
- 断点恢复：WritingProgress 可用 `Esc` 在安全点暂停，并从 checkpoint 恢复。
- 结构化导出：Markdown、DOCX、参考文献、引用追踪、运行报告和质量报告。

## 2. 安装与启动

### 环境要求

- Go 1.25.0 或更高版本
- 至少一个 LLM Provider 的 API Key，或可用的 Ollama 本地服务

### 构建

```bash
git clone <repo-url>
cd Paper-Cli
go build -o paper-cli ./cmd/aipaper-cli
```

Windows：

```powershell
go build -o paper-cli.exe ./cmd/aipaper-cli
go build -o aipaper-cli.exe ./cmd/aipaper-cli
```

如果希望沿用模块名，也可以构建为 `aipaper-cli` 或 `aipaper-cli.exe`：

```bash
go build -o aipaper-cli ./cmd/aipaper-cli
```

### 启动 TUI

```bash
./paper-cli
```

Windows：

```powershell
.\paper-cli.exe
```

无参数运行会进入 TUI。首次启动或配置缺失时，会先进入 ConfigWizard；如果存在未完成 checkpoint，会先进入 RecoverPrompt。

### CLI 命令

```text
paper-cli init     [--workdir DIR] [--config FILE]   初始化工作目录
paper-cli status   [--workdir DIR]                    查看当前状态
paper-cli recover  [--workdir DIR]                    校验 checkpoint 可恢复性
paper-cli config   [--workdir DIR] [--config FILE]    查看合并后配置（API Key 脱敏）
```

## 3. 首次配置 LLM

首次启动 TUI 时，如果没有找到有效配置，会自动进入 ConfigWizard。

### 选择 Provider 模板

| Provider | Type | 默认模型 | Base URL | API Key |
|---|---|---|---|---|
| OpenAI | `openai` | `gpt-5.5` | `https://api.openai.com/v1` | `OPENAI_API_KEY` |
| Anthropic | `anthropic` | `claude-opus-4-8` | `https://api.anthropic.com` | `ANTHROPIC_API_KEY` |
| Ollama | `ollama` | `llama3` | `http://localhost:11434` | 不需要 |
| Custom | 自定义 | 自定义 | 自定义 | `CUSTOM_LLM_API_KEY` |

Ollama 用户需要先确保本地服务已启动。Custom 模板适合 OpenAI-compatible API 或项目内新增的 provider 类型。

### API Key 安全

推荐在配置中使用 `env:VAR_NAME`：

```json
{
  "api_key": "env:OPENAI_API_KEY"
}
```

运行时会读取对应环境变量。如果变量不存在或为空，程序会返回明确错误。`config` 命令、配置摘要和日志会对密钥脱敏。

### 手动配置示例

可以手动创建项目级 `aipaper.json`：

```json
{
  "provider": "openai",
  "model": "gpt-5.5",
  "default_language": "zh-CN",
  "citation_style": "gbt7714",
  "providers": {
    "openai": {
      "type": "openai",
      "api_key": "env:OPENAI_API_KEY",
      "base_url": "https://api.openai.com/v1"
    }
  },
  "roles": {
    "coordinator": { "provider": "openai", "max_turns": 12 },
    "architect": { "provider": "openai" },
    "writer": { "provider": "openai" },
    "editor": { "provider": "openai" }
  }
}
```

配置加载优先级为后者覆盖前者：

1. 全局配置：`~/.aipaper/config.json`
2. 项目配置：`./aipaper.json`
3. 命令行指定：`--config FILE`

## 4. 准备材料

在项目目录下创建 `materials/`，放入参考材料：

```text
materials/
  survey-on-llm.pdf
  notes.md
  related-work.txt
  references.bib
```

支持格式：

| 格式 | 支持程度 | 说明 |
|---|---|---|
| PDF | 完整支持 | 提取文本和元数据 |
| Markdown | 完整支持 | 保留正文和标题结构 |
| TXT | 完整支持 | 按纯文本处理 |
| BibTeX | 完整支持 | 自动提取候选文献 |
| DOCX | 降级支持 | 基础文本提取 |
| URL | 降级支持 | 依赖网络可用性 |
| CSV | 降级支持 | 按行解析 |

MaterialsScan 会自动扫描 `material_dir`。如果目录不存在，TUI 会创建空目录并提示放入材料。单个文件解析失败不会阻断其他文件。

## 5. 生成文章

TUI 主流程如下：

```text
ConfigWizard -> Requirements -> MaterialsScan -> SearchProgress -> References -> WritingProgress -> ExportSummary -> Done
```

恢复场景下，RecoverPrompt 会在进入主流程前出现。

### Requirements

Requirements 表单收集主题、研究问题、综述范围、目标语言、引用格式、质量模式、目标字数、材料目录和搜索设置。

常见字段：

| 字段 | 说明 | 可选值或示例 |
|---|---|---|
| `topic` | 综述主题 | `大语言模型在代码生成中的应用` |
| `research_questions` | 研究问题 | 多行输入 |
| `scope` | 范围约束 | `聚焦 2022-2024 年研究` |
| `language` | 写作语言 | `zh-CN`、`en` |
| `citation_style` | 引用格式 | `gbt7714`、`apa` |
| `quality_mode` | 质量模式 | `fast`、`enhanced`、`strict` |
| `target_words` | 目标字数 | `8000` |
| `material_dir` | 材料目录 | `./materials` |
| `allow_online_search` | 是否联网搜索 | `true`、`false` |
| `search_providers` | 搜索 Provider | `semantic_scholar, crossref` |

默认值包括：`zh-CN`、`gbt7714`、`8000`、`./materials`、允许在线搜索，默认搜索源为 Semantic Scholar 和 Crossref。`quality_mode` 为空时按 `enhanced` 处理。

#### 质量模式

| 模式 | 行为 |
|---|---|
| `fast` | 跳过逐条 claim 验证，风险项以警告为主。 |
| `enhanced` | 默认模式，逐条验证 claim 支撑度；unsupported 或 overstated 会触发重写。 |
| `strict` | 在 enhanced 基础上提高门控严格度，中等风险也可能升级为需修订。 |

无论哪档模式，底线问题都会硬阻断：引用未确认文献、claim 没有 evidence id、evidence 或 claim 指向不存在的 reference key。

### MaterialsScan

MaterialsScan 扫描材料目录并写入：

```text
materials/manifest.json
materials/extracted/
materials/parsed/
```

BibTeX 条目会转为候选引用，后续与在线搜索结果合并。

### SearchProgress

根据写作需求执行学术搜索。默认需求中启用 Semantic Scholar 和 Crossref；项目也包含 arXiv 和 PubMed Provider。

搜索结果写入：

```text
references/candidates.json
```

如果搜索失败，可以跳过搜索并使用 BibTeX 候选继续；但进入写作阶段前至少需要确认一篇文献。

### References

候选文献会在 TUI 中展示，用户可以搜索、确认或拒绝。确认后的 reference key 是后续引用、证据和质量校验的可信来源。

输出：

```text
references/confirmed.json
references/confirmed.bib
references/rejected.json
```

### WritingProgress

WritingProgress 展示运行指标、事件日志、流式内容预览和章节状态。

写作流程：

1. Coordinator 决定下一步工具调用。
2. Architect 生成 `outline/outline.json`。
3. enhanced/strict 模式下，Architect 生成 Evidence Table 和 Section Quality Plan。
4. Writer 按章节生成草稿、claims 和 citation map。
5. Writer Guard 在写入前校验 evidence 与 reference key。
6. Editor 评审章节并写入 review。
7. Editor 验证 claim 后更新 Claim Graph 和 Verification Result。
8. 未通过章节进入重写，最多重写若干轮；通过章节提交为 accepted chapter。
9. 所有章节完成后进入 ExportSummary。

主要中间产物：

```text
outline/outline.json
quality/evidence-table.json
quality/evidence-table.md
quality/section-quality-plan.json
quality/section-quality-plan.md
drafts/{chapter_id}/draft-v{N}.md
drafts/{chapter_id}/claims-v{N}.json
drafts/{chapter_id}/citation-map-v{N}.json
reviews/{chapter_id}/review-v{N}.json
reviews/{chapter_id}/review-v{N}.md
quality/claim-graph.json
quality/claim-graph.md
quality/verification-result.json
accepted/{chapter_id}.md
```

### ExportSummary

导出阶段展示最终文件、导出问题、质量摘要和需人工复核项。DOCX 失败不会阻断其他最终产物。

### Done

完成页展示输出目录和后续建议。建议人工复核 `final/report.md`、`final/quality-report.md` 和 `final/citation-trace.json` 后再进行投稿或发布。

## 6. 输出文件说明

所有输出文件默认位于 `output/aipaper/`。

### 顶层状态文件

| 文件 | 说明 |
|---|---|
| `run.json` | 运行元信息、provider、模型和成本估计等。 |
| `progress.json` | 当前阶段、章节状态和更新时间。 |
| `requirements.json` | 结构化写作需求。 |

### 最终产物

| 文件 | 说明 |
|---|---|
| `final/paper.md` | Markdown 格式综述草稿。 |
| `final/paper.docx` | Word 文档，基础 OOXML 格式。 |
| `final/references.md` | 参考文献列表。 |
| `final/citation-trace.json` | 引用追踪，连接章节、段落、claim 和 reference key。 |
| `final/report.md` | 导出报告，包含输出、问题、质量摘要和兼容提示。 |
| `final/quality-report.md` | 质量引擎报告；只有质量产物可用时生成。 |

`final/citation-trace.json` 示例：

```json
{
  "version": "export-20260613T103000Z",
  "generated_at": "2026-06-13T10:30:00Z",
  "items": [
    {
      "chapter_id": "ch01",
      "paragraph_id": "p001",
      "claim_id": "claim_001",
      "reference_key": "vaswani2017",
      "source_type": "confirmed_reference",
      "editor_verified": true,
      "needs_human_review": false
    }
  ]
}
```

### 材料与引用

| 路径 | 说明 |
|---|---|
| `materials/manifest.json` | 材料扫描清单。 |
| `materials/extracted/` | 提取后的文本。 |
| `materials/parsed/` | 结构化解析结果。 |
| `references/candidates.json` | 候选文献。 |
| `references/confirmed.json` | 已确认文献。 |
| `references/confirmed.bib` | 已确认文献 BibTeX。 |
| `references/rejected.json` | 被拒绝文献。 |

### 草稿、评审和质量产物

| 路径 | 说明 |
|---|---|
| `outline/outline.json` | Architect 生成的大纲。 |
| `drafts/{chapter_id}/draft-v{N}.md` | Writer 第 N 版章节草稿。 |
| `drafts/{chapter_id}/claims-v{N}.json` | 章节 claim 清单。 |
| `drafts/{chapter_id}/citation-map-v{N}.json` | 段落、claim 与 reference key 的映射。 |
| `drafts/{chapter_id}/writer-notes.md` | Writer 备注；仅有备注时生成。 |
| `reviews/{chapter_id}/review-v{N}.json` | Editor 评审结果。 |
| `reviews/{chapter_id}/review-v{N}.md` | Editor 评审 Markdown；仅有内容时生成。 |
| `accepted/{chapter_id}.md` | 已接受章节正文。 |
| `quality/evidence-table.json` | Evidence Table。 |
| `quality/evidence-table.md` | Evidence Table Markdown 视图。 |
| `quality/section-quality-plan.json` | Section Quality Plan。 |
| `quality/section-quality-plan.md` | Section Quality Plan Markdown 视图。 |
| `quality/claim-graph.json` | Claim Graph。 |
| `quality/claim-graph.md` | Claim Graph Markdown 视图。 |
| `quality/verification-result.json` | Editor verifier 的 claim verdict。 |

`quality/evidence-table.json` 示例：

```json
{
  "generated_at": "2026-06-13T10:30:00Z",
  "items": [
    {
      "id": "ev_001",
      "reference_key": "vaswani2017",
      "material_id": "material_003",
      "depth": "snippet",
      "topics": ["transformer", "attention"],
      "key_findings": ["Self-attention enables parallel sequence modeling"],
      "excerpt": "The Transformer model architecture relies entirely on attention mechanisms...",
      "confidence": "high"
    }
  ]
}
```

## 7. 中断与恢复

### 安全暂停与退出

在 WritingProgress 界面：

1. 按 `Esc` 请求在最近安全点暂停。
2. 系统等待当前安全边界/checkpoint 完成后进入 Paused。
3. Paused 状态下按 `Enter` 继续生成。
4. 底部输入框可提交追加指令，指令会排队并在最近安全边界注入后续生成。
5. `Ctrl+C` 用于退出程序/退出确认。

不要直接强制关闭终端窗口；等待暂停或退出流程完成可以降低 checkpoint 损坏风险。

### 恢复运行

下次启动 TUI 时，如果检测到未完成 checkpoint，会进入 RecoverPrompt：

```text
[c] 继续写作  [r] 重新开始  [q] 退出
```

选项说明：

- 继续写作：从 checkpoint 恢复，进入 WritingProgress。
- 重新开始：二次确认后回到前序步骤，不主动删除已有文件。
- 退出：结束 TUI。

### CLI 校验

```bash
./paper-cli recover --workdir .
```

该命令检查 `checkpoints/latest.json` 指向的产物是否完整、哈希是否匹配。返回非零退出码表示不可恢复。

## 8. 常见问题

### API Key 错误

现象：WritingProgress 报 authentication 或缺少环境变量。

处理：

1. 检查环境变量，例如 `echo $OPENAI_API_KEY`。
2. 检查 `aipaper.json` 中的 `api_key` 是否使用正确的 `env:VAR_NAME`。
3. 使用 `./paper-cli config` 查看当前生效配置，输出会脱敏密钥。

### Ollama 不可用

现象：本地模型调用失败或连接 `http://localhost:11434` 失败。

处理：

1. 确认 Ollama 服务已启动。
2. 确认配置中的 base URL 与本机服务一致。
3. 确认所选模型已经拉取到本地。

### materials 为空

现象：MaterialsScan 提示没有找到文件。

处理：把 PDF、Markdown、TXT 或 BibTeX 放入 `materials/` 后重新扫描。也可以跳过材料扫描，仅依赖在线搜索或已有候选继续。

### 学术搜索失败

现象：SearchProgress 中搜索 Provider 全部失败。

处理：检查网络、代理和目标服务可用性；也可以跳过搜索并使用 BibTeX 候选继续。进入写作前仍至少需要确认一篇文献。

### Context 过长

现象：写作阶段报 context length exceeded。

处理：减少材料数量或体积，降低目标字数，或在 `roles` 中为 Writer、Editor 配置更大上下文窗口的模型。

### 质量门控过严

现象：章节反复重写或被标记为需修订。

处理：优先查看 `final/quality-report.md` 或 `quality/claim-graph.md` 中的 unsupported、overstated、partially_supported 项。确认证据不足时补充材料或调整表述；如果只是快速草稿，可将 `quality_mode` 调整为 `fast` 后重新运行。

### DOCX 导出失败

现象：ExportSummary 提示 Word 文档生成失败。

处理：这不影响其他文件。`final/paper.md`、`final/references.md`、`final/citation-trace.json` 和 `final/report.md` 仍可用。需要复杂排版时，建议从 Markdown 使用 Pandoc 等工具转换。

### Windows 双击后窗口闪退

处理：在 PowerShell 或 CMD 中运行可执行文件，查看具体错误：

```powershell
cd path\to\Paper-Cli
.\paper-cli.exe
```

## 9. 高级命令

### init

```bash
paper-cli init --workdir .
paper-cli init --workdir . --config ./aipaper.json
```

初始化 Store 布局，创建 `output/aipaper/` 下的必要目录和状态文件。

### status

```bash
paper-cli status --workdir .
```

输出当前阶段、step、章节进度和更新时间。未初始化时输出未初始化状态。

### recover

```bash
paper-cli recover --workdir .
```

校验 checkpoint 和产物哈希，判断是否可恢复。

### config

```bash
paper-cli config --workdir . --config ./aipaper.json
```

输出加载的配置文件列表和合并后的配置对象。API Key 会显示为脱敏值。

## 10. 开发与测试

常用检查：

```bash
go build ./...
go test ./...
go test -v ./internal/e2e
```

真实 LLM 冒烟工具：

```bash
go run ./tools/real-tui-smoke
```

该工具使用 `SMOKE_API_KEY`、`SMOKE_BASE_URL`、`SMOKE_MODEL` 等环境变量，并把配置中的密钥写成 `env:SMOKE_API_KEY`，避免把真实密钥落盘。

## 11. 环境变量参考

常用变量：

| 变量名 | 说明 |
|---|---|
| `OPENAI_API_KEY` | OpenAI API Key |
| `ANTHROPIC_API_KEY` | Anthropic API Key |
| `CUSTOM_LLM_API_KEY` | 自定义 Provider API Key |
| `SMOKE_API_KEY` | 真实 LLM 冒烟测试 API Key |
| `SMOKE_BASE_URL` | 真实 LLM 冒烟测试 Base URL |
| `SMOKE_MODEL` | 真实 LLM 冒烟测试模型 |

如果项目中提供 `.env.example`，以其中变量列表为准。