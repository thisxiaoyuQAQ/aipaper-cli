# aipaper-cli

`aipaper-cli` 是一个面向学术综述 / 文献综述写作的 AI Agent CLI。它围绕「需求收集 → 材料解析 → 学术搜索 → 人工确认文献 → Agent 写作与评审 → checkpoint 恢复 → 最终导出」构建本地工作流，把已有材料和联网检索结果整理成可追溯、可复核的综述草稿。

## 功能特性

- **交互式 TUI**：无参数运行 / Windows 双击即可进入全流程引导界面
- **本地项目 Store**：所有运行状态、材料、文献、草稿和导出文件都落盘到 `output/aipaper/`
- **多来源材料解析**：PDF、Markdown、TXT、BibTeX；DOCX、URL、CSV 降级支持
- **学术搜索**：内置 Semantic Scholar、Crossref、arXiv、PubMed
- **人工确认文献**：候选文献必须经用户确认后才能进入最终引用
- **Agent 写作流程**：Coordinator 调度 Architect、Writer、Editor，含质量门控与重写
- **断点恢复**：Ctrl+C 安全停止，下次启动自动恢复进度
- **结构化导出**：`paper.md`、`paper.docx`、`references.md`、`citation-trace.json`、`report.md`

## 快速开始

### 1. 构建

```bash
git clone https://github.com/thisxiaoyuQAQ/aipaper-cli.git
cd aipaper-cli
go build -o aipaper-cli ./cmd/aipaper-cli
```

Windows：

```powershell
go build -o aipaper-cli.exe ./cmd/aipaper-cli
```

### 2. 启动 TUI

无参数运行进入交互式界面：

```bash
./aipaper-cli
```

Windows 用户可直接双击 `aipaper-cli.exe`，或在终端运行 `.\aipaper-cli.exe`。

首次启动会进入配置向导（OpenAI / Anthropic / Ollama / Custom 模板），随后引导你完成需求填写、材料扫描、文献确认、AI 写作和导出。

### 3. CLI 命令（带参数模式）

```text
aipaper-cli init     [--workdir DIR] [--config FILE]   初始化工作目录
aipaper-cli status   [--workdir DIR]                    查看当前状态
aipaper-cli recover  [--workdir DIR]                    校验 checkpoint 恢复
aipaper-cli config   [--workdir DIR] [--config FILE]    查看合并后配置
```

## 流程概览

```text
ConfigWizard → Requirements → MaterialsScan → SearchProgress
                                                     │
                  RecoverPrompt（恢复时） ──────────  ▼
                                              References
                                                     │
                                                     ▼
                                            WritingProgress
                                                     │
                                                     ▼
                                       ExportSummary → Done
```

## 文档

完整的安装、配置、材料准备、生成、导出、恢复和常见问题说明，请查阅：

**[用户指南 docs/user-guide.md](docs/user-guide.md)**

其他文档：

- 需求与架构：[docs/需求与架构.md](docs/需求与架构.md)
- 接口契约索引：[docs/interfaces/_index.md](docs/interfaces/_index.md)
- 开发进度：[docs/开发进度.md](docs/开发进度.md)

## 开发与测试

```bash
go build ./...     # 编译检查
go test ./...      # 运行全部测试
```

主要代码结构：

```text
cmd/aipaper-cli/          CLI / TUI 入口
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
internal/tui/             TUI 屏幕：配置向导、需求、材料、搜索、文献、写作、导出
internal/artifacts/       写作产物与质量门控
internal/export/          Markdown、Docx、引用追踪和报告导出
fixtures/review-mini/     E2E 冒烟测试夹具
```

## 项目边界

当前版本不承诺：

- 生成可直接投稿的最终论文
- 完美处理 GB/T 7714、APA 或 Word 高级排版
- OCR 扫描 PDF、深度理解图片 / 表格 / 公式
- Web UI、多用户后台、云同步或远程任务队列
- 对联网文献真实性自动背书；文献仍需用户确认和复核

## 许可证

暂未在仓库中发现许可证文件。发布或分发前建议补充 `LICENSE`。
