# aipaper-cli 用户指南

## 1. 这是什么

`aipaper-cli` 是一个面向学术综述 / 文献综述写作的 AI Agent 命令行工具。它通过交互式 TUI 收集写作需求、解析用户材料、搜索学术文献、确认引用、调度 AI Agent 完成大纲 → 草稿 → 评审 → 重写 → 导出的全流程，最终生成可追溯的综述草稿。

核心能力：

- **交互式 TUI**：无参数启动即可进入全流程引导界面
- **多来源材料解析**：支持 PDF、Markdown、TXT、BibTeX，DOCX/URL/CSV 降级支持
- **学术搜索**：内置 Semantic Scholar、Crossref、arXiv、PubMed
- **文献人工确认**：候选文献必须经用户确认后才能被引用
- **AI 多角色写作**：Coordinator 调度 Architect、Writer、Editor 协作
- **质量门控**：章节评分、引用一致性检查、unsupported claim 拦截
- **断点恢复**：Ctrl+C 安全停止，下次启动自动恢复进度
- **结构化导出**：Markdown、Word、参考文献、引用追踪和质量报告

## 2. 安装与启动

### 环境要求

- Go 1.25.0 或更高版本
- 至少一个 LLM Provider 的 API Key（OpenAI、Anthropic 或 Ollama 本地部署）

### 构建

```bash
git clone https://github.com/thisxiaoyuQAQ/aipaper-cli.git
cd aipaper-cli
go build -o aipaper-cli ./cmd/aipaper-cli
```

Windows 用户：

```powershell
go build -o aipaper-cli.exe ./cmd/aipaper-cli
```

### 启动 TUI

无参数运行即进入交互式界面：

```bash
./aipaper-cli
```

Windows 用户可以直接双击 `aipaper-cli.exe`，或在终端运行：

```powershell
.\aipaper-cli.exe
```

### CLI 命令

带参数运行时使用传统命令行模式：

```text
aipaper-cli init     [--workdir DIR] [--config FILE]   初始化工作目录
aipaper-cli status   [--workdir DIR]                    查看当前状态
aipaper-cli recover  [--workdir DIR]                    校验 checkpoint 恢复
aipaper-cli config   [--workdir DIR] [--config FILE]    查看合并后配置
```

## 3. 首次配置 LLM

首次启动 TUI 时，如果没有找到有效配置，会自动进入 **ConfigWizard 配置向导**。

### 步骤 1：选择 Provider 模板

| 模板 | 默认模型 | API Key |
|---|---|---|
| OpenAI | `gpt-5.5` | 必填 |
| Anthropic | `claude-opus-4-8` | 必填 |
| Ollama | `llama3` | 无需 |
| Custom | 用户自定义 | 按需 |

### 步骤 2：填写必要字段

根据所选模板填写 API Key、Base URL 等。Ollama 用户只需确认本地服务地址（默认 `http://localhost:11434`）。

### 步骤 3：确认并保存

向导会展示配置摘要（API Key 自动脱敏显示，如 `sk-...abcd`），确认后保存为项目级配置文件 `./aipaper.json`。

### API Key 安全

- 推荐使用环境变量引用，如 `env:OPENAI_API_KEY`，避免密钥写入配置文件
- 若选择直接写入项目配置，向导会展示安全提示
- 所有日志、摘要和报告中 API Key 均自动脱敏

### 手动配置

也可以跳过向导，手动创建 `aipaper.json`：

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
    "coordinator": { "provider": "openai" },
    "architect":   { "provider": "openai" },
    "writer":      { "provider": "openai" },
    "editor":      { "provider": "openai" }
  }
}
```

配置加载优先级（后覆盖前）：

1. 全局配置：`~/.aipaper/config.json`
2. 项目配置：`./aipaper.json`
3. 命令行指定：`--config FILE`

## 4. 准备材料

在项目目录下创建 `materials/` 文件夹，放入你的参考材料：

```text
materials/
  survey-on-llm.pdf
  my-notes.md
  related-work.txt
  existing-refs.bib
```

支持的格式：

| 格式 | 支持程度 |
|---|---|
| PDF | 完整支持 |
| Markdown (.md) | 完整支持 |
| TXT | 完整支持 |
| BibTeX (.bib) | 完整支持（候选文献自动提取） |
| DOCX | 降级支持（基础文本提取） |
| URL 列表 | 降级支持 |
| CSV | 降级支持 |

TUI 进入 MaterialsScan 步骤后，会自动扫描 `materials/` 目录。如果目录不存在，TUI 会创建空目录并提示你放入文件。

BibTeX 文件中的文献会被自动提取为候选引用，与后续学术搜索结果合并。

## 5. 生成文章

TUI 全流程按以下顺序进行：

```text
ConfigWizard ─→ Requirements ─→ MaterialsScan ─→ SearchProgress
     │                                                  │
     │            ←─ RecoverPrompt（恢复时）              ▼
     │                                           References
     │                                                  │
     │                                                  ▼
     │                                          WritingProgress
     │                                                  │
     │                                                  ▼
     │                                          ExportSummary
     │                                                  │
     │                                                  ▼
     └──────────────────────────────────────────────  Done
```

### Requirements（填写写作需求）

填写主题、研究问题、综述范围、目标语言、引用格式、目标字数等。完成后需求保存到 `requirements.json`。

需求表单包含**质量模式**（`quality_mode`）三档选择，只改门控严格度，产物结构相同：

| 模式 | 行为 |
|---|---|
| `fast` | 跳过逐条论断验证（claim 标记 `skipped`），分级风险只给警告 |
| `enhanced`（默认） | 逐条论断验证；unsupported / overstated 论断触发章节重写 |
| `strict` | 在 enhanced 基础上，浅证据支撑强结论、部分支撑等也升级为需修订 |

无论哪档模式，**底线问题一律硬阻断**：引用未确认或伪造的文献 key、论断没有绑定证据、证据指向未确认引用。旧项目（无 `quality_mode` 字段）恢复时按兼容模式继续，不被阻断。

### MaterialsScan（扫描材料）

自动扫描 `materials/` 目录，解析全部文件。单个文件解析失败不影响其余文件。扫描结果展示已解析/失败/降级的文件统计。

### SearchProgress（学术搜索）

根据写作需求执行学术搜索，从 Semantic Scholar、Crossref、arXiv、PubMed 获取候选文献。搜索结果与材料中的 BibTeX 候选合并去重。

### References（确认文献）

候选文献列表展示在 TUI 中，支持搜索、排序、确认和拒绝操作。**至少确认一篇文献才能进入写作阶段**——这是硬性要求，未确认的文献不能被 AI 引用。

确认结果写入：
- `references/confirmed.json`（已确认文献）
- `references/confirmed.bib`（BibTeX 格式）
- `references/rejected.json`（被拒绝文献）

### WritingProgress（AI 写作）

四区实时进度界面：

- **左侧指标**：运行状态、阶段、完成比例、字数、模型、token 用量、费用、耗时
- **中上日志**：Coordinator / Architect / Writer / Editor 事件日志
- **中下内容**：Writer 流式输出的正文内容
- **右侧进度**：章节状态、评分、重写次数、引用一致性

写作流程：Architect 生成大纲 → Writer 逐章起草 → Editor 评审 → 不达标则重写（最多 2 轮）→ 全部通过后进入导出。

### ExportSummary（导出摘要）

展示导出结果：生成的文件列表、质量摘要（质量门控结论行，如"质量门控：1 章需要修订"）、需人工复核的章节。质量产物缺失的旧项目显示兼容模式提示。

### Done（完成页）

展示输出目录路径、文件清单和后续操作提示。

## 6. 输出文件说明

所有输出文件位于 `output/aipaper/` 目录：

### 最终产物（`final/`）

| 文件 | 说明 |
|---|---|
| `final/paper.md` | Markdown 格式的综述草稿 |
| `final/paper.docx` | Word 文档（基础排版，不承诺复杂格式） |
| `final/references.md` | 参考文献列表 |
| `final/citation-trace.json` | 引用追踪：章节 → 段落 → 论断 → 文献来源 |
| `final/report.md` | 质量报告：评分、需复核项、风险提示；含 Quality Summary（质量模式与门控结论） |
| `final/quality-report.md` | 质量引擎报告：门控结论、证据深度分布、论断支持度、unsupported/overstated 清单、重写汇总与下一步建议（仅当质量产物存在时生成；旧项目兼容模式不生成且不影响其他导出） |

### 中间产物

| 路径 | 说明 |
|---|---|
| `run.json` | 运行元信息、provider、成本估计 |
| `progress.json` | 当前阶段和章节状态 |
| `requirements.json` | 结构化写作需求 |
| `materials/manifest.json` | 材料扫描清单 |
| `materials/extracted/` | 提取的文本内容 |
| `materials/parsed/` | 解析后的结构化材料 |
| `references/candidates.json` | 候选文献 |
| `references/confirmed.json` | 已确认文献 |
| `references/confirmed.bib` | 已确认文献 BibTeX |
| `outline/outline.json` | 结构化大纲 |
| `drafts/` | 章节草稿、claims、citation map |
| `reviews/` | Editor 评审结果 |
| `checkpoints/` | 断点恢复数据 |

## 7. 中断与恢复

### 安全停止

在 WritingProgress 界面按 **Ctrl+C**，系统执行两阶段停止：

1. 发送停止请求，界面显示"正在停止..."
2. 等待当前 checkpoint 保存完成
3. 安全退出

不要强制关闭终端窗口——等待"停止完成"提示后再退出，确保进度不丢失。

### 恢复运行

下次启动 TUI 时，系统自动检测到未完成的 checkpoint，进入 **RecoverPrompt** 界面，提供三个选项：

- **继续写作**：从上次 checkpoint 恢复，进入 WritingProgress
- **重新开始**：二次确认后回到 Requirements 或 MaterialsScan（不删除已有文件）
- **退出**：结束 TUI

也可以通过 CLI 检查恢复状态：

```bash
./aipaper-cli recover --workdir .
```

## 8. 常见问题

### API Key 错误

**现象**：配置向导完成后，写作阶段报 authentication 错误。

**解决**：
1. 检查环境变量是否正确设置：`echo $OPENAI_API_KEY`（Linux/Mac）或 `echo %OPENAI_API_KEY%`（Windows CMD）
2. 确认 API Key 未过期且有足够额度
3. 使用 `aipaper-cli config` 命令查看当前生效的配置（Key 会脱敏显示）

### 没有材料（materials/ 为空）

**现象**：MaterialsScan 提示没有找到任何文件。

**解决**：将 PDF、Markdown、TXT 或 BibTeX 文件放入 `materials/` 目录后重新扫描。MaterialsScan 提供"重新扫描"选项。也可以选择跳过材料扫描，仅依赖学术搜索获取候选文献。

### 学术搜索失败

**现象**：SearchProgress 显示全部 provider 搜索失败。

**解决**：
1. 检查网络连接
2. 如果使用了增强搜索服务（SerpAPI、Tavily、Exa），确认对应 API Key 已设置
3. 可以选择"跳过搜索"，使用材料中的 BibTeX 候选继续；但如果候选文献不足，仍需在 References 步骤确认足够的引用
4. 也可以选择"重试"或"返回材料步骤"

### Context 过长

**现象**：写作阶段报 context length exceeded 错误。

**解决**：
1. 减少材料文件数量或体积，降低输入 token 量
2. 在 `aipaper.json` 中为 Writer 或 Editor 角色配置 context 窗口更大的模型
3. 减少写作需求中的目标字数

### Docx 导出失败

**现象**：ExportSummary 显示 Word 文档生成失败。

**解决**：这不影响其他导出文件。`final/paper.md`、`final/references.md`、`final/citation-trace.json` 和 `final/report.md` 仍然可用。Word 文档使用基础 OOXML 格式生成，不支持复杂排版。如果需要 Word 格式，可以使用 Pandoc 等工具从 `paper.md` 手动转换。

### Windows 双击后窗口闪退

**现象**：双击 `aipaper-cli.exe` 后窗口一闪而过。

**解决**：
1. 右键 `aipaper-cli.exe`，选择"在终端中打开"或"用 PowerShell 打开"
2. 或者打开 CMD / PowerShell，`cd` 到 `aipaper-cli.exe` 所在目录后运行 `.\aipaper-cli.exe`
3. 如果有错误信息，终端会显示具体原因（如缺少配置、Go 版本不兼容等）

## 9. 高级命令

### init —— 初始化工作目录

```bash
aipaper-cli init --workdir .
aipaper-cli init --workdir . --config ./aipaper.json
```

创建 `output/aipaper/` Store 布局，包括 `run.json`、`progress.json` 和所有子目录。如果 Store 已存在，复用现有结构。

### status —— 查看状态

```bash
aipaper-cli status --workdir .
```

输出当前运行状态：阶段、step、章节进度和更新时间。未初始化时输出 `status: not initialized`。

### recover —— 校验恢复状态

```bash
aipaper-cli recover --workdir .
```

检查 `checkpoints/latest.json` 指向的产物是否完整、哈希是否匹配。返回非零退出码表示不可恢复。

### config —— 查看配置

```bash
aipaper-cli config --workdir . --config ./aipaper.json
```

输出加载的配置文件列表和合并后的配置对象。API Key 自动脱敏为 `"redacted"`。

## 附录：环境变量参考

项目提供 `.env.example` 列出了全部可用环境变量：

| 变量名 | 说明 |
|---|---|
| `OPENAI_API_KEY` | OpenAI API 密钥 |
| `ANTHROPIC_API_KEY` | Anthropic API 密钥 |
| `OPENROUTER_API_KEY` | OpenRouter API 密钥 |
| `GEMINI_API_KEY` | Google Gemini API 密钥 |
| `DEEPSEEK_API_KEY` | DeepSeek API 密钥 |
| `QWEN_API_KEY` | Qwen API 密钥 |
| `GLM_API_KEY` | GLM (智谱) API 密钥 |
| `GROK_API_KEY` | Grok API 密钥 |
| `OLLAMA_BASE_URL` | Ollama 本地服务地址（默认 `http://localhost:11434`） |
| `ANTHROPIC_BASE_URL` | Anthropic API 地址（默认 `https://api.anthropic.com`） |
| `CUSTOM_LLM_PROVIDER` | 自定义 provider 名称 |
| `CUSTOM_LLM_TYPE` | 自定义 provider 类型 |
| `CUSTOM_LLM_BASE_URL` | 自定义 LLM 服务地址 |
| `CUSTOM_LLM_MODEL` | 自定义 LLM 模型名称 |
| `CUSTOM_LLM_API_KEY` | 自定义 LLM API 密钥 |
| `SERPAPI_KEY` | SerpAPI 搜索增强密钥 |
| `TAVILY_KEY` | Tavily 搜索增强密钥 |
| `EXA_KEY` | Exa 搜索增强密钥 |
| `GOOGLE_SCHOLAR_PROXY_URL` | Google Scholar 代理地址 |
| `AIPAPER_DEFAULT_LANGUAGE` | 默认写作语言（如 `zh-CN`） |
| `AIPAPER_CITATION_STYLE` | 默认引用格式（如 `gbt7714`） |
| `AIPAPER_OUTPUT_DIR` | 输出目录（默认 `output/aipaper`） |
