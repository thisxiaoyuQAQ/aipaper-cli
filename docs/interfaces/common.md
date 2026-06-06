# 公共类型与约定

本文件记录跨模块共享的契约：结构化错误、错误码枚举、写入约定、SHA256、store 路径，以及运行期状态 `Progress` / `Run` / `RunEvent`。

来源：`internal/store/atomic.go`、`internal/store/hash.go`、`internal/store/paths.go`、`internal/contracts/types.go`，错误码来自 spec 第 5 节。

## 1. 结构化错误契约

所有工具失败必须返回结构化 JSON，而非自然语言（spec 第 5 节）：

```json
{
  "ok": false,
  "error": {
    "code": "REFERENCE_SEARCH_TIMEOUT",
    "message": "Semantic Scholar request timed out",
    "retryable": true,
    "details": {
      "source": "semantic_scholar",
      "timeout_ms": 30000
    }
  }
}
```

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `ok` | bool | 固定为 `false` 表示失败；成功路径返回 `{"ok": true, ...}` |
| `error.code` | string | 错误码枚举（见下） |
| `error.message` | string | 人类可读描述 |
| `error.retryable` | bool | 是否可由系统自动重试 |
| `error.details` | object | 任意上下文键值，按错误类型而定 |

## 2. 错误码枚举（按 spec 第 5 节错误类型）

| 类别 | 典型 code | retryable |
| --- | --- | --- |
| 配置错误 | `CONFIG_MISSING_API_KEY` / `CONFIG_PROVIDER_UNSUPPORTED` / `CONFIG_MODEL_UNAVAILABLE` | 否 |
| 材料错误 | `MATERIAL_NOT_FOUND` / `MATERIAL_FORMAT_UNSUPPORTED` / `MATERIAL_PARSE_FAILED` | 否 |
| 搜索错误 | `REFERENCE_SEARCH_TIMEOUT` / `REFERENCE_SEARCH_RATE_LIMITED` / `REFERENCE_SEARCH_FIELD_MISSING` | 是（超时/限流） |
| 文献错误 | `REFERENCE_CANDIDATES_EMPTY` / `REFERENCE_NONE_CONFIRMED` | 否 |
| 写作错误 | `WRITER_INVALID_JSON` / `WRITER_CHAPTER_CONTRACT_MISSING` | 否 |
| 评审错误 | `REVIEW_CLAIMS_MISSING` / `REVIEW_CITATION_MAP_MISSING` / `REVIEW_UNSUPPORTED_CITATION` | 否 |
| 导出错误 | `EXPORT_DOCX_FAILED` | 是 |
| 存储错误 | `STORE_WRITE_FAILED` / `STORE_HASH_MISMATCH` / `STORE_CHECKPOINT_CONFLICT` | 否 |

重试策略：可重试外部错误（搜索超时、LLM 临时错误、导出临时失败）重试 2–3 次；需求缺字段、确认文献为空、artifact 哈希冲突需用户处理。Editor 不通过不是系统错误，而是质量门控结果。

## 3. 写入约定：WriteMode / WriteResult

来源 `internal/store/atomic.go`：

```go
type WriteMode int

const (
    CreateOnly WriteMode = iota // 已存在且哈希一致 → AlreadyExists；哈希不同 → 冲突错误
    Overwrite                   // 直接覆盖（latest.json / progress.json 等可变文件）
)

type WriteResult struct {
    Path          string
    SHA256        string
    AlreadyExists bool
}
```

- `WriteJSON` / `WriteFile` 采用 **temp + fsync + rename** 原子写入，写完 `syncDir` 父目录。
- `CreateOnly`：目标已存在且内容哈希相同 → 返回 `AlreadyExists=true`（幂等）；内容不同 → 返回 `artifact conflict at <path>` 错误。
- JSON 统一 `MarshalIndent("", "  ")` + 末尾换行；写入前 `json.Valid` 校验。

## 4. SHA256 约定

- 由 `internal/store/hash.go` 的 `SHA256(data)` 与 `FileSHA256(path)` 计算，返回十六进制小写字符串。
- checkpoint 的每个输出 artifact 都记录 `sha256`；恢复时逐一比对，不匹配则判定不可恢复。

## 5. store 路径约定

来源 `internal/store/paths.go`：

- Store 根：`output/aipaper/`。
- 固定文件：`run.json`、`progress.json`、`requirements.json`。
- checkpoint：`checkpoints/latest.json`、`checkpoints/step-NNNNNN.json`（`StepCheckpointName` 左补零 6 位）。
- 必备子目录（`RequiredDirs`）：`checkpoints`、`materials/extracted`、`materials/parsed`、`references`、`outline`、`drafts`、`reviews`、`final`。
- `Rel(parts...)` 生成正斜杠相对路径，用于 checkpoint `outputs[].path`。

## 6. Progress（progress.json）

```go
type Progress struct {
    Phase             string    `json:"phase"`
    CurrentStep       int       `json:"current_step"`
    CurrentChapter    string    `json:"current_chapter,omitempty"`
    CompletedChapters []string  `json:"completed_chapters"`
    PendingChapters   []string  `json:"pending_chapters"`
    Status            string    `json:"status"`
    UpdatedAt         time.Time `json:"updated_at"`
}
```

TUI 仅读取 `progress.json` 与事件流，不解析全部正文。

## 7. Run（run.json）

```go
type Run struct {
    RunID        string         `json:"run_id"`
    CreatedAt    time.Time      `json:"created_at"`
    ResumedFrom  *int           `json:"resumed_from"`
    Provider     string         `json:"provider,omitempty"`
    Model        string         `json:"model,omitempty"`
    CostEstimate map[string]any `json:"cost_estimate"`
    Events       []RunEvent     `json:"events"`
}
```

`ResumedFrom` 为指针：新建运行为 `null`，从 step N 恢复时指向该 step。

## 8. RunEvent

```go
type RunEvent struct {
    At      time.Time      `json:"at"`
    Kind    string         `json:"kind"`
    Message string         `json:"message,omitempty"`
    Fields  map[string]any `json:"fields,omitempty"`
}
```

事件流由 Host 从 agentcore 投影并转发给 TUI，同时可追加进 `run.json` 的 `events`。
