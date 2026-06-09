# 模块19 审查报告

## 审查概要
- 审查时间：2026-06-09
- 审查对象：vault/19-ExportSummary与完成页.md（编写 Agent 产出）
- 涉及文件：
  - `internal/tui/exportsummary/model.go`（186 行）
  - `internal/tui/exportsummary/view.go`（150 行）
  - `internal/tui/done/model.go`（124 行）
  - `internal/tui/app/root.go`（修改：转场桥接）
- 编译验证：`go build ./...` 通过（无输出）；`go vet ./internal/tui/exportsummary/... ./internal/tui/done/...` 通过
- 勿动文件验证：`git status internal/export/ internal/tui/writing/` → working tree clean，未修改

## 安全性
- ✅ 输入参数校验：`NewModel` 对空 `WorkDir` 兜底为 `"."`；`done.NewModel` 同样兜底。
- ✅ 路径遍历风险：输出路径全部经 `store.Store.Path()` 派生（`filepath.Join(root, parts...)`），root 由 `WorkDir + DefaultOutputDir + DefaultProject` 固定拼接，本模块未接受任意用户路径输入，无遍历风险。
- ✅ 敏感数据脱敏：View 仅渲染输出目录、文件列表、Issue.Code/Message。无 API key/密钥/token 输出（grep 确认无硬编码 secret）。
- ✅ 错误信息友好：`renderError` 对 `export.Error` 展示 Code+Message，并按 Code 附加中文用户提示（`errorHint`）；非 export.Error 才回退到 `err.Error()`，不暴露堆栈或内部路径细节。

## 并发与数据一致性
- ✅ exportDoneMsg 处理安全：导出在 `runExport` 返回的 `tea.Cmd` 闭包中执行，结果通过 `exportDoneMsg` 单向回传到 `Update`，符合 Bubble Tea 单 goroutine 消息循环模型，无竞态。
- ✅ Model 为值类型（非指针），`Update`/`handleKey` 返回新副本，无共享可变状态，无需锁。
- ✅ 闭包捕获：`defaultExport` 捕获 `docx`/`now`（值），`runExport` 捕获 `m.export`/`m.store`（值），无逃逸的可变引用。

## 资源管理
- ✅ Store 路径为值对象，Go GC 自动回收，无需手动释放。
- ✅ 无文件句柄、无网络连接、无 goroutine 泄漏（导出 cmd 执行一次即返回 msg）。
- ✅ View 用 `strings.Builder`，无大对象常驻；`result` 仅保存导出元数据（Outputs/Issues 列表），规模可控。

## 性能
- ✅ 文件读取：导出仅在 Init / 重试 / 同步 Run 时触发一次，`export.LoadInput` 内部已优化（按 accepted.md 哈希匹配版本）。
- ✅ View 渲染：全部使用 `strings.Builder`，无 `+` 字符串拼接累积；渲染按需分支（未导出/出错/成功）提前 return。
- ✅ 无 N+1 文件读取或循环内 I/O。

## 错误处理
- ✅ 外部调用错误捕获：`defaultExport` 对 `export.LoadInput` 错误立即返回；`ExportFinal` 错误经 `exportDoneMsg.err` 传递。
- ✅ 错误类型：本模块用标准 error 传递，View 层用类型断言 `err.(export.Error)` 区分结构化错误码，符合 export 包约定。
- ✅ 无静默吞异常：所有 error 路径均回传到 `m.err` 并在 View 展示；root.go `updateExportSummary` / default 分支同步 `m.err = m.ExportSummary.Err()`。
- ✅ 用户友好提示：`CodeNoAcceptedChapters` / `CodeUnconfirmedReference` 有明确中文 hint 且标注「未生成错误文件」；`CodeDocxFailed` 通过 `renderDocx` 警告降级，明确告知 Markdown/report 已成功。

## 项目约定一致性
- ✅ Store 根目录：经 `store.New(workDir)` → `output/aipaper/`，符合核心约定。
- ✅ 路径相对化：Outputs.Path 来自 `ExportFinal`（相对 Store 根正斜杠），输出目录展示用 `Store.Path("final")` 绝对路径，二者职责分离且一致。
- ✅ 时间 UTC：`Now` 经 Options 注入，默认零值时 `ExportFinal` 内部 `time.Now().UTC()` 并强制 `.UTC()`。
- ✅ 复用工具：严格调用 `export.LoadInput` + `export.ExportFinal`，未自行拼接 final 内容（见模块19特定审查）。
- ✅ 第三方依赖：仅 import bubbletea / lipgloss / 项目内 export、store，无新增未批准依赖。
- ✅ 命名清晰：`ExportSummary`、`Done`、`exportDoneMsg`、`ExportFunc`、`defaultExport` 一致明确。
- ✅ 魔法值：UI 提示字面量（允许），路径常量复用 export 包定义。

## 代码质量
- ✅ 单文件行数：model.go 186、view.go 150、done/model.go 124，均 < 500（交接单声明 178/140/98 与实际略有差异，但均远小于上限，不影响结论）。
- ✅ 接口/类型一致性：`export.Result{Version, Outputs, Issues, DocxWritten}`、`export.Issue{Code, Message, ChapterID, ClaimID, ReferenceKey}`、`export.Error{Code, Message}` 均与 `internal/export/types.go` 及 `docs/interfaces/export.md` 第 2/6 节一致。
- ✅ 边界条件：空 Outputs（renderOutputs 显示「(无)」）、零 Issues（renderIssues 返回空）、导出失败（renderError）、docx 降级（renderDocx）均覆盖。

## 模块19 特定审查（重点）
- ✅ **导出复用**：`defaultExport` 仅 `LoadInput` → `ExportFinal`，无任何 Markdown/references 拼接逻辑，完全委托 export 包。
- ✅ **错误处理**：
  - `CodeNoAcceptedChapters`：`ExportFinal`/`loadAcceptedChapters` 返回 `Error`，不写产物；View 提示「未生成错误文件」。✅
  - `CodeUnconfirmedReference`：`ExportFinal` 在写盘前 return Error，不写产物。✅
  - `CodeDocxFailed`：export 包记录 Issue 并清理 stale docx，`DocxWritten=false`，paper.md/report.md 正常生成；View `renderDocx` 明确降级提示。✅
- ✅ **依赖注入**：`Options{Export, DocxExporter, Now}` 三点可注入，便于 mock 成功/失败/降级/确定性版本；`done.Options{WorkDir, Result}` 注入结果避免重算。设计良好。
- ✅ **转场逻辑**：
  - `updateWriting`：`Writing.Done()` → `ScreenExportSummary`，构造 model 并 `Init()` 触发导出，不再直接 tea.Quit。✅
  - `updateExportSummary`：`Canceled()` → tea.Quit；`Done()` → `ScreenDone`，传入 `Result()`。✅
  - `updateDone`：`Quit()` → tea.Quit。✅
  - 完整路径 Writing → ExportSummary → Done → Quit 正确。
- ✅ **异步消息**：`exportDoneMsg` 通过 root.go `Update` 的 `default` 分支（line 384-392）转发到 `ExportSummary.Update`，并将更新后的 model 与 cmd 回传、同步 err。`Update` 内 case 正确处理 `exportDoneMsg`（置 exported/result/err）。✅
- ✅ **View 展示**：输出目录（绝对路径）、生成文件列表、docx 降级提示、Issues 列表（含 ChapterID/ReferenceKey 上下文）、错误提示与操作热键（[r]/[b]/[esc]/[enter]）均完整清晰。

## 勿动文件清单
- ✅ `internal/export/` 全部文件：`git status` 确认未修改（docx.go / export.go / export_test.go / load.go / types.go 均 clean）。
- ✅ `internal/tui/writing/`：未修改核心逻辑，仅在 root.go 边界通过 `Writing.Done()/Canceled()/Err()` 接入。

## 发现的问题

| 严重程度 | 文件 | 行号 | 描述 | 建议修复方式 |
|---|---|---|---|---|
| 🔵 建议 | model.go | 140-155 | `ApplyExportResult` 与 `Run` 为同步导出便利方法，但实际转场链（root.go）走的是异步 `Init()` → `exportDoneMsg` 路径，二者当前未被生产代码调用，仅供测试。若测试 Agent 不使用可考虑保留（便于单测）或注明用途。 | 保留供测试 mock；建议测试 Agent 至少覆盖其中之一，否则属未用代码。 |
| 🔵 建议 | view.go | 79 | `errorHint` 仅覆盖 NoAcceptedChapters / UnconfirmedReference。`CodeDocxFailed` 不会进入 renderError（它是 Issue 非致命 Error），逻辑正确，但 `CodeReferenceFormat` 同理也走 Issue，无需 hint，确认无遗漏。 | 无需修改，仅记录确认。 |
| 🔵 建议 | model.go | 115-118 | `UpdateKey` 用 `updated.(Model)` 类型断言 handleKey 返回的 tea.Model，因 handleKey 始终返回 Model 故安全，但断言失败会 panic。当前无风险（内部受控）。 | 可接受；保持现状。 |
| 🔵 建议 | done/model.go vs exportsummary/model.go | - | 两者 `UpdateKey` 签名不一致（done 返回 Model，exportsummary 返回 (Model, tea.Cmd)）。交接单已说明：Done 无异步 cmd 需求，合理。 | 可接受，已在交接单记录。 |

无 🔴 严重问题，无 🟡 警告。

## 结论
- [x] ✅ 审查通过
- [ ] 🔴 存在严重问题，必须修复后重新审查
- [ ] 🟡 存在警告，建议修复但不阻塞

代码实现严格遵守核心约定与勿动文件清单，编译/vet 通过，导出复用、错误处理、依赖注入、转场逻辑、异步消息、View 展示均符合模块19 要求。仅有少量 🔵 建议级别事项（主要是同步便利方法的测试覆盖），不阻塞。

## 下一步
进入**测试 Agent**，编写单元测试覆盖：
1. 成功导出（注入 mock Export 返回完整 Result，验证 Exported/Done/Result）
2. docx 降级（注入失败 DocxExporter 或 mock Result.DocxWritten=false，验证 View 降级提示）
3. 缺 confirmed refs（mock 返回 `export.Error{Code: CodeUnconfirmedReference}`，验证 Err 与 View 提示、不进入 Done）
4. 缺 accepted chapter（mock 返回 `export.Error{Code: CodeNoAcceptedChapters}`）
5. 重试逻辑（按 "r" 后状态重置并重新触发导出 cmd）
6. 转场（updateWriting/updateExportSummary/updateDone 路径）与 Done 页路径一致性（Outputs 与 Store.Path("final") 一致）
7. 建议覆盖 `ApplyExportResult`/`Run` 同步便利方法，避免成为未用代码。
