# Checkpoint 与恢复契约

Step 级 checkpoint：每个工具成功完成后记录可恢复状态。崩溃、断网、Ctrl+C 后同目录再次启动即可恢复。

来源：`internal/checkpoint/checkpoint.go` 的 `Checkpoint` / `OutputArtifact` / `Validation`；step 列表与一致性见 spec 第 5 节。

## 1. Checkpoint 结构

```go
type Checkpoint struct {
    Step         int              `json:"step"`
    Phase        string           `json:"phase"`
    Status       string           `json:"status"`
    CreatedAt    time.Time        `json:"created_at"`
    Input        map[string]any   `json:"input,omitempty"`
    Outputs      []OutputArtifact `json:"outputs"`
    StatePatch   map[string]any   `json:"state_patch,omitempty"`
    NextExpected string           `json:"next_expected,omitempty"`
}
```

| 字段 | JSON tag | 说明 |
| --- | --- | --- |
| Step | `step` | 步号，须 > 0（`Record` 校验） |
| Phase | `phase` | 阶段名，必填非空 |
| Status | `status` | 状态，缺省补 `success` |
| CreatedAt | `created_at` | 创建时间，缺省补 `time.Now().UTC()` |
| Input | `input` | 本 step 输入摘要（`omitempty`） |
| Outputs | `outputs` | 输出 artifact 列表 |
| StatePatch | `state_patch` | 状态增量补丁（`omitempty`） |
| NextExpected | `next_expected` | 下一预期 step（`omitempty`） |

## 2. OutputArtifact

```go
type OutputArtifact struct {
    Kind   string `json:"kind"`
    Path   string `json:"path"`
    SHA256 string `json:"sha256"`
}
```

| 字段 | 说明 |
| --- | --- |
| Kind | artifact 类型，如 `review`、`draft`、`outline` |
| Path | **相对 Store 根、正斜杠** 路径，如 `drafts/ch03/review-v2.json`（绝对路径或逃逸根目录会校验失败） |
| SHA256 | 文件内容哈希，恢复时比对 |

## 3. Validation（校验结果）

```go
type Validation struct {
    OK      bool     `json:"ok"`
    Step    int      `json:"step,omitempty"`
    Phase   string   `json:"phase,omitempty"`
    Next    string   `json:"next_expected,omitempty"`
    Errors  []string `json:"errors,omitempty"`
    Checked []string `json:"checked,omitempty"`
}
```

`ValidateLatest` 逐一检查 `latest.json` 的每个 output：路径须相对、不得逃逸 Store 根、文件存在、SHA256 匹配；任一不满足 `OK=false` 并追加 `Errors`，通过的路径记入 `Checked`。

## 4. checkpoint.json 示例

```json
{
  "step": 42,
  "phase": "review_chapter",
  "status": "success",
  "created_at": "2026-06-05T10:00:00Z",
  "input": { "chapter_id": "ch03", "draft_version": 2 },
  "outputs": [
    { "kind": "review", "path": "drafts/ch03/review-v2.json", "sha256": "..." }
  ],
  "state_patch": { "current_chapter": "ch03", "draft_version": 2, "review_passed": true },
  "next_expected": "commit_chapter"
}
```

## 5. Step 列表（spec 第 5 节）

每个可恢复动作即一个 step：

`collect_requirements` → `parse_materials` → `search_references` → `confirm_references` → `create_outline` → `draft_chapter` → `review_chapter` → `revise_chapter` → `commit_chapter` → `final_review` → `export_docx`

（章节循环中 `draft_chapter`/`review_chapter`/`revise_chapter`/`commit_chapter` 按章节与版本重复出现。）

## 6. 崩溃一致性写入顺序（spec 第 5 节）

`Record` 落盘顺序，确保任意点崩溃可恢复：

1. 写 artifact 临时文件；
2. fsync artifact；
3. rename 为正式文件；
4. 写 step checkpoint（`checkpoints/step-NNNNNN.json`，`CreateOnly`）；
5. 更新 `checkpoints/latest.json`（`Overwrite`）；
6. 更新 `progress.json`（`Overwrite`）；
7. 发送事件给 TUI。

> 代码中 `Record` 先 `EnsureLayout`，再依次写 step（CreateOnly）→ latest（Overwrite）→ progress（Overwrite）。artifact 由各工具在调用 `Record` 之前用 `WriteFile`（temp+fsync+rename）写好。

## 7. 幂等规则（spec 第 5 节）

- step checkpoint 用 `CreateOnly` 写入：同步号且内容相同 → `AlreadyExists`；内容不同 → 冲突错误 `STORE_CHECKPOINT_CONFLICT`。
- 同一 `chapter_id + draft_version` 不允许无提示覆盖；目标 artifact 已存在且哈希匹配 → 返回 `already_exists`；存在但内容不匹配 → 冲突错误。
- 搜索/候选文献按 DOI/URL/title hash 去重。
- `commit_chapter` 只接受已通过 review 的版本。
- `export` 可重复执行，但记录导出版本。

## 8. 恢复流程（spec 第 5 节）

1. Host 检查 `progress.json` 与 `checkpoints/latest.json`。
2. `ValidateLatest` 校验 artifact 存在与哈希匹配。
3. 一致则生成恢复 prompt：已完成什么、当前阶段、下一预期 step、可读 artifact、不可重复操作。
4. Coordinator 从恢复 prompt 与 Store 继续；`run.json` 的 `resumed_from` 指向恢复点 step。
5. TUI 展示「已从 step N 恢复」。
