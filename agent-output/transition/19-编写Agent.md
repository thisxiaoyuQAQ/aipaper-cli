# 模块19 编写Agent 交接单

## 任务信息
- **来源角色**: 编写 Agent
- **目标角色**: 审查 Agent
- **所属任务**: vault/19-ExportSummary与完成页.md
- **交接时间**: 2026-06-09

## 涉及文件

### 新增文件
1. `E:\Code\go\Paper-Cli\internal\tui\exportsummary\model.go`
   - ExportSummary model 实现
   - Options 支持依赖注入（Export、DocxExporter、Now）
   - 状态：exported、result、err、done、canceled
   - Init 触发导出、Update 处理 exportDoneMsg、按键：r 重试、b/esc 取消、enter 完成
   - 复用 export.LoadInput + export.ExportFinal（禁止自行拼接）
   
2. `E:\Code\go\Paper-Cli\internal\tui\exportsummary\view.go`
   - 展示输出目录（store.Path("final")）
   - 展示 Result.Outputs（文件列表）
   - docx 降级提示（DocxWritten=false 时明确说明 Markdown/report 已成功）
   - Result.Issues 列表展示
   - 错误处理（CodeNoAcceptedChapters / CodeUnconfirmedReference 提示不生成错误文件）
   
3. `E:\Code\go\Paper-Cli\internal\tui\done\model.go`
   - Done 完成页 model
   - Options{ WorkDir, Result }
   - 展示输出目录、文件列表（来自 Result.Outputs）
   - 高级命令提示（recover、status、config）
   - 按 q/ctrl+c/enter 退出

### 修改文件
4. `E:\Code\go\Paper-Cli\internal\tui\app\root.go`
   - 导入 exportsummarytui、donetui、export
   - RootModel 新增字段：ExportSummary、Done
   - ScreenExportSummary、ScreenDone 已在 Screen 枚举中（前置模块已定义）
   - KnownScreen 已包含这两个屏幕
   - NewRootModel：支持 InitialScreen 直接为 ScreenExportSummary/ScreenDone
   - Init：ScreenExportSummary 时触发 ExportSummary.Init()
   - Update：ScreenTransitionMsg 处理 ExportSummary/Done 转场、default 分支转发 exportDoneMsg 到 ExportSummary.Update
   - KeyMsg 分支：添加 updateExportSummary、updateDone
   - View、currentScreenView：添加 ExportSummary、Done 分支
   - updateWriting：Writing.Done() 时转场 ScreenExportSummary（构造 model 并 Init），不再直接 tea.Quit
   - newExportSummaryModel、newDoneModel 工厂方法
   - updateExportSummary：处理 Done() → 转场 ScreenDone、Canceled() → tea.Quit
   - updateDone：Quit() → tea.Quit

## 变更摘要

### 核心实现
- **ExportSummary**：薄封装，复用 export.LoadInput/ExportFinal（默认实现），Options 支持依赖注入便于测试 mock（Export、DocxExporter、Now）。
- **Done**：展示最终产物、下一步提示（recover/status/config），按任意确认键退出。
- **RootModel 桥接**：Writing.Done() → ExportSummary（Init 触发导出）→ Done → tea.Quit。
- **异步导出**：exportDoneMsg 通过 default 分支转发到 ExportSummary.Update。

### 依赖注入设计（供测试 mock）
- `exportsummary.Options.Export`: ExportFunc(store.Store) (Result, error) 默认调用 LoadInput+ExportFinal
- `exportsummary.Options.DocxExporter`: export.DocxExporter 默认 nil 用标准实现
- `exportsummary.Options.Now`: time.Time 默认零值用 time.Now().UTC()
- `done.Options.Result`: export.Result 注入导出结果（Done 页无需重算）

### 关键约束遵守
- ✅ 不修改 internal/export/ 任何文件
- ✅ 不修改 internal/tui/writing/ 核心逻辑，只在 root.go 边界接入
- ✅ 单文件行数：model.go 178 行、view.go 140 行、done/model.go 98 行，均 < 500
- ✅ 错误 wrap 用 fmt.Errorf（done 页无需）
- ✅ 时间用 UTC（通过 Options.Now 注入）
- ✅ 复用 export.LoadInput/ExportFinal，未自行拼接 final 内容

## 下游 Agent 需额外读取

### 审查 Agent
- `E:\Code\go\Paper-Cli\internal\export\export.go`（理解 Result、Issues、DocxWritten 语义）
- `E:\Code\go\Paper-Cli\internal\export\types.go`（Error、Result、Issue 定义）
- `E:\Code\go\Paper-Cli\internal\export\load.go`（LoadInput 逻辑）
- `E:\Code\go\Paper-Cli\internal\store\paths.go`（Store.Path）
- `E:\Code\go\Paper-Cli\internal\tui\writing\model.go`（Done()/Err()/Canceled() 方法风格参考）

### 测试 Agent
- 上述审查文件
- `E:\Code\go\Paper-Cli\internal\tui\writing\model_test.go`（测试风格参考）
- `E:\Code\go\Paper-Cli\项目备忘录.skill`（测试约定：依赖注入、错误场景、docx 降级）
- vault/19-ExportSummary与完成页.md（测试要求：成功导出、docx 降级、缺 confirmed refs、缺 accepted chapter、Done 页路径一致）

## 已知风险/待确认项

### 编译验证
- **状态**: 未完成
- **原因**: `go build ./...` 被 auto-mode classifier 阻止，无法验证编译通过
- **IDE 诊断**: 通过 mcp__ide__getDiagnostics，无错误诊断
- **人工审查**: 已检查导入、类型匹配、方法签名，理论无编译错误
- **建议**: 审查 Agent 或用户首先运行 `go build ./...` 确认编译通过

### 依赖注入点
- `exportsummary.Options.Export`: 测试可注入返回预设 Result 或 Error 的 mock
- `exportsummary.Options.DocxExporter`: 测试可注入返回错误的 exporter 触发降级
- `exportsummary.Options.Now`: 测试可注入固定时间确保 Result.Version 确定性

### 错误场景处理
- CodeNoAcceptedChapters：返回 Error，不生成错误文件 ✅
- CodeUnconfirmedReference：返回 Error，不生成错误文件 ✅
- CodeDocxFailed：记录 Issue，DocxWritten=false，paper.md/report.md 正常生成 ✅
- View 展示明确提示 ✅

### 文件路径一致性
- Done 页展示 Result.Outputs（来自 ExportFinal，路径相对 Store 根：`final/paper.md` 等）
- 输出目录展示 Store.Path("final")（绝对路径：`E:\Code\go\Paper-Cli\output\aipaper\final`）
- 一致性已确保

### 屏幕转场
- Writing.Done() → ExportSummary（构造 + Init）✅
- ExportSummary.Done() → Done（构造，传入 Result）✅
- Done → tea.Quit ✅

### UpdateKey 签名不一致
- `exportsummary.Model.UpdateKey(key) (Model, tea.Cmd)` 返回 cmd 以支持重试
- `done.Model.UpdateKey(key) Model` 仅返回 model（Done 无异步操作）
- 不一致但合理，因 Done 无 cmd 需求

### 本任务不写测试
- 按要求，本任务只实现代码，测试由测试 Agent 负责
- 但已确保代码可测（依赖可注入）

## 审查重点

1. **导出复用**: 确认 ExportSummary 未自行拼接 final 内容，仅调用 export.LoadInput/ExportFinal
2. **错误处理**: CodeNoAcceptedChapters / CodeUnconfirmedReference 返回 Error，不生成错误产物；CodeDocxFailed 记录 Issue，Markdown 正常生成
3. **依赖注入**: Options 设计是否便于测试 mock
4. **转场逻辑**: Writing → ExportSummary → Done → Quit 路径正确性
5. **异步消息**: exportDoneMsg 通过 default 分支转发，Update 正确处理
6. **View 展示**: 输出目录、文件列表、docx 降级提示、Issues、错误提示是否完整清晰
7. **代码风格**: 是否符合项目 Go 风格（error wrap、时间 UTC、单文件行数）

## 下一步
1. 审查 Agent 审查代码（特别是导出复用、错误处理、转场逻辑）
2. 测试 Agent 编写单元测试（成功导出、docx 降级、缺引用/章节错误、Done 页展示）
3. 若审查通过且测试覆盖充分，模块19 可标记完成
