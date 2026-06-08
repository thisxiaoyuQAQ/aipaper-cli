# TUI 全流程增量架构设计

> 本文是 `docs/需求与架构.md` 的增量架构补充，覆盖无参数 TUI、配置向导、状态探测、材料/搜索/文献确认桥接、写作进度四区布局、导出摘要和用户文档。

## 1. 系统架构图

```text
cmd/aipaper-cli/main.go
  ├─ len(args)>0 ──> internal/cli.Run              # 保留 init/status/recover/config
  └─ len(args)==0 ─> internal/tui/app.RunProgram   # 新增 TUI 入口

internal/tui/app RootModel
  ├─ StateProbe ── config/store/checkpoint/artifacts 状态探测
  ├─ ConfigWizard ──> internal/config 保存 ./aipaper.json
  ├─ RequirementsBridge ──> internal/tui/requirements + store.WriteJSON
  ├─ MaterialsScan ──> internal/materials.ProcessDir
  ├─ SearchProgress ──> internal/search.Run + internal/references 去重合并
  ├─ ReferencesBridge ──> internal/tui/references + references.ConfirmCandidates
  ├─ WritingProgress ──> internal/app.AgentRuntime + RuntimeEventBridge
  ├─ ExportSummary ──> internal/export.LoadInput / ExportFinal
  └─ Done
```

核心原则：**TUI 编排屏幕，Host 执行运行时，Coordinator 决策写作流程，Store 是状态权威。**

## 2. 屏幕状态机

```text
ConfigWizard
  -> Requirements
  -> MaterialsScan
  -> SearchProgress
  -> References
  -> WritingProgress
  -> ExportSummary
  -> Done

RecoverPrompt
  -> 继续写作: WritingProgress（携带 recovery prompt）
  -> 重新开始: 二次确认后回到 Requirements 或 MaterialsScan
  -> 退出: 结束 TUI
```

启动时 RootModel 根据 StateProbe 进入合适屏幕：

1. 配置缺失、provider 为空、role 映射缺失或 role 指向不存在 provider：`ConfigWizard`。
2. 存在 valid 未完成 checkpoint：`RecoverPrompt`。
3. 缺 `requirements.json`：`Requirements`。
4. 缺材料 manifest 或用户请求重扫：`MaterialsScan`。
5. 缺 confirmed references：若缺 candidates 先 `SearchProgress`，否则 `References`。
6. 缺完整写作产物：`WritingProgress`。
7. 缺 final artifacts：`ExportSummary`。
8. 全部完成：`Done`。

屏幕切换统一使用：

```go
type ScreenTransitionMsg struct {
    Next Screen
    Data any
}
```

子屏幕只发 transition，不直接修改 RootModel 全局流程。

## 3. 领域模型

```text
Config
  ├─ providers[default] ProviderConfig
  └─ roles[coordinator|architect|writer|editor] RoleConfig

Requirements
  └─ material_dir -> MaterialsScan

MaterialsScanResult
  ├─ manifest.items[]
  └─ bibtexCandidates[] -> CandidateMerge

SearchResult
  └─ candidates[] -> CandidateMerge

ReferenceCandidates
  └─ ReferencesBridge -> ConfirmedReferences

AgentRuntime
  ├─ RuntimeEventBridge -> WritingProgressState
  ├─ artifacts/drafts/reviews
  └─ checkpoint/progress

ExportInput
  └─ final outputs -> ExportSummary/Done
```

## 4. 技术决策记录

| 决策项 | 选择 | 备选方案 | 选择理由 |
|---|---|---|---|
| TUI 入口 | `main.go` 无参数分流到 TUI | 把 TUI 塞进 `cli.Run` | 不破坏现有 CLI 命令测试与 stdout/stderr 语义 |
| TUI 架构 | 单一 Bubble Tea Program + RootModel 状态机 | 多 Program 链式启动 | 保持连续用户体验和统一状态管理 |
| 配置保存 | TUI 默认写项目 `aipaper.json` | 写全局 `~/.aipaper/config.json` | 避免首次运行污染全局配置 |
| ConfigWizard 格式 | 复用 `config.Config` | 新增 provider JSON | 避免双配置体系和 runtime 漂移 |
| Requirements | 桥接既有 model | 重写表单 | 保留已验收校验规则 |
| References | 桥接既有 model | 重写确认 UI | 保留 key 冲突、confirmed/rejected/BibTeX 规则 |
| 材料扫描 | 调用 `materials.ProcessDir` | TUI 内重写解析 | 复用已验收解析和降级逻辑 |
| 搜索候选合并 | TUI 编排层合并材料候选与搜索候选 | 修改 `search.Run` 强行支持材料 | 避免破坏搜索模块既有落盘语义 |
| Runtime 事件 | 内部 `RuntimeEvent` 桥接到 TUI | TUI 直接依赖 agentcore/litellm event | 隔离第三方事件细节 |
| 写作顺序 | Coordinator 决策 | TUI/Host 硬编码 Architect→Writer→Editor | 维持“LLM 驱动，Host 服务”原则 |
| 导出 | 复用 `export.LoadInput` / `ExportFinal` | TUI 自行拼接文件 | 避免导出契约漂移 |

## 5. 全局约定补充

- Screen 名称使用稳定枚举：`config_wizard`、`recover_prompt`、`requirements`、`materials_scan`、`search_progress`、`references`、`writing_progress`、`export_summary`、`done`。
- TUI 模式项目根默认使用进程当前工作目录；Windows 双击场景若工作目录不可控，启动页和用户文档必须明确当前项目根，并提示用户可从目标项目目录的终端运行 exe。
- TUI 事件不得包含完整 API key。
- role 映射建议覆盖 `coordinator`、`architect`、`writer`、`editor`；若实现只要求后三者，也必须保证 coordinator 通过顶层 provider/model 可解析。
- 确认引用仍是写作硬门槛；`references/confirmed.json` 为空不得进入 WritingProgress。
- 缺 usage 时 token/cost 显示 `--`。
- final 输出以 `internal/export` 实际 Result 为准；BibTeX 路径必须明确为 `references/confirmed.bib`，除非实现新增 `final/references.bib`。

## 6. 目录结构设计

新增或调整：

```text
internal/tui/
  app/                    RootModel、状态探测、program 启动封装
  configwizard/           provider 模板配置向导
  materials/              MaterialsScan 屏幕 model/view
  search/                 SearchProgress 屏幕 model/view
  writing/                四区布局与 RuntimeEvent 消费
  exportsummary/          ExportSummary 屏幕
  done/                   完成页
  requirements/           已有，桥接不重写
  references/             已有，桥接不重写

docs/
  TUI全流程增量需求.md
  TUI全流程增量架构设计.md
  TUI全流程详细开发流程.md
  user-guide.md           后续实现阶段新增
  interfaces/tui.md

vault/
  10-无参数TUI启动与RootModel骨架.md
  ...
  21-TUI全流程测试与Windows双击验收.md
```

## 7. 部署架构与环境变量

- 开发命令：`go test ./...`、`go build ./cmd/aipaper-cli`。
- Windows 构建：`go build -o aipaper-cli.exe ./cmd/aipaper-cli`。
- 无参数运行：`./aipaper-cli` 或双击 `aipaper-cli.exe`。
- 高级命令：`init/status/recover/config` 保持带参数模式。
- 环境变量模板沿用 `.env.example`，后续文档需解释 `OPENAI_API_KEY`、`ANTHROPIC_API_KEY`、`OLLAMA_BASE_URL`、`CUSTOM_LLM_*`。

## 8. 测试架构

- RootModel 和 StateProbe 使用纯 Go 单元测试，构造临时 Store 状态。
- ConfigWizard 使用 model 测试覆盖模板默认值、保存配置、脱敏。
- MaterialsScan/SearchProgress 使用 fake dir / fake provider 测试成功、失败、降级、合并。
- WritingProgress 使用 mock RuntimeEvent 序列测试布局状态，不依赖真实 LLM。
- ExportSummary 使用现有 export fixture 和 fake docx exporter。
- 最终 E2E 用 mock runtime 跑配置到导出全流程；真实 LLM 路径保留手动验收。

## 9. 主要风险与缓解

1. **无参数语义改变**：只在 `main.go` 分流，保留 `cli.Run([]string{})` 现有语义。
2. **材料 BibTeX 候选被 search 覆盖**：在 SearchProgress 后显式合并并重写 candidates。
3. **Requirements 要求材料目录存在**：TUI 启动或提交前确保默认 `materials/` 存在，或在 MaterialsScan 创建并引导。
4. **Streaming 事件契约不稳定**：先定义内部 `RuntimeEvent`，TUI 不直接依赖第三方事件。
5. **Writer runtime 未完全实现**：在 WritingProgress vault 中明确真实 Writer/Architect/Editor runner 接入边界和 mock 验收。
6. **Export 文档路径漂移**：ExportSummary 和 user guide 读取 `ExportFinal` Result 或同步新增 final BibTeX。
7. **Windows 双击工作目录不确定**：文档和验收清单必须覆盖工作目录、配置保存位置和错误兜底。
