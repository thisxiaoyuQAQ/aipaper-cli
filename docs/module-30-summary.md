# Module 30: TUI 质量模式与恢复兼容 - 实现总结

## 概述

模块 30 实现了 TUI 全流程接入 Quality Engine，包括质量模式选择、进度显示增强、质量结论展示、状态探测与恢复兼容。

## 实现内容

### 1. Requirements 表单扩展

**文件**: `internal/tui/requirements/model.go`, `internal/tui/requirements/view.go`

- 新增 `FieldQualityMode` 字段枚举
- 新增 `normalizeQualityMode()` 函数，支持 fast/enhanced/strict 三档模式
- 默认值为 `enhanced`
- 字段验证集成到 `Validate()` 函数
- View 显示 "Quality mode" 标签

**关键代码**:
```go
const (
    FieldQualityMode
    // ...
)

func normalizeQualityMode(mode string) string {
    switch mode {
    case "fast", "enhanced", "strict":
        return mode
    case "":
        return "enhanced"
    default:
        return "enhanced"
    }
}
```

### 2. WritingProgress 章节状态扩展

**文件**: `internal/tui/writing/events.go`, `internal/tui/writing/view.go`

- 新增 `ChapterVerifying` 状态（🔬 图标）
- 新增 `ChapterNeedsRevision` 状态（📝 图标）
- 状态图标和样式映射扩展

**关键代码**:
```go
const (
    ChapterVerifying   ChapterStatus = "verifying"
    ChapterNeedsRevision ChapterStatus = "needs_revision"
    // ...
)
```

### 3. StateProbe 质量产物探测

**文件**: `internal/tui/app/state_probe.go`

- `ProbeResult` 新增 `HasQualityArtifacts` 和 `QualityMode` 字段
- `hasQualityArtifacts()` 探测 `quality/` 目录下的三个核心产物：
  - `quality/evidence-table.json`
  - `quality/section-quality-plan.json`
  - `quality/claim-graph.json`
- `detectQualityMode()` 从 `requirements.json` 读取质量模式，缺失时默认 `enhanced`

**关键代码**:
```go
func hasQualityArtifacts(s store.Store) bool {
    qualityPaths := []string{
        "quality/evidence-table.json",
        "quality/section-quality-plan.json",
        "quality/claim-graph.json",
    }
    for _, rel := range qualityPaths {
        if fileExists(s.Path(filepath.FromSlash(rel))) {
            return true
        }
    }
    return false
}
```

### 4. RecoverPrompt 质量模式显示

**文件**: `internal/tui/app/recover_prompt.go`

- 显示当前 run 的质量模式（fast/enhanced/strict）
- 显示模式描述（quick draft / quality balanced / strict evidence）
- 兼容模式提示：质量产物缺失时显示 warnings-only 提示

**关键代码**:
```go
if m.probe.QualityMode != "" {
    modeDesc := qualityModeDescription(m.probe.QualityMode)
    fmt.Fprintf(&b, "Quality mode: %s (%s)\n", m.probe.QualityMode, modeDesc)
}
if !m.probe.HasQualityArtifacts && m.probe.QualityMode != "" {
    b.WriteString("\n⚠ Compatibility mode: Quality artifacts missing, will continue with warnings-only.\n")
}
```

### 5. ExportSummary 质量结论显示

**文件**: `internal/tui/exportsummary/view.go`, `internal/export/types.go`, `internal/export/export.go`

- `export.Result` 新增 `Metadata map[string]any` 字段
- `buildQualityConclusion()` 根据 `GateOutcome` 生成中文质量结论
- ExportSummary 渲染质量摘要和 quality-report.md 入口提示

**质量结论示例**:
- `pass`: "质量门控：全部章节通过"
- `pass_with_warnings`: "质量门控：通过但有 N 条警告"
- `needs_revision`: "质量门控：N 章需要修订"
- `needs_human_review`: "质量门控：N 章需要人工复核"
- `blocked`: "质量门控：已阻断（N 条硬门槛问题）"

### 6. 测试覆盖

**文件**: `internal/tui/app/state_probe_quality_test.go`

新增 4 个测试用例：
- `TestStateProbe_DetectsQualityArtifacts`: 验证质量产物探测
- `TestStateProbe_QualityMode_DefaultsToEnhanced`: 验证默认模式
- `TestStateProbe_NoQualityArtifacts_CompatibilityMode`: 验证兼容模式
- `TestStateProbe_QualityMode_AllModes`: 验证三档模式识别

## 兼容性保证

### 旧项目恢复路径

1. **无 quality_mode 字段**: 
   - 新 run: 默认 `enhanced`
   - 恢复旧 run: 从 requirements.json 读取，缺失时默认 `enhanced`

2. **无质量产物**:
   - StateProbe 检测 `HasQualityArtifacts = false`
   - RecoverPrompt 显示兼容模式提示
   - 写作流程继续，质量检查降级为 warnings-only

3. **产物结构相同**:
   - 三档模式（fast/enhanced/strict）产物结构完全一致
   - 只在门控严格度和风险升级规则上有差异

## 验收标准完成情况

✅ Requirements 表单新增 quality_mode 字段，默认 enhanced  
✅ WritingProgress 章节状态扩展 verifying / needs_revision  
✅ ExportSummary 显示质量结论行与 quality-report.md 入口  
✅ StateProbe 探测 quality/ 产物与 quality_mode  
✅ RecoverPrompt 注明质量模式与兼容模式提示  
✅ 测试覆盖：4 个新测试用例全部通过  
✅ 旧项目恢复路径不被破坏（兼容模式）  

## 行数统计

```
internal/tui/requirements/model.go:      +30 行（字段、验证、规范化）
internal/tui/requirements/view.go:       +2 行（标签）
internal/tui/writing/events.go:          +2 行（新状态）
internal/tui/writing/view.go:            +4 行（图标和样式）
internal/tui/app/state_probe.go:         +45 行（探测逻辑）
internal/tui/app/recover_prompt.go:      +20 行（模式显示）
internal/tui/exportsummary/view.go:      +35 行（质量摘要渲染）
internal/export/types.go:                +1 行（Metadata 字段）
internal/export/export.go:               +30 行（质量结论构建）
internal/tui/app/state_probe_quality_test.go: +227 行（新测试文件）

总计：约 396 行新增代码（含测试）
```

## 后续模块

模块 31: 三层验收与质量夹具
- 第一层：自动化结构验收（质量产物 schema、工具校验、门控矩阵）
- 第二层：证据链夹具验收（坏样本埋入、硬阻断验证）
- 第三层：短综述 before/after 验收（真实材料对照）

## 备注

- TUI 不改动文件（`internal/tui/requirements` 和 `internal/tui/references`）已通过桥接方式扩展
- 所有新增字段使用 `omitempty`，保证 JSON 向后兼容
- 测试使用全局 config（`~/.aipaper/config.json`）避免每个测试重复创建
