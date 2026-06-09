## 审查报告 - TUI全流程测试与Windows双击验收

### 1. 安全性
- [x] 测试不依赖真实 LLM 或真实网络 provider
- [x] 测试 API key 使用 `t.Setenv` 注入测试值，不读取或打印真实 secret
- [x] 文档记录真实 provider smoke 因无 API key 跳过，未触发外部调用

### 2. 并发与数据一致性
- [x] 新增测试使用 `t.TempDir()` 隔离工作目录
- [x] Store 写入仍复用现有原子写入接口
- [x] candidates 合并、confirmed references、final artifacts 均读取权威 Store 状态验证

### 3. 资源管理
- [x] 测试无持久外部资源依赖
- [x] 临时目录由 Go test 生命周期清理

### 4. 性能
- [x] 新增测试为离线 mock 流程，执行轻量
- [x] 无真实网络/LLM 等慢依赖

### 5. 错误处理
- [x] References 无确认文献硬门槛有测试覆盖
- [x] Windows 非交互控制台错误记录为可读错误，未声明为桌面双击通过
- [x] 真实 provider smoke 跳过原因已记录

### 6. 项目约定一致性
- [x] 符合 TUI 增量原则：TUI 编排状态，Store 是状态权威
- [x] ConfigWizard/project config 角色映射通过 `runtimeapp.ResolveRoleRuntime` 真实解析验证
- [x] SearchProgress → References RootModel 转场有测试覆盖
- [x] 材料 BibTeX 候选与搜索候选合并顺序有测试覆盖
- [x] 文档路径与 user-guide 保持一致

### 7. 代码质量
- [x] `internal/e2e/tui_flow_test.go` 已经审查 Agent 用 `gofmt -l` 复核无输出
- [x] `go test ./internal/e2e/...` 通过
- [x] `go test ./...` 通过
- [x] `go build ./cmd/aipaper-cli` 通过
- [x] 修复 Agent 已执行 `go build -o aipaper-cli.exe ./cmd/aipaper-cli` 通过

### 发现的问题
| 严重程度 | 文件 | 行号 | 描述 | 建议修复方式 |
|---|---|---|---|---|
| 无 | | | 未发现高置信度可行动问题 | |

### 结论
- [x] ✅ 审查通过
- [ ] 🔴 存在严重问题，必须修复后重新审查
- [ ] 🟡 存在警告，建议修复但不阻塞

### 审查证据
审查 Agent 复核结论：上一轮发现的 3 个问题均已修复：
1. Config/project config 已真实调用 `runtimeapp.ResolveRoleRuntime`。
2. RootModel `SearchProgress -> References` 转场已有测试覆盖。
3. Windows 双击验收文档已避免过度声明。

审查 Agent 已复核：
- `gofmt -l internal/e2e/tui_flow_test.go`：无输出
- `go test ./internal/e2e/...`：通过
- `go test ./...`：通过
- `go build ./cmd/aipaper-cli`：通过