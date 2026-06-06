# 07-StepCheckpoint恢复

## 模块做啥（1 行）

把现有 checkpoint 记录与校验扩展为完整恢复流程，覆盖 Host 恢复 prompt、幂等写入和冲突处理。

## 依赖谁（1 行）

- 必须先完成：vault/01-基础脚手架与配置Store.md
- 可并行：vault/05-Agent运行时与Coordinator.md

## 需要先读哪几个文件（2~5 个）

- 项目备忘录.md
- docs/需求与架构.md「Step 级恢复」「输出目录契约」
- docs/interfaces/checkpoint.md
- internal/checkpoint/checkpoint.go
- internal/store/atomic.go

## 接口与类型

- `checkpoint.Checkpoint`
- `checkpoint.OutputArtifact`
- `checkpoint.Validation`
- `Record(s, cp, progress)`：写 step、latest、progress。
- `ValidateLatest(s)`：校验 latest 指向的 artifact 存在、路径安全、哈希匹配。

## 实现要点

- 保持 artifact 先写成功，再调用 `Record` 写 checkpoint。
- step checkpoint 使用 CreateOnly，latest/progress 使用 Overwrite。
- path 校验必须拒绝绝对路径和逃逸 Store 根路径。
- 恢复 prompt 由 Host 生成，内容包含当前 step、phase、next_expected、checked artifacts、不可重复操作。
- `run.json.resumed_from` 在恢复运行时指向 latest step。
- CLI `recover` 输出应对用户可读，也可被测试解析。

## 测试要点

- latest 不存在、artifact 不存在、哈希不匹配、路径逃逸都应 `OK=false`。
- 正常 checkpoint 校验 `OK=true`，`Checked` 包含输出路径。
- 重复 step 且内容不同返回冲突错误。
- 模拟恢复后不覆盖已有 draft/review artifact。

## 产出清单

- internal/checkpoint/checkpoint.go
- internal/app/recover.go 或 Host 恢复入口
- internal/cli recover 相关测试
- 对应 `*_test.go`

## 行数预估

- 现有文件可继续使用；新增恢复 prompt 构造单独拆文件。
