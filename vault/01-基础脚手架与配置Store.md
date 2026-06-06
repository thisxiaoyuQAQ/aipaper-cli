# 01-基础脚手架与配置Store

## 模块做啥（1 行）

建立 CLI 入口、配置加载、Store 布局、原子写入、基础状态文件和对应测试。

## 依赖谁（1 行）

- 必须先完成：无
- 可并行：vault/07-StepCheckpoint恢复.md

## 需要先读哪几个文件（2~5 个）

- 项目备忘录.md
- docs/需求与架构.md「技术栈」「核心约定」「CLI 接口速查」
- internal/cli/cli.go
- internal/config/config.go
- internal/store/*.go

## 接口与类型

- `config.Config` / `ProviderConfig` / `RoleConfig`：全局、项目级、显式配置合并。
- `store.Store`：Store 根目录、固定路径、相对路径生成。
- `store.WriteJSON` / `WriteFile`：CreateOnly / Overwrite 两种写入模式。
- `contracts.Run` / `Progress`：`run.json` 和 `progress.json`。
- CLI 命令：`init`、`status`、`recover`、`config`。

## 实现要点

- 保持 Host 薄外壳原则，CLI 只完成命令解析和基础调用。
- 配置查找顺序：`~/.aipaper/config.json`、`./aipaper.json`、`--config`。
- `config` 命令输出时必须 redact `api_key`。
- Store 必须创建 `checkpoints`、`materials`、`references`、`outline`、`drafts`、`reviews`、`final`。
- `WriteFile` 保持 temp + fsync + rename + syncDir。
- 补齐单元测试：配置合并、校验、redact、Store 路径、CreateOnly 冲突、JSON 读取多余字段。
- 补齐现有 `docs/interfaces/_index.md` 中已列但缺失的配置接口文档。

## 测试要点

- `go test ./...`
- 使用临时目录验证 `init` 创建布局且重复运行不破坏既有 run/progress。
- 验证 `config` 命令不会输出真实 API key。
- 验证 CreateOnly 对相同内容幂等、不同内容报冲突。

## 产出清单

- internal/cli/cli.go
- internal/app/bootstrap.go
- internal/config/config.go
- internal/store/*.go
- internal/contracts/types.go
- docs/interfaces/config.md
- 对应 `*_test.go`

## 行数预估

- 单个源码文件目标 < 500 行；测试可按包拆分。
