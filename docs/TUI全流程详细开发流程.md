# TUI 全流程详细开发流程

> 本流程将 `docs/superpowers/specs/2026-06-08-tui-full-generation-user-guide-design.md` 拆解为 10-21 号增量 vault 任务。已有 01-09 模块保持稳定，只在明确边界上桥接和扩展。

## 1. 总体路线

```text
10 RootModel/入口
  ├─ 11 ConfigWizard
  └─ 12 StateProbe/恢复入口
       └─ 13 Requirements 桥接
            └─ 14 MaterialsScan
                 └─ 15 SearchProgress/候选合并
                      └─ 16 References 桥接
                           └─ 17 WritingProgress/RuntimeEvent
                                ├─ 18 暂停退出恢复
                                └─ 19 ExportSummary/Done
                                     └─ 20 用户文档/README
                                          └─ 21 全流程测试/Windows 验收
```

## 2. 模块任务表

| 编号 | 模块 | 依赖 | 可并行 | 核心验收 |
|---|---|---|---|---|
| 10 | 无参数 TUI 启动与 RootModel 骨架 | 01 | 无 | 无参数进 TUI，有参数 CLI 不变 |
| 11 | ConfigWizard 配置向导 | 10 | 12 前半可并行 | 模板默认值正确，保存后 config/runtime 可解析，key 脱敏 |
| 12 | 启动状态探测与恢复入口 | 07,10,11 | 13 准备可并行 | Store 状态映射到正确首屏，valid checkpoint 可继续 |
| 13 | Requirements 屏幕桥接与落盘 | 10,12 | 无 | 复用既有 requirements model，写入 requirements.json |
| 14 | MaterialsScan 与候选材料入口 | 02,13 | 无 | 缺目录创建，扫描统计展示，BibTeX 候选保留 |
| 15 | SearchProgress 与候选合并 | 03,14 | 无 | 搜索候选 + 材料候选合并去重，provider 错误可处理 |
| 16 | References 屏幕桥接与确认落盘 | 04,15 | 无 | confirmed/rejected/confirmed.bib 写入，空确认阻塞写作 |
| 17 | WritingProgress 四区布局与 Runtime 事件桥 | 05,06,07,16 | 18 设计可并行 | mock runtime events 驱动日志、streaming、usage、章节进度 |
| 18 | 暂停退出与安全恢复 | 07,12,17 | 19 前半可并行 | Ctrl+C 安全停止，checkpoint 可恢复，重新开始二次确认 |
| 19 | ExportSummary 与完成页 | 08,17 | 18 部分可并行 | 调用 export 接口并展示实际输出和 docx 降级 |
| 20 | 用户文档与 README 快速入口 | 10-19 | 无 | user-guide 与实际命令/路径一致 |
| 21 | TUI 全流程测试与 Windows 双击验收 | 10-20 | 无 | `go test ./...`，mock 全流程，Windows 手动验收清单 |

## 3. 分阶段实施

### 阶段 A：入口和配置基础（10-12）

目标：无参数启动 TUI，首次运行能配置 provider，启动时能识别当前进度。

1. `10` 建立 `internal/tui/app`，实现 RootModel、Screen 枚举、transition msg、program 封装和 main.go 分流。
2. `11` 实现 ConfigWizard 模板、字段输入、摘要、保存、脱敏。
3. `12` 实现 StateProbe 和恢复提示，接入 `checkpoint.ValidateLatest` / `app.Recover`。

验收门槛：有参数 CLI 测试不回退；无参数可启动测试替身；ConfigWizard 保存后 `config.Load` 能读。

### 阶段 B：需求到文献确认（13-16）

目标：把现有 requirements/materials/search/references 能力串成 TUI 前半流程。

1. `13` 桥接 Requirements model，完成落盘。
2. `14` 后台调用 `materials.ProcessDir`，展示空目录、失败、降级、成功和详情。
3. `15` 调用 `search.Run`，合并材料 BibTeX 候选和搜索候选。
4. `16` 桥接 References model，调用 `ConfirmCandidates`。

验收门槛：从 requirements 到 confirmed references 的 mock 流程可跑通；BibTeX 候选不丢失；未确认文献不进入写作。

### 阶段 C：真实写作进度与恢复（17-18）

目标：四区布局消费 RuntimeEvent，支持 streaming 和 Ctrl+C 安全恢复。

1. `17` 定义 TUI 内部 RuntimeEvent，构建 event bridge 和四区布局 model。
2. `18` 增加安全停止、退出确认、恢复提示、重新开始二次确认。

验收门槛：mock runtime event 序列可驱动 UI；streaming delta 实时追加；Ctrl+C 不破坏 checkpoint。

### 阶段 D：导出、文档和全量验收（19-21）

目标：完成最终用户可用闭环和文档。

1. `19` 接入 ExportSummary / Done。
2. `20` 新增 `docs/user-guide.md`，更新 README 快速入口。
3. `21` 补齐全流程测试和 Windows 双击验收清单。

验收门槛：`go test ./...` 通过；最终输出文件说明与实现一致；Windows 启动路径已手动验证或记录限制。

## 4. 代码复用清单

- `internal/cli.Run`：保留带参数命令。
- `internal/config`：配置结构、加载、合并、校验；新增保存和脱敏公共函数时仍保持既有语义。
- `internal/store`：所有落盘走原子写入。
- `internal/checkpoint` / `internal/app/recover.go`：恢复入口。
- `internal/tui/requirements`：需求表单 model/view/update/validation。
- `internal/materials.ProcessDir`：材料扫描和解析。
- `internal/search.Run`：学术搜索。
- `internal/references`：候选去重、ID 分配、确认、BibTeX。
- `internal/tui/references`：文献确认 model/view/update。
- `internal/app/agent_runtime.go` / `internal/agent`：真实 LLM runtime 边界。
- `internal/artifacts`：写作产物和质量门控。
- `internal/export`：最终导出。

## 5. 禁止事项

- 不重写已稳定的 requirements / references 校验和确认规则。
- 不绕过 `references/confirmed.json` 直接写作。
- 不让 TUI 直接依赖 agentcore/litellm 的第三方事件细节。
- 不在 UI、日志、run events、report 中写完整 API key。
- 不在未二次确认时清空或覆盖已有 run 产物。
- 不把写作调度顺序硬编码到 TUI 或 Host。

## 6. 测试策略

| 层级 | 覆盖 |
|---|---|
| 单元测试 | ConfigWizard 模板、StateProbe、各屏幕 model、候选合并、RuntimeEvent 消费 |
| 集成测试 | requirements→materials→search→references、writing mock events、export summary |
| E2E | mock runtime 从首次配置到 final 导出 |
| 手动验收 | Windows 双击、窄窗口、Ctrl+C、真实 provider 配置 |

全量验证：每个 vault 完成后运行相关包测试；阶段 D 完成后运行 `gofmt`、`go test ./...`、`go build ./cmd/aipaper-cli`。如项目后续引入 `go vet`、`staticcheck` 或 `golangci-lint`，21 号验收需同步纳入。

## 7. 交付物

- 增量需求：`docs/TUI全流程增量需求.md`
- 增量架构：`docs/TUI全流程增量架构设计.md`
- 开发流程：`docs/TUI全流程详细开发流程.md`
- 接口补充：`docs/interfaces/tui.md`
- 增量任务：`vault/10-*.md` 到 `vault/21-*.md`
- 进度追踪：`docs/开发进度.md` 的 TUI 增量任务区
- 最终用户文档：实施阶段的 `docs/user-guide.md`
