# Paper-Cli

`Paper-Cli` 是一个面向学术综述 / 文献综述写作的 AI Agent 命令行工具。它通过交互式 TUI 收集写作需求、解析用户材料、搜索学术文献、确认引用，并调度多角色 AI Agent 协作完成「大纲 → 草稿 → 评审 → 重写 → 导出」流程，最终生成可追溯、可复核的综述草稿。

代码入口仍位于 `cmd/aipaper-cli`，默认配置文件仍为 `aipaper.json`，默认输出目录仍为 `output/aipaper/`。

## 核心特性

### 交互式工作流

- 无参数运行进入 TUI，引导完成配置、需求、材料、检索、引用确认、写作和导出。
- 首次启动或配置缺失时进入 ConfigWizard，支持 OpenAI、Anthropic、Ollama 和自定义 Provider。
- WritingProgress 支持 `Esc` 在安全点暂停、`Enter` 继续；下次启动检测到 checkpoint 后进入 RecoverPrompt。

### 材料与文献

- 支持 PDF、Markdown、TXT、BibTeX 等材料解析，DOCX、URL、CSV 走降级解析路径。
- 内置 Semantic Scholar、Crossref、arXiv、PubMed 学术搜索 Provider。
- 候选文献必须经过用户确认后才能被 AI 引用，确认结果写入 `references/confirmed.json` 和 `references/confirmed.bib`。

### 多角色 AI 协作

- `Coordinator`：主控调度器，驱动各阶段决策与工具调用。
- `Architect`：生成大纲、证据表和章节质量计划。
- `Writer`：按章节起草内容，每个 claim 绑定 evidence 与 reference key。
- `Editor`：评审引用一致性、论断支撑度和可读性，并触发重写。

### 质量保障

- 质量模式：`fast`、`enhanced`（默认）、`strict`。
- Writer Guard 在草稿写入前校验 claim 是否绑定证据、证据是否存在、引用 key 是否来自已确认文献。
- Editor 验证结果写入 Claim Graph 和 `quality/verification-result.json`。
- 导出阶段生成 citation trace、报告和可选的质量引擎报告。

### 质量引擎与 Paper Quality Policy

运行时内置一份本地论文质量策略（`paper-cli-paper-quality-v1`），由 `internal/quality/paper_quality_policy.go` 提供。该策略在执行时被注入到各角色 prompt，而不是运行时读取外部 `docs/skills` 文件：

- `Coordinator`：硬性规则与角色边界（Host 做机器校验，Coordinator 依据工具事实做流程决策，角色 Agent 在既有 JSON 契约内做语义判断）。
- `Architect`：叙事契约（围绕一条证据有界的论点设计论文，而非罗列材料）、大纲去重、证据深度约束。
- `Writer`：每个重要 claim 必须出现在 `claims[]` 并绑定已确认 `evidence_ids`；措辞强度匹配证据深度；禁止把「证据不足/待验证/只能提出框架」写成正文体。
- `Verifier`：claim 支撑度判定（descriptive / comparative / causal / generalization / methodological / limitation / reproducibility），同对象、同关系、同范围才视为支撑；仅输出既有 `ClaimVerdict` 契约。
- `Editor`：区分语言问题、证据问题与结构问题；unsupported / overstated / 高风险 claim 必须给出带位置、问题、指令与 `suggested_evidence_ids` 的重写指令；需要新证据或领域判断时标记人工复核或 gap。

硬性门控在所有模式下都不能静默通过：未确认引用、claim 缺少 evidence id、evidence 指向不存在或未确认 reference key。`quality/verification-result.json` 记录 verifier 判定，`quality/claim-graph.*` 记录 claim 图。

## 快速开始

### 前置要求

- Go 1.25.0 或更高版本
- 至少一个 LLM Provider：OpenAI、Anthropic、Ollama 本地服务，或兼容 OpenAI API 的自定义服务

### 构建

```bash
git clone <repo-url>
cd Paper-Cli
go build -o paper-cli ./cmd/aipaper-cli
```

Windows：

```powershell
go build -o paper-cli.exe ./cmd/aipaper-cli
```

如果希望沿用模块名作为二进制名，也可以把 `paper-cli` 改为 `aipaper-cli`。

### 启动 TUI

```bash
./paper-cli
```

Windows：

```powershell
.\paper-cli.exe
```

首次启动会进入 ConfigWizard；配置完成后进入主流程：

```text
ConfigWizard -> Requirements -> MaterialsScan -> SearchProgress -> References -> WritingProgress -> ExportSummary -> Done
```

如果存在未完成 checkpoint，启动时会先进入 RecoverPrompt，让你选择继续、重新开始或退出。

### CLI 命令

```bash
paper-cli init     [--workdir DIR] [--config FILE]
paper-cli status   [--workdir DIR]
paper-cli recover  [--workdir DIR]
paper-cli config   [--workdir DIR] [--config FILE]
```

`config` 命令会展示合并后的配置，并对 API Key 脱敏。

## 配置

ConfigWizard 当前内置模板如下：

| Provider  | Type          | 默认模型            | Base URL                      | API Key                |
| --------- | ------------- | ------------------- | ----------------------------- | ---------------------- |
| OpenAI    | `openai`    | `gpt-5.5`         | `https://api.openai.com/v1` | `OPENAI_API_KEY`     |
| Anthropic | `anthropic` | `claude-opus-4-8` | `https://api.anthropic.com` | `ANTHROPIC_API_KEY`  |
| Ollama    | `ollama`    | `llama3`          | `http://localhost:11434`    | 不需要                 |
| Custom    | 自定义        | 自定义              | 自定义                        | `CUSTOM_LLM_API_KEY` |

推荐使用 `env:VAR_NAME` 引用环境变量，避免把密钥明文写入 `aipaper.json`。运行时会解析 `env:` 值；如果环境变量不存在，会返回明确错误。

示例：

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

配置加载优先级为：全局配置 `~/.aipaper/config.json`、项目配置 `./aipaper.json`、命令行 `--config FILE`。

## 写作需求

Requirements 表单会生成结构化 `requirements.json`，主要字段包括：

| 字段                    | 说明             | 示例                               |
| ----------------------- | ---------------- | ---------------------------------- |
| `topic`               | 综述主题         | `大语言模型在代码生成中的应用`   |
| `research_questions`  | 研究问题         | 多行输入                           |
| `scope`               | 综述范围         | `聚焦 2022-2024 年研究`          |
| `language`            | 目标语言         | `zh-CN` 或 `en`                |
| `citation_style`      | 引用格式         | `gbt7714` 或 `apa`             |
| `quality_mode`        | 质量模式         | `fast`、`enhanced`、`strict` |
| `target_words`        | 目标字数         | `8000`                           |
| `material_dir`        | 材料目录         | `./materials`                    |
| `allow_online_search` | 是否允许联网搜索 | `true` 或 `false`              |
| `search_providers`    | 搜索源           | `semantic_scholar, crossref`     |

默认需求使用 `zh-CN`、`gbt7714`、`8000` 字、`./materials`、允许在线搜索，默认搜索源为 Semantic Scholar 和 Crossref。`quality_mode` 为空时按 `enhanced` 处理。

### 质量模式

| 模式         | 行为                                                                       |
| ------------ | -------------------------------------------------------------------------- |
| `fast`     | 跳过逐条 claim 验证，风险项以警告为主。                                    |
| `enhanced` | 默认模式，逐条验证 claim 支撑度，unsupported 或 overstated 会触发重写。    |
| `strict`   | 在 enhanced 基础上提高门控严格度，浅证据支撑强结论等中等风险也会升级处理。 |

所有模式都遵守硬性门控：未确认引用、claim 缺少 evidence id、evidence 指向不存在或未确认 reference key，都不能静默通过。

## 工作流程

### 1. MaterialsScan

扫描 `material_dir`，写入材料清单和解析产物。BibTeX 中的条目会转成候选引用，与后续搜索结果合并。

常见输出：

```text
materials/manifest.json
materials/extracted/
materials/parsed/
```

### 2. SearchProgress

根据需求查询学术搜索 Provider，并将候选文献写入：

```text
references/candidates.json
```

如果搜索失败，可以跳过搜索并使用材料中提取的 BibTeX 候选；但进入写作前至少需要确认一篇文献。

### 3. References

用户在 TUI 中确认或拒绝候选文献。确认后的文献是后续引用和证据校验的唯一可信 reference key 来源。

```text
references/confirmed.json
references/confirmed.bib
references/rejected.json
```

### 4. WritingProgress

WritingProgress 会展示运行指标、事件日志、流式正文预览和章节状态。核心过程如下：

1. Architect 生成 `outline/outline.json`。
2. enhanced/strict 模式下，Architect 生成 `quality/evidence-table.json`、`quality/evidence-table.md`、`quality/section-quality-plan.json`、`quality/section-quality-plan.md`。
3. Writer 按章节写入 `drafts/{chapter_id}/draft-v{N}.md`、`claims-v{N}.json`、`citation-map-v{N}.json`。
4. Writer Guard 校验 claim、evidence 和 reference key 后才允许写入草稿 bundle。
5. Editor 写入 `reviews/{chapter_id}/review-v{N}.json`，必要时也写入 `review-v{N}.md`。
6. Editor 验证 claim 后更新 `quality/claim-graph.json`、`quality/claim-graph.md` 和 `quality/verification-result.json`。
7. 通过评审的章节提交到 `accepted/{chapter_id}.md`。

### 5. ExportSummary

导出阶段生成最终文件列表、导出问题、质量摘要和需人工复核项。DOCX 导出失败不会阻断 Markdown、参考文献、citation trace 和 report 输出。

## 输出文件结构

所有运行产物默认位于 `output/aipaper/`。

```text
output/aipaper/
├── run.json
├── progress.json
├── requirements.json
├── materials/
│   ├── manifest.json
│   ├── extracted/
│   └── parsed/
├── references/
│   ├── candidates.json
│   ├── confirmed.json
│   ├── confirmed.bib
│   └── rejected.json
├── outline/
│   └── outline.json
├── drafts/
│   └── {chapter_id}/
│       ├── draft-v1.md
│       ├── claims-v1.json
│       ├── citation-map-v1.json
│       └── writer-notes.md
├── reviews/
│   └── {chapter_id}/
│       ├── review-v1.json
│       └── review-v1.md
├── accepted/
│   └── {chapter_id}.md
├── quality/
│   ├── evidence-table.json
│   ├── evidence-table.md
│   ├── section-quality-plan.json
│   ├── section-quality-plan.md
│   ├── claim-graph.json
│   ├── claim-graph.md
│   └── verification-result.json
├── checkpoints/
│   ├── latest.json
│   └── checkpoint-*.json
└── final/
    ├── paper.md
    ├── paper.docx
    ├── references.md
    ├── citation-trace.json
    ├── report.md
    └── quality-report.md
```

部分质量文件只会在 enhanced/strict 路径或质量产物可用时生成；旧项目或兼容模式下，`final/quality-report.md` 可能不生成，但 `final/report.md` 会记录兼容提示。

### `final/quality-report.md`

质量引擎报告，基于已加载的质量产物在本地渲染（不调用 LLM）。报告头部记录 `Paper Quality policy` 版本（`paper-cli-paper-quality-v1`）、质量模式、整体状态、claim 校验数与 verifier 判定数，正文包含以下小节：

- **Hard Gate Summary**：硬阻断项与风险发现表。
- **Evidence Depth Distribution**：`metadata_only` / `abstract` / `snippet` / `fulltext_excerpt` 证据深度分布。
- **Claim Support Summary**：`supported` / `partially_supported` / `unsupported` / `overstated` / `skipped` / `unverified` 计数。
- **Unsupported / Overstated Claims**：逐条列出问题 claim 及 verifier note。
- **Evidence Sufficiency and Content Signals**：证据不足、必需证据未使用、metadata_only 独占支撑、低内容信号等发现。
- **Human Action Items**：可执行的人工处理建议（重写/弱化/补证据/人工复核）。
- **Needs Human Review**：strict 模式下高优先项、待复核章节与发现表。
- **Rewrite Summary**：各章重写轮次、必需/可选重写指令数与收敛状态（`converged` / `needs_revision` / `needs_human_review`）。
- **Suggested Next Human Edits**：发布前建议的人工修订步骤。

### `final/citation-trace.json`

Citation trace 是扁平列表，每条记录描述一个段落中的 claim 与已确认文献之间的关系：

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

### `quality/evidence-table.json`

Evidence table 记录证据与文献、材料、主题、发现、证据深度和置信度之间的关系：

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

## 断点恢复

WritingProgress 中按 `Esc` 会请求在最近安全点暂停；暂停完成后可按 `Enter` 从最新 checkpoint 继续。底部输入框可随时提交追加指令，这些指令会排队并在最近安全边界注入后续生成。`Ctrl+C` 用于退出程序/退出确认；不要直接强制关闭终端，等待暂停或退出流程完成可以降低进度损坏风险。

下次启动时，如果检测到未完成 checkpoint，会进入 RecoverPrompt：

```text
[c] 继续写作  [r] 重新开始  [q] 退出
```

也可以通过命令校验恢复状态：

```bash
./paper-cli recover --workdir .
```

该命令会检查 `checkpoints/latest.json` 指向的产物是否存在且哈希匹配；不可恢复时返回非零退出码。

## 开发与测试

```bash
go build ./...
go test ./...
go test -v ./internal/e2e
```

真实 LLM 冒烟工具位于 `tools/real-tui-smoke/`，使用 `SMOKE_API_KEY`、`SMOKE_BASE_URL`、`SMOKE_MODEL` 等环境变量注入密钥，不会把真实密钥写入配置文件。

常见模块：

| 路径                     | 职责                                                                                          |
| ------------------------ | --------------------------------------------------------------------------------------------- |
| `cmd/aipaper-cli/`     | CLI/TUI 入口                                                                                  |
| `internal/cli/`        | `init`、`status`、`recover`、`config` 命令                                            |
| `internal/tui/`        | ConfigWizard、Requirements、MaterialsScan、SearchProgress、References、WritingProgress 等界面 |
| `internal/app/`        | Bootstrap、AgentRuntime、角色 Runner                                                          |
| `internal/config/`     | 配置加载、合并、校验、脱敏                                                                    |
| `internal/store/`      | Store 路径、布局、原子写入、哈希                                                              |
| `internal/materials/`  | 材料扫描与解析                                                                                |
| `internal/search/`     | 学术搜索 Provider                                                                             |
| `internal/references/` | 候选、确认、拒绝文献管理                                                                      |
| `internal/agent/`      | Coordinator、角色工具、质量协议                                                               |
| `internal/quality/`    | Evidence、Section Plan、Claim Graph、Verification、Gate                                       |
| `internal/artifacts/`  | 草稿、claim、citation map、review、accepted chapter 写入                                      |
| `internal/export/`     | Markdown、DOCX、引用追踪和报告导出                                                            |

## 常见问题

### API Key 认证失败

1. 确认环境变量已设置，例如 `echo $OPENAI_API_KEY`。
2. 确认配置中使用的是正确的 `env:VAR_NAME`。
3. 运行 `./paper-cli config` 查看生效配置，输出会自动脱敏密钥。

### 材料目录为空

把 PDF、Markdown、TXT 或 BibTeX 文件放入 `materials/` 后重新扫描。也可以跳过材料扫描，仅依赖学术搜索或 BibTeX 候选继续。

### 学术搜索失败

检查网络和 Provider 可用性。搜索失败时可以跳过搜索，但 References 阶段仍至少需要确认一篇文献才能进入写作。

### Context 过长

减少材料数量或体积，降低目标字数，或在 `roles` 中为 Writer/Editor 指定更大上下文窗口的模型。

### Word 文档导出失败

DOCX 使用基础导出器。导出失败时，`final/paper.md`、`final/references.md`、`final/citation-trace.json` 和 `final/report.md` 仍可用；可以再用 Pandoc 等工具从 Markdown 转换。

### Windows 双击后窗口闪退

在 PowerShell 或 CMD 中运行 `paper-cli.exe` 查看错误信息，例如缺少配置、环境变量或本地服务不可用。

## 项目边界

当前版本不承诺：

- 生成可直接投稿的最终论文；仍需要人工复核和润色。
- 完美处理所有 GB/T 7714、APA 排版细节；DOCX 为基础格式。
- OCR 扫描 PDF 或深度理解图片、表格、公式。
- Web UI、多用户后台或云同步。
- 自动背书联网文献真实性；文献仍需用户确认。
- 保证所有学术论断完全正确；质量门控检查证据链和引用一致性，不替代人工学术判断。

## 文档索引

- [用户指南（详细版）](docs/user-guide.md)：完整安装、配置、材料准备、生成、导出、恢复说明。
- [需求与架构](docs/需求与架构.md)：设计背景、技术选型、模块设计。
- [接口契约索引](docs/interfaces/_index.md)：JSON Schema 与契约定义。
- [Paper Quality Skill 运行时设计](docs/superpowers/specs/2026-06-20-paper-quality-skill-runtime-design.md)：质量策略本地化注入与质量引擎设计。
- [质量接口契约](docs/interfaces/quality.md)：Evidence、Section Plan、Claim Graph、Verification、Gate 契约。
- [开发进度](docs/开发进度.md)：开发日志与版本历史。

## 许可证

本项目基于 [MIT License](LICENSE) 开源。任何人可自由使用、复制、修改、合并、发布、分发、再授权或销售本软件，仅需在所有副本中保留版权声明与本许可证声明。

版权所有 © 2026 thisxiaoyuQAQ。

## 致谢

本项目使用 Bubble Tea、agentcore 以及 Semantic Scholar、Crossref、arXiv、PubMed 等学术检索服务。
