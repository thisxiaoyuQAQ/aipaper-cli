# 接口与类型定义文档索引

本目录按模块拆分记录 `aipaper-cli` 的数据契约与类型定义。所有字段以已实现的 Go 结构体（`internal/contracts/types.go`、`internal/checkpoint/checkpoint.go`、`internal/config/config.go`、`internal/store/*.go`）为权威来源，JSON tag 与之完全一致。设计依据为已批准 spec：`docs/superpowers/specs/2026-06-05-aipaper-cli-design.md`。

## 文件列表

| 文件 | 一句话说明 | 对应 Go 包 / 源文件 |
| --- | --- | --- |
| [common.md](./common.md) | 公共类型：结构化错误契约、错误码枚举、原子写入约定、Progress / Run / RunEvent | `internal/contracts/types.go`、`internal/store/atomic.go`、`internal/store/hash.go` |
| [requirements.md](./requirements.md) | 写作需求 `Requirements` 结构、`requirements.json` 示例与校验规则 | `internal/contracts/types.go` |
| [materials.md](./materials.md) | 材料清单 `MaterialManifest` / `MaterialItem`、`manifest.json` 示例与格式支持分层 | `internal/contracts/types.go` |
| [search.md](./search.md) | 学术搜索输入、数据源、标准化字段与去重策略 | `internal/contracts/types.go`（复用 `ReferenceCandidate`） |
| [references.md](./references.md) | 候选文献 / 确认文献结构、示例与 reference key 生成规则 | `internal/contracts/types.go` |
| [artifacts.md](./artifacts.md) | Outline / Claims / CitationMap / Review 等写作产物契约与章节状态机 | `internal/contracts/types.go` |
| [checkpoint.md](./checkpoint.md) | `Checkpoint` / `OutputArtifact` / `Validation` / `RecoveryResult`、step 列表、崩溃一致性与幂等 | `internal/checkpoint/checkpoint.go`、`internal/app/recover.go` |
| [export.md](./export.md) | `final/` 交付产物契约与 `citation-trace.json` 字段 | `internal/export` |
| [config.md](./config.md) | `Config` / `ProviderConfig` / `RoleConfig`、查找顺序与合并规则 | `internal/config/config.go` |
| [agent.md](./agent.md) | Architect / Writer / Editor 输入输出契约、引用硬规则与质量门控阈值 | spec 第 3 节 + `internal/contracts/types.go` |

## 通用约定

- **Store 根目录**：`output/aipaper/`（常量 `contracts.DefaultOutputDir="output"`、`contracts.DefaultProject="aipaper"`）。
- **路径表示**：checkpoint 中 `outputs[].path` 必须是相对 Store 根、使用正斜杠 `/` 的相对路径。
- **JSON 序列化**：统一 `MarshalIndent`（两空格缩进）+ 末尾换行；读取时 `DisallowUnknownFields`，多余字段会报错。
- **时间字段**：Go `time.Time`，序列化为 RFC3339（UTC）。
