# aipaper-cli 首版需求设计

- 状态：已批准进入文档落盘
- 日期：2026-06-05
- 相关文档：[need.md](../../../need.md)、[agentcore](../../../origindocs/agentcore.md)、[bubbletea](../../../origindocs/bubbletea.md)、[litellm](../../../origindocs/litellm.md)

本文件是 `aipaper-cli` 第一版（MVP）的需求设计草案，共 8 节，覆盖产品范围、系统架构、写作流程、数据契约、恢复设计、TUI 交互、文献管理与验收标准。

---

## 1. 产品范围与首版目标

### 产品定位

`aipaper-cli` 是一个面向 **学术综述 / 文献综述** 的 AI Agent CLI。第一版不追求成为完整论文管理平台，而是优先证明核心能力：

> 用户通过 TUI 填写结构化写作需求，系统结合用户材料与联网学术搜索，经过人工确认文献后，由多 Agent 自动规划、写作、评审、重写，并输出一个结构化论文项目目录。

### 首版核心闭环

MVP 必须稳定跑完这条链路：

1. 用户启动 CLI。
2. TUI 表单收集论文需求：
   - 主题 / 研究问题
   - 综述范围
   - 目标语言：中文或英文
   - 引用格式：中文默认 GB/T 7714，英文默认 APA
   - 目标字数 / 章节偏好
   - 材料目录路径
3. 系统解析用户材料：
   - 完整支持：PDF、Markdown、TXT、BibTeX
   - 基础支持：DOCX、网页链接、CSV
4. 系统执行学术搜索：
   - 免费公开 API：Semantic Scholar、Crossref、arXiv、PubMed 等
   - 可选增强源：SerpAPI、Tavily、Exa、Google Scholar 代理等
5. TUI 展示候选文献，用户多选确认。
6. Architect 基于需求与确认文献生成综述大纲。
7. Writer 按章节生成论文草稿。
8. Editor 做两类质量门控：
   - 引用一致性：关键论断必须能对应确认文献。
   - 质量评分：章节逻辑、综述完整度、可读性低于阈值时触发重写。
9. 系统输出结构化项目目录：Markdown 主稿、Docx 交付稿、文献库、章节草稿、评审报告、checkpoint / run 日志。

### 明确不做或弱化

第一版先不做：

- 强实时 Steer 干预。写作过程中用户不能立即中断当前 Agent 改方向。
- 完整论文管理平台。不会做多项目仪表盘、云同步、团队协作。
- 精细 Word 排版。Docx 是可交付初稿，不承诺复杂模板、自动目录、脚注等高级排版。
- 全格式深度解析。DOCX、网页链接、CSV 只做基础提取或降级支持。
- 全自动无确认引用。联网搜索结果必须经过用户确认后才能作为写作依据。

### 成功标准

第一版成功不只是“生成一篇文章”，而是：

- 整条流程稳定闭环；
- 引用来源经过用户确认；
- 每章草稿都有 Editor 审查记录；
- 低质量章节能自动重写；
- 崩溃后可从 Step checkpoint 恢复；
- 输出目录能让用户追溯“每段正文基于哪些文献、经过哪些评审”。

---

## 2. 系统架构与模块边界

首版架构采用 [need.md](../../../need.md) 里的核心原则：**LLM 驱动，Host 服务**。代码只负责可靠执行、持久化、恢复和展示，不在 Host 里写复杂调度规则。

### 总体结构

```text
┌─────────────────────────────────────────────────────────┐
│                         TUI                             │
│  需求表单 / 材料路径 / 文献确认 / 进度观察 / 结果入口       │
└──────────────────────────┬──────────────────────────────┘
                           │ events + commands
┌──────────────────────────▼──────────────────────────────┐
│                         Host                            │
│  启动 Agent / 恢复 checkpoint / 投影事件 / 调用工具注册     │
│  不决定写作顺序，不判断章节是否该重写                       │
└──────────────────────────┬──────────────────────────────┘
                           │ agentcore event stream
┌──────────────────────────▼──────────────────────────────┐
│                      Coordinator                        │
│  唯一流程决策者：规划 → 搜索/确认 → 写作 → 评审 → 导出       │
└──────────────┬────────────────┬────────────────┬────────┘
               │                │                │
     ┌─────────▼──────┐ ┌───────▼───────┐ ┌──────▼───────┐
     │   Architect    │ │    Writer     │ │    Editor    │
     │ 大纲/章节计划   │ │ 章节草稿/改写  │ │ 引用/质量评审 │
     └────────────────┘ └───────────────┘ └──────────────┘
               │                │                │
┌──────────────▼────────────────▼────────────────▼────────┐
│                         Tools                           │
│ 材料解析 / 学术搜索 / 文献库 / 工件读写 / checkpoint / 导出 │
└──────────────────────────┬──────────────────────────────┘
                           │ atomic writes
┌──────────────────────────▼──────────────────────────────┐
│                       Store / output                    │
│ progress.json / checkpoints / artifacts / paper.md/docx  │
└─────────────────────────────────────────────────────────┘
```

### 模块职责

#### 1. TUI

使用 Bubble Tea 实现。首版只承担交互入口，不承载复杂业务逻辑：

- 首次启动配置引导；
- 结构化论文需求表单；
- 材料目录选择或输入；
- 学术搜索结果多选确认；
- Agent 运行进度展示；
- 输出目录、报告、错误提示展示。

TUI 不直接生成论文，不直接决定哪些章节重写，只把用户输入转换成结构化事实写入 Store 或传给 Host。

#### 2. Host

Host 是薄外壳：

- 加载配置；
- 创建 litellm model；
- 创建 agentcore Agent；
- 注册 Coordinator 可用工具；
- 订阅 agentcore 事件并转发给 TUI；
- 根据 checkpoint 构造恢复 prompt；
- 管理中断、错误、退出。

Host 不写规则引擎、不写 scheduler、不维护任务队列。流程控制交给 Coordinator。

#### 3. Coordinator

Coordinator 是主 Agent，也是唯一流程决策者。它负责：

- 读取用户需求；
- 要求工具解析材料；
- 要求工具搜索候选文献；
- 等待用户确认文献；
- 调用 Architect 规划大纲；
- 调用 Writer 生成章节；
- 调用 Editor 评审；
- 根据 Editor 结果决定是否重写；
- 最后组织导出。

Coordinator 应通过明确的 system prompt 和工具返回 JSON 驱动，而不是依赖 Host 硬编码流程。

#### 4. SubAgents

使用 agentcore/subagent：

- **Architect**：设计综述结构、章节目标、每章应覆盖的文献群。
- **Writer**：根据章节合同、确认文献、已有上下文写章节草稿或重写。
- **Editor**：检查引用一致性、综述逻辑、重复、遗漏、表达质量，并给出评分和修改要求。

三个子 Agent 各自有隔离 context，通过 Store 中的 artifacts 协作，不直接共享长对话。

#### 5. Tools

工具只返回事实 JSON，不夹带“下一步你应该……”这类指令。首版工具分组：

- `requirements`：读写结构化需求。
- `materials`：解析 PDF/Markdown/TXT/BibTeX，基础解析 DOCX/URL/CSV。
- `search`：学术搜索、去重、候选文献标准化。
- `references`：候选文献、确认文献、引用键、来源追踪。
- `artifacts`：写入大纲、章节草稿、评审报告、最终稿。
- `checkpoint`：Step 级进度写入和恢复读取。
- `export`：Markdown 汇总与 Docx 导出。

### 技术栈约束

- Go 1.25 作为主语言；
- agentcore 作为 Agent loop、事件流、SubAgent、ContextManager 基础；
- litellm 作为多 provider LLM 适配；
- Bubble Tea + Bubbles + Lip Gloss 实现 TUI；
- 文件系统作为首版 Store，优先 temp + fsync + rename 原子写入。

---

## 3. 核心写作流程与 Agent 协作协议

首版流程要保持“可追踪、可恢复、可评审”。Coordinator 负责调度判断，Host 只提供工具和事件。

### 主流程

```text
启动/恢复
  ↓
结构化需求收集
  ↓
材料导入与索引
  ↓
学术搜索候选文献
  ↓
TUI 人工确认文献
  ↓
Architect 生成论文计划
  ↓
逐章节循环：
  Writer 写章节草稿
    ↓
  Editor 引用一致性 + 质量评分
    ↓
  是否达标？
    ├─ 否：Writer 根据评审意见重写，直到达标或达到上限
    └─ 是：提交章节
  ↓
整稿 Editor 总评
  ↓
Markdown 汇总 + Docx 导出
  ↓
完成报告
```

### Coordinator 的职责边界

Coordinator 不直接写整篇论文，而是维护“写作项目状态”：当前阶段、已确认文献、论文大纲、每章合同、每章草稿状态、Editor 评审结果、当前重写轮次、待导出产物。

Coordinator 每一步都通过工具读取事实、写入 artifact、写入 checkpoint。它可以决定“是否重写”“下一章写哪一节”“是否需要补充搜索”，但决定必须基于工具返回的 JSON 事实。

### Agent 协作产物

#### Architect 输出

Architect 生成 `outline.json` 和 `outline.md`，包含：论文标题建议、摘要目标、章节列表、每章写作目标、每章关键问题、每章应引用的文献候选、章节之间的逻辑关系、预计字数分配、需要 Writer 避免的重复点。

#### Writer 输出

Writer 每次只写一个章节或小节，输出：

- `draft.md`：章节正文；
- `claims.json`：关键论断列表；
- `citation_map.json`：每个关键论断对应的文献 key；
- `writer_notes.md`：无法确定、材料不足、建议补充之处。

Writer 不允许引用未确认文献。若材料不足，必须显式标记为 gap，而不是编造来源。

#### Editor 输出

Editor 输出 `review.json` 和 `review.md`，包含：总分、结构逻辑评分、引用一致性评分、综述完整度评分、表达可读性评分、重复/跑题/unsupported claim 列表、是否通过、必须修改项、可选优化项。

首版建议通过阈值：

- 单章总分 >= 80；
- 引用一致性 >= 90；
- 不存在高风险 unsupported claim；
- 重写最多 2 轮，超过后标记为 `needs_human_review`，但不中断整篇流程。

### 引用一致性规则

首版把引用一致性作为硬门槛：

- 正文中所有关键论断必须出现在 `claims.json`；
- 每个关键论断必须映射到至少一篇确认文献；
- Editor 必须检查文献是否真的支持该论断；
- 不确定支持关系时按不通过处理；
- 不允许使用“研究表明”“大量文献指出”等无来源泛化表达；
- 最终输出中保留 claim → reference 的追踪文件。

### 质量门控

质量门控不是 Host 规则引擎，而是 Coordinator 根据 Editor 事实结果决策。建议状态：

- `drafting`：Writer 正在写；
- `reviewing`：Editor 正在审；
- `revision_required`：Editor 不通过；
- `accepted`：章节通过；
- `needs_human_review`：超过重写上限仍未通过；
- `committed`：章节写入最终稿集合。

### 人工确认文献

学术搜索后，系统生成候选文献列表。TUI 展示：标题、作者、年份、来源、DOI / URL、摘要、与主题相关性理由、系统建议使用/排除原因、去重合并信息。

用户多选确认后，只有确认文献进入 `references/confirmed.json`。Coordinator 和所有子 Agent 只能基于 confirmed references 写作。

---

## 4. 数据结构、输出目录与文件契约

首版用文件系统作为 Store。每个论文项目绑定启动目录，产物统一落在 `output/` 下。所有 Agent 和工具通过结构化 artifact 协作，避免把关键状态只留在模型上下文里。

### 输出目录

建议首版结构：

```text
output/
  aipaper/
    run.json
    progress.json
    requirements.json

    checkpoints/
      latest.json
      step-000001.json
      step-000002.json

    materials/
      manifest.json
      extracted/
        material-001.md
        material-002.md
      parsed/
        material-001.json
        material-002.json

    references/
      candidates.json
      candidates.md
      confirmed.json
      confirmed.bib
      rejected.json

    outline/
      outline.json
      outline.md

    drafts/
      ch01/
        draft-v1.md
        claims-v1.json
        citation-map-v1.json
        review-v1.json
        draft-v2.md
        claims-v2.json
        citation-map-v2.json
        review-v2.json
        accepted.md
      ch02/
        ...

    reviews/
      chapter-summary.json
      final-review.json
      final-review.md

    final/
      paper.md
      paper.docx
      references.md
      citation-trace.json
      report.md
```

### 核心状态文件

#### `requirements.json`

保存 TUI 表单收集的写作需求：

```json
{
  "topic": "生成式 AI 在教育评估中的应用综述",
  "research_questions": ["..."],
  "scope": "2020-2026 年英文与中文核心研究",
  "language": "zh-CN",
  "citation_style": "gbt7714",
  "target_words": 8000,
  "material_dir": "./materials",
  "chapter_preferences": [],
  "constraints": []
}
```

#### `progress.json`

保存当前流程阶段，供 TUI 展示和恢复入口使用：

```json
{
  "phase": "chapter_review",
  "current_step": 42,
  "current_chapter": "ch03",
  "completed_chapters": ["ch01", "ch02"],
  "pending_chapters": ["ch03", "ch04"],
  "status": "running",
  "updated_at": "..."
}
```

#### `run.json`

保存本次运行元信息：

```json
{
  "run_id": "...",
  "created_at": "...",
  "resumed_from": null,
  "provider": "openrouter",
  "model": "google/gemini-2.5-pro",
  "cost_estimate": {},
  "events": []
}
```

### 文献文件契约

#### `references/candidates.json`

候选文献统一格式：

```json
{
  "items": [
    {
      "id": "cand_001",
      "title": "...",
      "authors": ["..."],
      "year": 2024,
      "source": "semantic_scholar",
      "doi": "...",
      "url": "...",
      "abstract": "...",
      "relevance_score": 0.86,
      "relevance_reason": "...",
      "dedupe_group": "doi:...",
      "status": "pending"
    }
  ]
}
```

#### `references/confirmed.json`

用户确认后的文献：

```json
{
  "items": [
    {
      "key": "zhang2024GenerativeAssessment",
      "title": "...",
      "authors": ["Zhang", "Li"],
      "year": 2024,
      "doi": "...",
      "url": "...",
      "abstract": "...",
      "source_material_ids": [],
      "confirmed_at": "..."
    }
  ]
}
```

### 章节文件契约

#### `claims-vN.json`

每章关键论断：

```json
{
  "chapter_id": "ch01",
  "draft_version": 1,
  "claims": [
    {
      "id": "ch01_claim_001",
      "text": "...",
      "importance": "high",
      "reference_keys": ["zhang2024GenerativeAssessment"],
      "confidence": 0.82
    }
  ]
}
```

#### `citation-map-vN.json`

正文段落与引用的映射：

```json
{
  "chapter_id": "ch01",
  "draft_version": 1,
  "mappings": [
    {
      "paragraph_id": "ch01_p003",
      "claim_ids": ["ch01_claim_001"],
      "reference_keys": ["zhang2024GenerativeAssessment"]
    }
  ]
}
```

#### `review-vN.json`

Editor 评审结果：

```json
{
  "chapter_id": "ch01",
  "draft_version": 1,
  "scores": {
    "overall": 84,
    "citation_consistency": 92,
    "structure_logic": 82,
    "coverage": 80,
    "readability": 85
  },
  "passed": true,
  "unsupported_claims": [],
  "required_fixes": [],
  "optional_fixes": []
}
```

### 文件写入规则

为支持 Step 级恢复，所有关键文件写入遵循：

- 工具执行成功后才写 checkpoint；
- artifact 写入使用 temp + fsync + rename；
- JSON 文件必须可被独立校验；
- checkpoint 记录本 step 的输入摘要、输出路径、状态变化；
- 不把长正文塞进 checkpoint，只记录 artifact 路径和哈希；
- TUI 只读 `progress.json` 和事件流，不直接解析所有正文文件。

---

## 5. Checkpoint、恢复与错误处理设计

第一版采用 **Step 级 checkpoint**，贴近 [need.md](../../../need.md) 的目标：每个工具成功完成后都记录可恢复状态。崩溃、断网、Ctrl+C 后，同一目录再次启动即可恢复。

### Step 级 checkpoint

每个可恢复动作都视为一个 Step，例如：`collect_requirements`、`parse_materials`、`search_references`、`confirm_references`、`create_outline`、`draft_chapter`、`review_chapter`、`revise_chapter`、`commit_chapter`、`final_review`、`export_docx`。

工具成功后写入：

```text
output/aipaper/checkpoints/step-000042.json
output/aipaper/checkpoints/latest.json
output/aipaper/progress.json
```

### Checkpoint 内容

checkpoint 不保存大正文，只保存事实状态、路径和哈希：

```json
{
  "step": 42,
  "phase": "review_chapter",
  "status": "success",
  "created_at": "...",
  "input": {
    "chapter_id": "ch03",
    "draft_version": 2
  },
  "outputs": [
    {
      "kind": "review",
      "path": "drafts/ch03/review-v2.json",
      "sha256": "..."
    }
  ],
  "state_patch": {
    "current_chapter": "ch03",
    "draft_version": 2,
    "review_passed": true
  },
  "next_expected": "commit_chapter"
}
```

### 恢复流程

同一目录再次运行时：

1. Host 检查 `output/aipaper/progress.json` 和 `checkpoints/latest.json`。
2. 校验 checkpoint 指向的 artifact 是否存在、哈希是否匹配。
3. 如果一致，生成恢复 prompt：当前已完成什么、当前阶段是什么、下一个预期 step 是什么、哪些 artifact 可读取、哪些操作不能重复。
4. Host 启动 Coordinator，Coordinator 从恢复 prompt 和 Store 继续。
5. TUI 展示“已从 step N 恢复”。

### 幂等工具设计

为了避免恢复后重复写坏状态，工具要尽量幂等：

- 同一 `chapter_id + draft_version` 不允许无提示覆盖；
- 如果目标 artifact 已存在且哈希匹配，工具返回 `already_exists`；
- 如果目标 artifact 已存在但内容不匹配，工具返回冲突错误；
- 搜索结果和候选文献按 DOI / URL / title hash 去重；
- commit chapter 只接受已通过 review 的版本；
- export 可以重复执行，但要记录导出版本。

### 错误分类

工具错误必须结构化返回，不能只给自然语言：

```json
{
  "ok": false,
  "error": {
    "code": "REFERENCE_SEARCH_TIMEOUT",
    "message": "Semantic Scholar request timed out",
    "retryable": true,
    "details": {
      "source": "semantic_scholar",
      "timeout_ms": 30000
    }
  }
}
```

首版错误类型：配置错误（缺少 API key、provider 不支持、模型不可用）、材料错误（文件不存在、格式不支持、解析失败）、搜索错误（API 超时、限流、返回字段缺失）、文献错误（候选文献为空、用户未确认任何文献）、写作错误（Agent 输出 JSON 不合法、章节合同缺失）、评审错误（claims/citation map 缺失、不支持的引用）、导出错误（Docx 转换失败）、存储错误（写入失败、哈希不匹配、checkpoint 冲突）。

### 重试策略

- 可重试外部错误：搜索超时、LLM 临时错误、导出临时失败，可重试 2-3 次；
- 不可重试错误：需求缺字段、确认文献为空、artifact 哈希冲突，需要用户处理；
- Editor 不通过不是系统错误，而是质量门控结果；
- 超过章节重写上限后标记 `needs_human_review`，继续处理后续章节，并在最终报告里突出显示。

### 崩溃一致性

每个 Step 写入顺序：

1. 写 artifact 临时文件；
2. fsync artifact；
3. rename 为正式文件；
4. 写 step checkpoint；
5. 更新 `latest.json`；
6. 更新 `progress.json`；
7. 发送事件给 TUI。

这样即使在任意点崩溃，也能通过 latest checkpoint 与 artifact 哈希判断可恢复位置。

---

## 6. TUI 交互与用户流程

第一版 TUI 以“写作引擎入口 + 可观测面板”为目标，不做复杂工作台。它需要让用户完成结构化需求输入、材料路径选择、文献确认，并在写作阶段看到系统在做什么。

### 启动流程

```text
aipaper-cli
  ↓
检查配置
  ├─ 无配置：进入 Provider 引导
  └─ 有配置：进入项目首页
  ↓
检查当前目录 output/aipaper/
  ├─ 有未完成 run：提示恢复 / 新建
  └─ 无 run：创建新论文项目
```

### 配置引导

首次运行引导用户填写：

- Provider：OpenRouter / Anthropic / Gemini / OpenAI / DeepSeek / Qwen / GLM / Grok / Ollama / Bedrock / Custom
- API Key：允许部分 provider 为空，如 Ollama、Bedrock、自定义无鉴权代理
- Base URL：可选或按 provider 默认
- Model：手动输入，也可从 provider 模型列表选择
- 默认语言和引用格式

配置写入 `~/.aipaper/config.json`，项目级可选覆盖写入 `./aipaper.json`。

### 项目首页

项目首页显示：当前目录、是否存在历史 run、当前 provider/model、新建论文、恢复论文、查看输出目录、退出。

### 结构化需求表单

用户选择新建后进入表单：

1. 论文主题；
2. 研究问题；
3. 综述范围；
4. 目标语言：中文 / English；
5. 引用格式：GB/T 7714 / APA；
6. 目标字数；
7. 材料目录；
8. 是否允许联网学术搜索；
9. 搜索增强源配置；
10. 章节偏好或特殊要求。

表单提交后写入 `requirements.json`。

### 材料解析进度

TUI 展示：已发现材料数量、当前解析文件、成功 / 失败数量、降级解析提示、可跳过文件列表。

DOCX、网页链接、CSV 如果只能基础解析，必须明确标注“降级提取”。

### 文献确认界面

学术搜索结束后进入多选列表。每条候选文献展示摘要信息：

```text
[ ] 2024  Zhang et al.
    Generative AI in Educational Assessment
    DOI: ...
    source: Semantic Scholar
    relevance: 0.86
```

用户可展开查看：abstract、相关性理由、DOI / URL、source、去重信息、系统建议、是否来自用户材料或联网搜索。

操作：

- Space：选择 / 取消；
- Enter：确认选择；
- `/`：搜索候选列表；
- `s`：按相关性 / 年份 / 来源排序；
- `a`：全选高相关候选；
- `r`：标记拒绝；
- `q`：返回或退出确认。

确认后写入 `references/confirmed.json`、`references/rejected.json`、`references/confirmed.bib`。

### 运行进度界面

进入写作后，TUI 展示：当前阶段、当前章节、当前 agent（Coordinator / Architect / Writer / Editor）、当前工具调用、已完成章节、重写轮次、最近错误、token / cost 估算、checkpoint step、输出文件路径。

首版不支持强实时 Steer。用户在写作中可以：查看日志、请求安全停止、等当前 Step 完成后退出、下次启动从 checkpoint 恢复。

### 完成界面

写作完成后展示：最终 Markdown 路径、Docx 路径、参考文献路径、质量报告路径、需要人工复核的章节、unsupported claim 统计、总耗时与成本估算。

### TUI 边界

TUI 不能直接修改正文 artifact。它只能：收集用户输入、写入确认选择、发送启动 / 停止 / 恢复命令、观察事件流、打开或提示输出路径。

---

## 7. 材料解析、学术搜索与引用管理

这一部分是文献综述质量的基础。首版目标不是覆盖所有学术数据库，而是建立可靠的材料/文献事实层，确保 Writer 和 Editor 只使用可追踪来源。

### 材料解析

材料目录由用户在 TUI 表单中指定。系统扫描后生成 `output/aipaper/materials/manifest.json`。每个材料都有稳定 ID：

```json
{
  "id": "material_001",
  "path": "./materials/paper-a.pdf",
  "kind": "pdf",
  "status": "parsed",
  "parser": "pdf_text",
  "degraded": false,
  "output_text": "materials/extracted/material_001.md",
  "output_meta": "materials/parsed/material_001.json"
}
```

### 格式支持分层

#### 完整支持

1. **PDF**：提取正文文本；尝试提取标题、作者、年份；提取 DOI / URL；长文本分块；记录页码或段落位置。
2. **Markdown / TXT**：作为用户笔记、论文要求、材料说明；保留标题层级；支持长文本分块；可进入 evidence pool。
3. **BibTeX**：解析 reference key；提取 title、author、year、doi、url、journal；直接进入候选文献池，默认标记为来自用户材料。

#### 基础 / 降级支持

1. **DOCX**：提取纯文本；保留基本标题；不承诺复杂表格、批注、脚注完整解析。
2. **网页链接**：抓取标题、正文摘要、URL；如果抓取失败，保留 URL 作为待用户确认来源；不把普通网页默认当成学术文献，除非能匹配 DOI/论文元数据。
3. **CSV**：读取表头和行；作为数据/笔记材料进入 evidence pool；不默认生成引用，除非列中包含 DOI/URL/title/author/year。

### 学术搜索

首版内置免费公开 API，并支持可选增强源。

- **内置源**：Semantic Scholar、Crossref、arXiv、PubMed。
- **可选增强源**（通过配置启用）：SerpAPI、Tavily、Exa、Google Scholar 代理、其他自定义搜索 provider。

### 搜索流程

1. Coordinator 根据需求生成搜索查询：主主题、研究问题、时间范围、中英文关键词、排除词。
2. Search Tool 分别调用各数据源。
3. 统一标准化字段：title、authors、year、DOI、URL、abstract、venue、source、citation count（如有）、relevance score。
4. 去重：DOI 优先；URL 次之；title + author + year hash 兜底。
5. 生成 `references/candidates.json` 与 `references/candidates.md`。
6. TUI 展示候选文献，等待用户确认。

### 引用 key 生成

确认文献生成稳定 key，格式 `firstAuthorYearShortTitle`，例如 `zhang2024GenerativeAssessment`。冲突时加后缀：`zhang2024GenerativeAssessmentA`、`zhang2024GenerativeAssessmentB`。

### 引用管理

确认文献写入 `references/confirmed.json` 与 `references/confirmed.bib`。Writer 和 Editor 只能读取 confirmed references。候选但未确认的文献不能用于正文引用。

### 来源追踪

最终必须输出 `final/citation-trace.json`，记录：章节、段落、claim、reference key、来源类型（用户材料 / 学术搜索 / BibTeX）、Editor 是否验证支持、是否需要人工复核。

### 质量规则

首版引用管理有几条硬规则：

- DOI 不存在时允许使用 URL，但必须标记来源可信度；
- 摘要缺失的文献可被确认，但 Editor 评审时可信度较低；
- 普通网页不能伪装成论文引用；
- 用户材料中的笔记可作为写作依据，但不能自动冒充学术文献；
- 引用格式化失败不应阻塞正文生成，但必须在报告中列出。

---

## 8. 测试、验收标准与首版边界

这一节把“做到什么算完成”说清楚，避免首版范围继续膨胀。

### 测试策略

首版测试分四层。

#### 1. 单元测试

覆盖纯逻辑模块：配置合并、requirements 校验、材料 manifest 生成、BibTeX 解析、文献去重、reference key 生成、checkpoint 写入顺序、checkpoint 校验与恢复、score threshold 判断、输出目录路径生成。

#### 2. 工具集成测试

使用 mock 或 fixture 覆盖：PDF / Markdown / TXT / BibTeX 解析；DOCX / URL / CSV 降级解析；Semantic Scholar / Crossref / arXiv / PubMed 搜索适配；搜索超时 / 限流 / 空结果；confirmed references 写入；chapter draft / review artifact 写入；Markdown 汇总；Docx 导出失败降级。

#### 3. Agent 流程测试

使用 mock LLM 或固定响应，验证 Coordinator 不依赖真实模型也能跑通状态机：新建项目完整流程；文献候选生成后等待用户确认；无确认文献时不进入写作；Architect 生成 outline 后进入章节循环；Editor 不通过触发 Writer 重写；超过重写上限标记 `needs_human_review`；最终生成 paper.md、report.md。

#### 4. 端到端冒烟测试

准备一个小型 fixture：

```text
fixtures/review-mini/
  materials/
    paper1.md
    paper2.bib
    notes.txt
```

跑完整流程，验收：能收集结构化需求；能生成候选文献；能 TUI 确认；能写 2 个章节；至少 1 次 Editor 评审；能输出 Markdown 和 Docx；中途杀进程后能恢复。

### 验收标准

MVP 完成需要满足：

1. **流程闭环**：从 TUI 表单到最终输出目录能完整跑通；没有人工修改中间 JSON 也能完成；文献确认是必须步骤。
2. **质量门控**：Editor 评审 JSON 结构稳定；章节低于阈值时触发重写；unsupported claim 会被记录；超过重写上限不会死循环。
3. **引用追踪**：Writer 不使用未确认文献；每章有 claims 和 citation map；最终有 citation trace；参考文献能导出为 Markdown 和 BibTeX。
4. **恢复能力**：每个关键 step 有 checkpoint；latest checkpoint 可校验 artifact hash；崩溃后能恢复到下一个预期 step；已完成 artifact 不被重复覆盖。
5. **TUI 可用性**：首次配置可完成；文献候选可多选确认；写作进度可观察；错误可读，不只显示 panic。
6. **输出可用**：`final/paper.md` 可读；`final/paper.docx` 可打开；`final/report.md` 能说明质量、风险和待人工复核项；结构化目录可追踪写作过程。

### 首版明确边界

第一版不承诺：生成可直接投稿的终稿；完美 GB/T 7714 或 APA 格式；Word 高级排版；Google Scholar 原生稳定抓取；OCR 扫描 PDF；表格、图片、公式深度理解；实时中断正在运行的 Agent；多用户、多项目管理后台；云同步或远程任务队列；自动判断文献真实性（所有联网文献仍需用户确认）。

### 推荐首版里程碑

后续实现可以拆成 5 个阶段：

1. **项目骨架 + 配置 + Store**
2. **材料解析 + 学术搜索 + 文献确认 TUI**
3. **Coordinator + SubAgents + artifact 协议**
4. **Editor 门控 + Step checkpoint 恢复**
5. **最终导出 + 报告 + E2E 验收**









