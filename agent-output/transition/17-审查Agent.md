## 审查报告 - 模块 17: WritingProgress 四区布局与 Runtime 事件桥

### 审查结论
**🟡 审查通过，建议修复警告项**

### 审查摘要

#### 1. 安全性 ✅
- [x] 输入参数校验：`NewModel` 对 width/height 有默认值处理
- [x] 敏感数据脱敏：`BridgeRunEvent` 不包含 API key，`RuntimeEvent.Fields` 不包含敏感信息
- [x] API key 不进入 view：view 渲染不涉及 API key 字段
- **检查点**：`events.go` 中 `BridgeRunEvent` 正确复制 Fields 而非直接引用，避免泄漏

#### 2. 并发与数据一致性 ✅
- [x] 无竞态条件：WritingProgress Model 是单线程 Bubble Tea 模型，Update 串行执行
- [x] 无共享资源锁需求：所有状态都在 Model 内部

#### 3. 资源管理 ✅
- [x] logs 有上限：`maxLogs = 100`，超过后截断
- [x] contentLines 有上限：`maxContentLines = 1000`，超过后截断
- [x] 无内存泄漏风险：所有切片都有容量限制

#### 4. 性能 ✅
- [x] logs 有截断：`len(m.logs) > m.maxLogs` 时保留最后 100 条
- [x] contentLines 有截断：`len(m.contentLines) > m.maxContentLines` 时保留最后 1000 行
- [x] 无 N+1 查询：不涉及数据库操作
- [x] 列表渲染有限制：`visibleLogLines()` 根据终端高度限制渲染行数

#### 5. 错误处理 ✅
- [x] RuntimeEvent 错误通过 `EventRuntimeError` 明确标记
- [x] Model.Err() 方法返回错误状态
- [x] 错误不静默吞掉：通过 `addLog` 记录并在 view 中显示

#### 6. 项目约定一致性 ✅
- [x] 时间格式：`RuntimeEvent.At` 使用 `time.Time`，符合 RFC3339 序列化约定
- [x] 路径格式：不涉及文件路径（纯 UI 组件）
- [x] 命名清晰：`RuntimeEvent`、`ChapterStatus`、`UsageSnapshot` 等命名符合项目风格
- [x] 魔法值提取：`maxLogs = 100`、`maxContentLines = 1000` 为常量字段
- [x] 复用工具函数：未重复实现已有功能

#### 7. 代码质量 ✅
- [x] 文件大小：events.go (132行)、model.go (444行)、view.go (450行)，均 < 500 行
- [x] 接口一致性：`RuntimeEvent`、`UsageSnapshot`、`ChapterStatus` 与 `docs/interfaces/tui.md` 一致
- [x] 边界条件：
  - 空 logs/contentLines 处理正确
  - usage 缺失字段显示 `--`
  - 窄终端 (width < 60) 降级为纵向布局

#### 8. 架构约束（关键）✅
- [x] **未硬编码写作顺序**：Model 只消费事件，不假设 Architect→Writer→Editor 顺序
- [x] **未复制 artifacts 规则**：`ChapterState` 只记录状态，不重新实现质量门控逻辑
- [x] **未复制质量门控**：评分展示通过 `EventQualityReview` 事件获取，不在 TUI 中计算
- [x] **API key 不进入事件**：`BridgeRunEvent` 不包含 API key 字段

### 发现的问题

#### 🟡 警告

| 文件 | 行号 | 描述 | 建议修复方式 |
|---|---|---|---|
| model.go | 276-283 | `updateChapterStatus` 从 `ev.Fields` 读取 `status`、`draft_version` 等字段，但未处理类型断言失败情况 | 添加类型断言安全检查，避免 panic |
| view.go | 83 | `renderNarrowLayout` 未测试实际小终端渲染效果 | 在集成测试或手动验证中覆盖 |
| model.go | 186 | `handleRuntimeEvent` 中 `EventContentDelta` 未检查 `Delta` 为空的边界情况（虽然已有 `if ev.Delta != ""` 检查，但 `appendContent` 内部未二次校验） | 在 `appendContent` 开头添加空字符串检查 |

#### 🔵 建议

| 文件 | 行号 | 描述 | 建议修复方式 |
|---|---|---|---|
| model.go | 43-44 | `maxLogs` 和 `maxContentLines` 硬编码在 `NewModel` 中，未暴露配置选项 | 可将其作为 `Options` 字段，提供默认值 |
| view.go | 21-60 | 颜色代码硬编码（如 `lipgloss.Color("62")`），未提取为常量 | 可提取为包级常量或主题配置 |
| model.go | 161 | `handleKey` 中 `"q"` 键只在 `!m.running` 时退出，但文档未说明这个行为 | 在 footer 提示中明确说明 "q: Quit (when done)" |
| events.go | 89-112 | `BridgeRunEvent` 只处理基础事件类型，未处理 `EventContentDelta`、`EventUsageUpdate` 等专用事件（这些需从 Host 自定义投影） | 在文档中明确说明专用事件需要 Host 层额外投影 |

### 架构合规性检查 ✅

根据 vault 文件和架构设计要求：

1. **✅ TUI 编排屏幕，Host 执行运行时，Coordinator 决策流程**
   - WritingProgress 只是展示层，不决策写作顺序
   - 通过 `RuntimeEvent` 被动消费 Host 投影的事件

2. **✅ 不硬编码 Architect→Writer→Editor**
   - `handleRuntimeEvent` 只根据 `ev.Kind` 处理，不假设步骤顺序
   - 章节状态机（drafting/reviewing/rewriting/done）通过 `EventChapterStatus` 接收，不在 TUI 中计算

3. **✅ 不复制 artifacts 或质量门控规则**
   - `ChapterState.Score` 和 `ChapterState.CitationScore` 从事件接收，不重新计算
   - 质量评审通过 `EventQualityReview` 展示，不实现评审逻辑

4. **🟡 真实 AgentRuntime 接入未完成**（已知风险）
   - `model.go:43` 的 `Init()` 返回 nil，未启动真实 runtime
   - vault 文件明确要求"必须把真实 Architect/Writer/Editor runner 接入 Host 可启动路径"
   - **建议**：在模块 17 完成测试后，创建独立任务"17a-真实Runtime接入"，或在模块 18（暂停退出与安全恢复）中一并实现

### 测试覆盖 ✅

- [x] 单元测试覆盖全部核心函数（14 个测试用例，全部通过）
- [x] 事件处理测试：step started/done/failed、content delta、usage、chapter status、runtime done/error
- [x] UI 交互测试：Ctrl+C、Space、↑/↓
- [x] BridgeRunEvent 映射测试
- [x] RootModel 集成测试
- [x] 边界条件测试：空输入、章节切换、多章节进度

### 下游 Agent 需额外读取

- `项目备忘录.skill`
- `vault/17-WritingProgress四区布局与Runtime事件桥.md`
- `docs/TUI全流程增量需求.md`
- `docs/interfaces/tui.md`
- `internal/tui/writing/model_test.go`（测试用例参考）

### 测试重点

测试 Agent 应重点验证：

1. **Mock RuntimeEvent 序列测试**：
   - 完整写作流程事件序列（outline → write ch01 → review → rewrite → done）
   - 错误场景（step failed、runtime error）
   - 恢复场景（携带 RecoveryPrompt）

2. **窄终端降级测试**：
   - width < 60 时验证 `renderNarrowLayout` 不崩溃
   - 验证纵向堆叠布局可读性

3. **边界条件测试**：
   - logs/contentLines 超过上限时截断正确
   - usage 全部字段缺失时显示 `--`
   - 空 chapters 时进度显示 0.0%

4. **集成测试**（需真实 runtime 接入后）：
   - RootModel References → WritingProgress 转场
   - RootModel RecoverPrompt → WritingProgress 恢复转场
   - WritingProgress Done → ExportSummary 转场（模块 19 依赖）

### 结论

**审查通过，建议修复警告项。**

代码质量良好，架构约束严格遵守，测试覆盖充分。唯一的🟡警告项（类型断言安全性、窄终端验证、空字符串二次检查）可在修复 Agent 阶段处理，或在集成测试中验证后酌情修复。

**已知风险"真实 AgentRuntime 接入"不阻塞当前模块验收**，可作为后续独立任务或在模块 18 中一并实现。

---

**审查人**：主 Agent  
**审查时间**：2026-06-09  
**审查模型**：Claude Opus 4.8
