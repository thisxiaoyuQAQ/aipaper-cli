# 交接单：模块 15 编写 Agent → 审查 Agent

## 基本信息

- **来源 Agent**: 编写 Agent
- **目标 Agent**: 审查 Agent
- **所属任务**: vault/15-SearchProgress 与候选合并
- **完成时间**: 2026-06-09
- **工作目录**: E:\Code\go\Paper-Cli

## 涉及文件清单

### 新增文件

1. `internal/tui/search/model.go` (245 行)
   - SearchProgress TUI model
   - Status: searching/complete/all_failed/disabled/error
   - Action: continue/retry/skip/back/cancel
   - 搜索与候选合并逻辑

2. `internal/tui/search/view.go` (63 行)
   - SearchProgress TUI view
   - 展示搜索状态、provider 错误、候选数量统计

3. `internal/tui/search/model_test.go` (398 行)
   - 覆盖全部 vault 测试要求
   - 搜索关闭/开启场景
   - DOI/URL 去重场景
   - provider 部分/全部失败场景
   - ID 连续性验证

4. `internal/references/writer.go` (69 行)
   - WriteCandidates: 公共候选写入函数
   - FormatCandidatesMarkdown: 公共 markdown 格式化函数
   - 替代 search.Run 中原有的私有 formatCandidatesMarkdown

5. `internal/references/writer_test.go` (155 行)
   - WriteCandidates 测试
   - FormatCandidatesMarkdown 格式测试
   - 空列表、完整字段、最小字段、多候选场景

### 修改文件

1. `internal/search/run.go`
   - writeResult 函数改为调用 `references.WriteCandidates`
   - 删除私有 formatCandidatesMarkdown 函数（已提取为公共 helper）
   - 删除未使用的 `fmt` import

2. `internal/tui/app/root.go`
   - 新增 searchtui import
   - RootModel 新增 Search 字段 (searchtui.Model)
   - Init() 增加 ScreenSearchProgress 分支
   - Update() 增加 ScreenSearchProgress 转场逻辑和 SearchFinishedMsg 处理
   - View() 增加 ScreenSearchProgress 分支
   - 新增 updateSearch() 处理搜索屏按键（Continue→References、Back→Materials、Cancel→Quit）
   - 新增 newSearchModel() 从 materialstui.ScanResult 和 requirements.json 构建搜索 model

## 变更摘要

### 核心功能

1. **搜索与合并**:
   - 根据 `requirements.AllowOnlineSearch` 决定是否调用 `search.Run`
   - 合并材料候选 + 搜索候选
   - 用 `references.DedupeCandidates` 去重
   - 用 `references.AssignCandidateIDs(..., 1)` 重新分配连续 ID（从 cand_001）
   - 写入最终 `references/candidates.json` 和 `references/candidates.md`（覆盖 search.Run 已写内容）

2. **容错与用户选择**:
   - provider 单点失败不阻塞，UI 展示错误
   - 所有 provider 失败时提供：重试(R)、跳过(S)、返回材料(B)
   - 搜索成功或禁用时 Enter 继续到 References 屏

3. **代码复用**:
   - 提取 `references.WriteCandidates` 和 `references.FormatCandidatesMarkdown` 为公共 helper
   - `internal/search/run.go` 改为复用该 helper，避免重复实现格式化逻辑

### RootModel 集成

- Materials 屏 Continue/Skip → SearchProgress 屏（传入 materialstui.ScanResult）
- SearchProgress 屏 Continue/Skip → References 屏（传入 []contracts.ReferenceCandidate）
- SearchProgress 屏 Back → Materials 屏（重新初始化）
- SearchProgress 屏 Cancel → Quit

### 测试覆盖

- 搜索关闭时材料候选保留（2 个候选 → 2 个最终候选，ID cand_001/002）
- 搜索开启时材料+搜索候选都保留（1+2 → 3 个最终候选）
- DOI 重复去重且字段合并（保留更完整的 abstract）
- URL 重复去重且字段合并（保留更完整的 venue、authors）
- provider 部分失败不阻塞（1 成功 + 1 失败 → status=complete，显示错误）
- 所有 provider 失败可重试/跳过/返回（status=all_failed）
- 最终 ID 从 cand_001 连续
- 搜索错误可重试（status=error）
- DedupeGroup 正确设置

## 下游需额外读取文件

审查 Agent 建议额外读取以验证集成正确性：

1. `vault/15-SearchProgress与候选合并.md`（任务权威定义）
2. `internal/contracts/types.go`（ReferenceCandidate、ReferenceCandidates 定义）
3. `internal/references/dedupe.go`（DedupeCandidates、AssignCandidateIDs 实现）
4. `internal/tui/materials/model.go`（ScanResult 结构参考）
5. `docs/interfaces/tui.md`（TUI 接口约定）

## 已知风险与待确认项

### 已知风险

1. **References 屏未实现**：
   - 本任务 ScreenReferences 转场后停在占位状态（View 返回 `aipaper-cli\n\nreferences\n`）
   - References 屏的实际接入是模块 16，本任务已为其传递正确数据类型 `[]contracts.ReferenceCandidate`

2. **Search 屏 Back 行为**：
   - updateSearch 中 ActionBack 返回 Materials 屏时会重新初始化 MaterialsModel 并触发扫描
   - 可能导致用户已跳过的材料目录再次被扫描
   - 审查时需确认此行为是否符合预期用户体验

### 待确认项

1. **搜索错误重试逻辑**：
   - 当前 StatusError 时按 R 会重试，但 search.Run 可能因 store/requirements 问题持续失败
   - 审查时建议确认是否需要限制重试次数或提供"跳过搜索"选项

2. **材料候选为空时的行为**：
   - 材料扫描 Skip 会传入空 Candidates 列表
   - 搜索也可能返回空结果
   - 当前最终 candidates.json 会写入空列表，References 屏需能处理此情况

3. **searchCmd 无 context cancel**：
   - 当前 searchCmd 用 `context.Background()`，用户按 Cancel 时无法中断正在进行的搜索
   - 若搜索耗时长，用户体验可能不佳
   - 审查时建议确认是否需要支持搜索中途取消

## 构建与测试结果

- `go build ./...`: ✓ 通过
- `go test ./...`: ✓ 全部通过
  - internal/tui/search: ok 0.357s
  - internal/references: ok 0.326s
  - internal/search: ok 0.384s（既有测试未受影响）
  - internal/tui/app: ok 0.463s

## 关键设计决策

1. **候选合并策略**：先合并材料+搜索候选为单一列表，再统一去重和分配 ID，确保 ID 连续且不受来源影响。

2. **格式化复用**：将 markdown 格式化逻辑提取为 `references.FormatCandidatesMarkdown`，避免 TUI 层手写漂移格式，保持与 search.Run 输出一致。

3. **错误展示**：provider 错误存储在 model.searchErrors 中，view 根据 status 决定展示方式（complete 时为警告，all_failed 时为完整错误列表）。

4. **Status 语义**：
   - `StatusSearching`: 正在执行搜索
   - `StatusDisabled`: AllowOnlineSearch=false，仅使用材料候选
   - `StatusComplete`: 搜索成功（即使部分 provider 失败）
   - `StatusAllFailed`: 所有 provider 失败且无搜索结果
   - `StatusError`: search.Run 本身返回错误

5. **测试注入**：SearchFunc 可注入，便于测试时 mock search.Run 返回，避免真实网络调用。

## 遗留风险

- 无
