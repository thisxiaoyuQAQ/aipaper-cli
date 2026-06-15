# Paper-Cli 项目结构图

本文档基于 codegraph MCP 索引和当前仓库结构整理，用于快速理解 `Paper-Cli` 的模块边界、运行链路、数据产物和质量引擎关系。

## 1. 项目定位

`Paper-Cli` 是一个面向学术综述 / 文献综述写作的 Go CLI/TUI 应用。它通过交互式 TUI 收集写作需求、扫描本地材料、执行学术搜索、确认引用，并调度 Coordinator、Architect、Writer、Editor 多角色 AI Agent 完成「大纲 → 草稿 → 评审 → 重写 → 导出」流程。

关键约定：

- 代码入口：`cmd/aipaper-cli/`
- 默认配置文件：`aipaper.json`
- 默认输出目录：`output/aipaper/`
- 默认材料目录：`./materials`
- 默认语言：`zh-CN`
- 默认引用格式：`gbt7714`
- 默认质量模式：`enhanced`

## 2. 顶层目录

```text
.
├── cmd/
│   └── aipaper-cli/              # CLI/TUI 程序入口
├── internal/                     # 业务实现，按模块隔离
│   ├── agent/                    # Coordinator、角色工具、质量协议
│   ├── app/                      # Bootstrap、AgentRuntime、恢复与角色 Runner
│   ├── artifacts/                # 草稿、claim、citation map、review、accepted 写入
│   ├── checkpoint/               # checkpoint 持久化、校验、恢复引用
│   ├── cli/                      # init/status/recover/config 命令
│   ├── config/                   # 配置 schema、加载、合并、脱敏
│   ├── contracts/                # 跨模块 JSON 契约和运行事件类型
│   ├── e2e/                      # 端到端与质量流测试
│   ├── export/                   # Markdown、DOCX、citation trace、报告导出
│   ├── materials/                # 材料扫描、文本抽取、解析
│   ├── quality/                  # Evidence、Section Plan、Claim Graph、Verification、Gate
│   ├── references/               # BibTeX、候选、确认、拒绝、去重
│   ├── search/                   # Semantic Scholar、Crossref、arXiv、PubMed Provider
│   ├── store/                    # 输出路径、布局、原子写入、哈希
│   └── tui/                      # Bubble Tea TUI RootModel 与各屏幕
├── docs/                         # 用户文档、接口契约、设计文档、验收记录
├── fixtures/                     # 材料解析、review、quality 测试夹具
├── tools/
│   ├── real-before-after/        # 真实 LLM 前后对比工具
│   └── real-tui-smoke/           # 真实 TUI/LLM 冒烟工具
├── vault/                        # 分模块开发记录
├── README.md                     # 项目总览
├── go.mod
└── go.sum
```

## 3. 程序入口与 CLI 层

### `cmd/aipaper-cli/`

职责：

- 提供可执行程序入口。
- 无参数运行时启动 TUI。
- 有参数运行时转交 `internal/cli` 解析命令。

常见运行方式：

```bash
go run ./cmd/aipaper-cli
go build -o paper-cli ./cmd/aipaper-cli
```

### `internal/cli/`

职责：

- 解析并执行非交互 CLI 命令。
- 暴露初始化、状态查看、恢复校验、配置查看能力。

主要命令：

```text
paper-cli init     [--workdir DIR] [--config FILE]
paper-cli status   [--workdir DIR]
paper-cli recover  [--workdir DIR]
paper-cli config   [--workdir DIR] [--config FILE]
```

模块边界：

- 不直接实现写作流程。
- 通过 `internal/app`、`internal/store`、`internal/config`、`internal/checkpoint` 完成具体操作。
- `config` 输出必须脱敏 API Key。

## 4. TUI 层结构

### `internal/tui/app/`

职责：

- 管理 RootModel。
- 串联各屏幕状态迁移。
- 处理启动状态探测、恢复入口和全流程事件桥接。

主流程：

```text
ConfigWizard -> Requirements -> MaterialsScan -> SearchProgress -> References -> WritingProgress -> ExportSummary -> Done
```

恢复场景：

```text
RecoverPrompt -> WritingProgress
RecoverPrompt -> Requirements 或前序流程
RecoverPrompt -> Quit
```

### `internal/tui/configwizard/`

职责：

- 首次启动或配置缺失时收集 LLM Provider 配置。
- 生成 `config.Config`。
- 为 Coordinator、Architect、Writer、Editor 初始化相同 Provider/Model 角色配置。

内置 Provider 模板：

| Provider | Type | 默认模型 | Base URL | API Key |
|---|---|---|---|---|
| OpenAI | `openai` | `gpt-5.5` | `https://api.openai.com/v1` | `OPENAI_API_KEY` |
| Anthropic | `anthropic` | `claude-opus-4-8` | `https://api.anthropic.com` | `ANTHROPIC_API_KEY` |
| Ollama | `ollama` | `llama3` | `http://localhost:11434` | 不需要 |
| Custom | 自定义 | 自定义 | 自定义 | `CUSTOM_LLM_API_KEY` |

配置向导输出示意：

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
    "coordinator": { "provider": "openai", "model": "gpt-5.5" },
    "architect": { "provider": "openai", "model": "gpt-5.5" },
    "writer": { "provider": "openai", "model": "gpt-5.5" },
    "editor": { "provider": "openai", "model": "gpt-5.5" }
  }
}
```

### `internal/tui/requirements/`

职责：

- 收集写作需求。
- 校验语言、引用格式、目标字数、质量模式、搜索设置。
- 写入结构化 requirements。

关键字段：

| 字段 | 说明 |
|---|---|
| `topic` | 综述主题 |
| `research_questions` | 研究问题 |
| `scope` | 范围约束 |
| `language` | `zh-CN` 或 `en` |
| `citation_style` | `gbt7714` 或 `apa` |
| `quality_mode` | `fast`、`enhanced`、`strict` |
| `target_words` | 目标字数 |
| `material_dir` | 材料目录 |
| `allow_online_search` | 是否联网搜索 |
| `search_providers` | 搜索源列表 |

默认要求由 RootModel 构造：

- `language`: `zh-CN`
- `citation_style`: `gbt7714`
- `target_words`: `8000`
- `material_dir`: `./materials`
- `allow_online_search`: `true`
- `search_providers`: Semantic Scholar、Crossref
- `quality_mode`: 空值时按 `enhanced` 处理

### `internal/tui/materials/`

职责：

- 展示材料扫描进度。
- 调用 `internal/materials` 解析用户材料。
- 将 BibTeX 条目转入候选引用链路。

主要输出：

```text
materials/manifest.json
materials/extracted/
materials/parsed/
```

### `internal/tui/search/`

职责：

- 展示在线学术搜索进度。
- 根据 requirements 调用 `internal/search` Provider。
- 合并本地材料候选与在线搜索候选。

主要输出：

```text
references/candidates.json
```

### `internal/tui/references/`

职责：

- 展示候选文献。
- 支持搜索、确认、拒绝候选。
- 确认后的 reference key 作为后续写作和质量校验的可信来源。

主要输出：

```text
references/confirmed.json
references/confirmed.bib
references/rejected.json
```

### `internal/tui/writing/`

职责：

- 展示写作运行指标、事件日志、流式内容预览、章节状态。
- 桥接 TUI 与 `AgentRuntime`。
- 支持 Ctrl+C 安全停止并等待 checkpoint 完成。

写作链路：

```text
Coordinator
  -> Architect: outline / evidence table / section quality plan
  -> Writer: chapter draft / claims / citation map
  -> Writer Guard: evidence 与 confirmed references 校验
  -> Editor: review / claim verification
  -> Quality Gate: pass / rewrite / human review
  -> accepted chapter
```

### `internal/tui/exportsummary/` 与 `internal/tui/done/`

职责：

- 展示导出结果、质量摘要、导出问题和人工复核项。
- 完成页展示输出目录和后续操作建议。

## 5. 应用运行时

### `internal/app/`

职责：

- 初始化工作目录和 Store。
- 加载配置并构造 Runtime。
- 处理 checkpoint 恢复。
- 连接 Coordinator 与角色 Runner。

核心对象：

```text
AgentRuntime
AgentRuntimeOptions
RoleRuntimeConfig
```

运行时构造关系：

```text
config.Config
  -> ResolveRoleRuntime(role)
  -> NewChatModel(provider, model, api_key, base_url)
  -> store.New(workDir)
  -> agent.RoleTools(store, writer, architect, editor)
  -> CoordinatorSystemPrompt(recoveryPrompt)
  -> agentcore.NewAgent(...)
```

### 角色配置解析

配置结构支持全局 Provider/Model 与角色级覆盖：

```json
{
  "provider": "openai",
  "model": "gpt-5.5",
  "providers": {
    "openai": {
      "type": "openai",
      "api_key": "env:OPENAI_API_KEY",
      "base_url": "https://api.openai.com/v1"
    }
  },
  "roles": {
    "writer": {
      "provider": "openai",
      "model": "gpt-5.5",
      "max_turns": 8,
      "temperature": 0.4
    }
  }
}
```

`env:VAR_NAME` 密钥在运行时解析；环境变量不存在或为空时返回错误。

## 6. Agent 层

### `internal/agent/`

职责：

- 定义 AI Agent 角色、Coordinator 决策协议、工具调用协议和质量写作协议。
- 将 Architect、Writer、Editor 能力包装成 Coordinator 可调用工具。
- 实现 Writer Guard，避免无证据或未确认引用进入草稿。

角色边界：

| 角色 | 职责 |
|---|---|
| Coordinator | 主控调度，决定下一步工具调用和流程推进 |
| Architect | 生成大纲、证据表、章节质量计划 |
| Writer | 生成章节草稿、claims、citation map |
| Editor | 评审章节、验证 claim、提出重写建议 |

Writer Guard 硬性规则：

- 每个 claim 必须绑定至少一个 evidence id。
- claim 绑定的 evidence id 必须存在于 Evidence Table。
- claim 引用的 reference key 必须来自 `references/confirmed.json`。
- 违规时草稿 bundle 不写入。

## 7. 配置系统

### `internal/config/`

职责：

- 定义配置 schema。
- 从全局配置、项目配置和命令行指定配置加载并合并。
- 校验 Provider、Model、Role 配置。
- 脱敏输出 API Key。

加载优先级：

```text
~/.aipaper/config.json
  < ./aipaper.json
  < --config FILE
```

主要结构：

```text
Config
  Provider
  Model
  DefaultLanguage
  CitationStyle
  Providers map[string]ProviderConfig
  Roles     map[string]RoleConfig

ProviderConfig
  Type
  APIKey
  BaseURL
  Models
  Extra

RoleConfig
  Provider
  Model
  MaxTurns
  Temp
```

## 8. Store 与输出布局

### `internal/store/`

职责：

- 管理 `output/aipaper/` 根目录。
- 创建必要目录。
- 提供原子写入、create-only 写入、覆盖写入。
- 计算并记录 SHA256。
- 处理 Windows 与 Unix 文件替换差异。

默认 Store root：

```text
filepath.Join(workDir, contracts.DefaultOutputDir, contracts.DefaultProject)
```

文档化后表现为：

```text
output/aipaper/
```

必要目录：

```text
checkpoints/
materials/extracted/
materials/parsed/
references/
outline/
drafts/
reviews/
final/
```

实际运行还会产生：

```text
accepted/
quality/
```

## 9. 材料解析

### `internal/materials/`

职责：

- 扫描 `material_dir`。
- 生成材料 manifest。
- 提取文本和元数据。
- 对不同文件类型走对应解析路径。

支持范围：

| 格式 | 处理方式 |
|---|---|
| PDF | 提取文本和元数据 |
| Markdown | 保留正文和标题结构 |
| TXT | 纯文本解析 |
| BibTeX | 提取候选文献 |
| DOCX | 降级解析 |
| URL | 降级解析 |
| CSV | 按行解析 |

测试夹具位于：

```text
fixtures/materials/
```

## 10. 学术搜索

### `internal/search/`

职责：

- 定义搜索请求、结果、Provider 接口。
- 实现多学术源查询。
- 封装 HTTP 请求和错误处理。

内置 Provider：

| Provider | 用途 |
|---|---|
| Semantic Scholar | 默认搜索源之一 |
| Crossref | 默认搜索源之一 |
| arXiv | 预印本搜索 |
| PubMed | 生物医学文献搜索 |

搜索阶段输出：

```text
references/candidates.json
```

## 11. 文献管理

### `internal/references/`

职责：

- 解析 BibTeX。
- 合并和去重候选文献。
- 管理确认与拒绝状态。
- 写出 confirmed BibTeX。

关键产物：

```text
references/candidates.json
references/confirmed.json
references/confirmed.bib
references/rejected.json
```

写作阶段只允许引用 confirmed references。

## 12. 写作产物

### `internal/artifacts/`

职责：

- 定义草稿、claims、citation map、review、accepted chapter 的路径规则。
- 校验 draft bundle。
- 写入章节版本产物。
- 将通过评审的章节提交为 accepted chapter。
- 兼容质量引擎产物加载。

主要路径：

```text
drafts/{chapter_id}/draft-v{N}.md
drafts/{chapter_id}/claims-v{N}.json
drafts/{chapter_id}/citation-map-v{N}.json
drafts/{chapter_id}/writer-notes.md
reviews/{chapter_id}/review-v{N}.json
reviews/{chapter_id}/review-v{N}.md
accepted/{chapter_id}.md
```

## 13. 质量引擎

### `internal/quality/`

职责：

- 建模证据、章节质量计划、claim 图谱、验证结果和质量门控。
- 保存 JSON 与 Markdown 双格式质量产物。
- 为 export 阶段提供质量报告输入。

质量产物关系：

```text
Confirmed References + Materials
  -> Evidence Table
  -> Section Quality Plan
  -> Writer Claims
  -> Claim Graph
  -> Verification Result
  -> Gate Outcome
  -> final/quality-report.md
```

主要文件：

```text
quality/evidence-table.json
quality/evidence-table.md
quality/section-quality-plan.json
quality/section-quality-plan.md
quality/claim-graph.json
quality/claim-graph.md
quality/verification-result.json
```

### Evidence Table

记录证据与文献、材料、主题、发现、方法、局限、置信度之间的关系。

典型字段：

```text
id
reference_key
material_id
depth
topics
key_findings
method
subjects
limitations
excerpt
confidence
coverage
risk_flags
```

### Section Quality Plan

描述每个章节需要覆盖的证据、风险点、引用策略和质量要求。

### Claim Graph

连接章节 claim、evidence、reference key、支撑状态和风险状态。

### Verification Result

Editor verifier 的逐 claim verdict，保存到：

```text
quality/verification-result.json
```

典型字段：

```text
claim_id
support
risk_level
verifier_note
```

### Gate Outcome

根据质量模式、验证结果和风险项决定章节是否通过、重写或需要人工复核。

质量模式：

| 模式 | 行为 |
|---|---|
| `fast` | 跳过逐条 claim 验证，风险项以警告为主 |
| `enhanced` | 默认模式，逐条验证 claim 支撑度，unsupported 或 overstated 触发重写 |
| `strict` | 在 enhanced 基础上提高门控严格度 |

## 14. Checkpoint 与恢复

### `internal/checkpoint/`

职责：

- 保存运行断点。
- 记录输出产物引用与哈希。
- 校验 checkpoint 是否可恢复。
- 支持 WritingProgress 安全停止后的继续运行。

关键路径：

```text
checkpoints/latest.json
checkpoints/checkpoint-*.json
```

恢复校验关注：

- `latest.json` 是否存在。
- checkpoint 引用的文件是否存在。
- 文件 SHA256 是否匹配。
- 当前阶段是否允许恢复。

## 15. 导出系统

### `internal/export/`

职责：

- 汇总 accepted chapters 与 confirmed references。
- 导出 Markdown 论文、DOCX、参考文献、citation trace、运行报告和质量报告。
- 在质量产物缺失时进入兼容模式，并将问题写入 `final/report.md`。

最终产物：

```text
final/paper.md
final/paper.docx
final/references.md
final/citation-trace.json
final/quality-report.md
final/report.md
```

`final/citation-trace.json` 是扁平列表结构：

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

质量报告生成规则：

- 质量产物可用时生成 `final/quality-report.md`。
- 质量产物缺失但可兼容导出时，`final/report.md` 记录 compatibility warning。
- DOCX 写入失败不阻断 Markdown、references、citation trace 和 report。

## 16. 契约层

### `internal/contracts/`

职责：

- 定义跨模块共享的结构化数据类型。
- 统一 requirements、run event、review、claim、citation map、confirmed references 等 JSON 契约。
- 降低 TUI、Agent、Export、Quality 之间的直接耦合。

典型流向：

```text
TUI requirements form
  -> contracts.Requirements
  -> Store requirements.json
  -> AgentRuntime
  -> Architect/Writer/Editor
  -> ExportInput
```

## 17. 测试与夹具

### `internal/e2e/`

职责：

- 覆盖 TUI/Runtime/Quality/Review 等端到端链路。
- 验证小型材料集能完成关键流程。

### `fixtures/`

主要夹具：

```text
fixtures/materials/             # 通用材料解析夹具
fixtures/review-mini/materials/  # review mini flow 材料
fixtures/quality-mini/materials/ # quality mini flow 材料
```

### 常用测试命令

```bash
go build ./...
go test ./...
go test -v ./internal/e2e
```

## 18. 真实 LLM 工具

### `tools/real-tui-smoke/`

职责：

- 运行真实 TUI/Runtime/LLM 冒烟流程。
- 使用环境变量注入密钥，避免明文落盘。

相关环境变量：

```text
SMOKE_API_KEY
SMOKE_BASE_URL
SMOKE_MODEL
```

工具写入配置时使用：

```json
{
  "api_key": "env:SMOKE_API_KEY"
}
```

### `tools/real-before-after/`

职责：

- 对比 legacy/no-quality 与 enhanced/full-evidence-chain 路径。
- 用于真实 Provider 下验证质量引擎引入前后的行为差异。

## 19. 文档目录

### `docs/`

职责：

- 保存用户指南、需求、架构、接口契约、验收报告和历史设计记录。

关键文档：

```text
docs/user-guide.md
docs/需求与架构.md
docs/开发进度.md
docs/QualityEngine验收报告.md
docs/全量验收报告.md
docs/interfaces/_index.md
docs/interfaces/*.md
docs/superpowers/specs/*.md
```

### `docs/interfaces/`

职责：

- 描述模块间 JSON 契约和接口边界。
- 覆盖 common、config、materials、search、references、requirements、tui、checkpoint、agent、artifacts、quality、export 等模块。

### `docs/origindocs/`

职责：

- 保存 Bubble Tea、agentcore、litellm 等依赖或参考资料。

### `vault/`

职责：

- 保存分阶段开发记录和验收说明。
- 文件名按模块编号组织，例如 Store、材料解析、学术搜索、References TUI、Agent Runtime、导出、TUI 全流程等。

## 20. 端到端数据流

完整数据流可以概括为：

```text
ConfigWizard
  -> aipaper.json / config.Config
  -> AgentRuntime role config

Requirements
  -> requirements.json

MaterialsScan
  -> materials/manifest.json
  -> materials/extracted/
  -> materials/parsed/
  -> BibTeX candidates

SearchProgress
  -> online candidates
  -> references/candidates.json

References
  -> references/confirmed.json
  -> references/confirmed.bib
  -> references/rejected.json

WritingProgress
  -> outline/outline.json
  -> quality/evidence-table.*
  -> quality/section-quality-plan.*
  -> drafts/{chapter_id}/draft-v{N}.md
  -> drafts/{chapter_id}/claims-v{N}.json
  -> drafts/{chapter_id}/citation-map-v{N}.json
  -> reviews/{chapter_id}/review-v{N}.*
  -> quality/claim-graph.*
  -> quality/verification-result.json
  -> accepted/{chapter_id}.md
  -> checkpoints/*.json

ExportSummary
  -> final/paper.md
  -> final/paper.docx
  -> final/references.md
  -> final/citation-trace.json
  -> final/quality-report.md
  -> final/report.md

Done
  -> 用户人工复核 final/ 与 quality/ 产物
```

## 21. 模块依赖方向

推荐理解依赖方向如下：

```text
cmd/aipaper-cli
  -> internal/cli
  -> internal/tui/app
  -> internal/app

internal/tui/*
  -> internal/contracts
  -> internal/materials / search / references / app / export

internal/app
  -> internal/config
  -> internal/store
  -> internal/agent
  -> internal/checkpoint

internal/agent
  -> internal/contracts
  -> internal/artifacts
  -> internal/quality
  -> internal/store

internal/export
  -> internal/contracts
  -> internal/artifacts
  -> internal/quality
  -> internal/references
  -> internal/store

internal/quality
  -> internal/contracts
  -> internal/store
```

总体原则：

- `contracts` 承载跨模块数据结构。
- `store` 承载持久化基础设施。
- `tui` 负责交互状态，不直接承担底层业务持久化细节。
- `app` 负责把配置、Store、Agent、TUI 运行时接起来。
- `agent` 负责角色协议和工具编排。
- `quality` 负责证据链与质量门控。
- `export` 只消费已完成产物，不重新执行写作。

## 22. 快速定位表

| 想了解的问题 | 入口目录 |
|---|---|
| 程序如何启动 | `cmd/aipaper-cli/` |
| CLI 命令如何解析 | `internal/cli/` |
| TUI 页面如何流转 | `internal/tui/app/` |
| 首次配置如何生成 | `internal/tui/configwizard/` |
| Requirements 默认值在哪里 | `internal/tui/app/`、`internal/tui/requirements/` |
| LLM Provider 如何解析 | `internal/config/`、`internal/app/agent_runtime.go` |
| API Key 如何处理 | `internal/app/agent_runtime.go`、`internal/config/` |
| Coordinator 如何调度 | `internal/agent/`、`internal/app/` |
| Writer 产物写到哪里 | `internal/artifacts/` |
| claim/evidence 如何校验 | `internal/agent/`、`internal/quality/` |
| 材料扫描如何工作 | `internal/materials/` |
| 学术搜索 Provider 在哪里 | `internal/search/` |
| 引用确认如何落盘 | `internal/references/`、`internal/tui/references/` |
| checkpoint 如何恢复 | `internal/checkpoint/`、`internal/app/recover.go` |
| 最终导出路径在哪里定义 | `internal/export/` |
| 真实 LLM 冒烟怎么跑 | `tools/real-tui-smoke/` |

## 23. 维护注意事项

- 新增输出文件时，应同步更新 Store 布局、接口契约、导出报告和用户文档。
- 新增质量产物时，应同步更新 `internal/quality`、`internal/export` 的质量报告加载逻辑和 `docs/interfaces/quality.md`。
- 新增 Provider 时，应同步更新 `internal/search` 或 `internal/config` 相关 schema，并更新 ConfigWizard 模板或文档。
- 改动 claim、citation map、review schema 时，应同步检查 Writer Guard、Editor verifier、Citation Trace 和质量报告。
- 改动 TUI 主流程时，应同步检查 RootModel 状态迁移、恢复入口、README 和用户指南。
- 涉及 API Key 的改动应保持 `env:` 优先、输出脱敏、真实 smoke 不写明文密钥。
