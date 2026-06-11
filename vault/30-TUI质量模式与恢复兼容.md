# 30-TUI质量模式与恢复兼容

**状态**: ✅ 已完成

## 任务目标

TUI 接入 Quality Engine：Requirements 模式选择、WritingProgress/ExportSummary 质量信息展示、StateProbe/RecoverPrompt 恢复兼容。

## 前置依赖

- 29-QualityReport导出汇总
- 10-22 TUI 全流程增量任务

## 需要读取

- `项目备忘录.skill`
- `docs/interfaces/quality.md`
- `docs/superpowers/specs/2026-06-10-quality-engine-design.md`（第 5、6 节）
- `docs/interfaces/tui.md`
- `internal/tui/requirements/`（勿动，桥接扩展）
- `internal/tui/writing/`、`internal/tui/app/`

## 实现要求

### 1. Requirements 模式选择

- 表单新增 `quality_mode` 字段：`fast` / `enhanced`（默认）/ `strict`，落盘 `requirements.json`；
- 旧 requirements.json 无该字段：新 run 按 enhanced；恢复旧 run 按兼容模式；
- `internal/tui/requirements` 勿动，通过桥接层扩展字段。

### 2. WritingProgress 增强

- 四区布局不重排：
  - 步骤区显示 `evidence_extraction` / `section_quality_plan` / `claim_extraction` / `claim_verification`；
  - 章节进度状态新增 `verifying` / `needs_revision`；
  - 日志区显示硬门槛阻断与风险分级事件（不展示完整 evidence 内容）；
- RuntimeEvent 投影扩展对应事件类型，不泄露 secret（沿用模块 22 规则）。

### 3. ExportSummary 增强

- 新增 quality-report.md 入口；
- 显示一行整体质量结论（如「质量门控：8 章通过，1 章 needs_human_review」）。

### 4. StateProbe / RecoverPrompt

- StateProbe 探测 `quality/` 产物存在性与 run 的 quality_mode；
- 旧 run 无质量产物 → 兼容模式（warnings-only，不阻断）；
- 新 run 中途恢复 → 从最近完成的质量 step 续跑；
- RecoverPrompt 文案注明当前 run 的质量模式（三档）。

### 5. 不改动

- ConfigWizard 不新增质量配置；
- References 确认交互不变。

## 测试要求

- 模式选择落盘与默认值测试；
- WritingProgress 新事件渲染测试（含窄窗口降级）；
- ExportSummary 质量结论行测试；
- StateProbe 兼容模式与新 run 恢复测试；
- RecoverPrompt 三档模式文案测试。

## 验收标准

- `go test ./...` 通过；
- mock TUI 全流程（tui_flow_test）扩展后通过且不回归；
- 旧项目恢复路径不被破坏。

## 已知风险/边界

- 勿动文件只桥接不重写。

## 行数预估

- requirements 桥接 ≈ 100 行、writing/exportsummary/stateprobe/recoverprompt 各 ≈ 80-150 行 + 测试 ≈ 400 行；单文件 < 500 行。
