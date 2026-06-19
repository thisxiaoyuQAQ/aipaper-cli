# 文献候选一行概要设计

日期：2026-06-18

## 背景

用户在文献确认阶段需要快速判断候选文献是否相关。目前 References TUI 和候选文献 Markdown 快照展示标题、作者、年份、来源、DOI/URL、相关度与搜索理由，但没有显示论文摘要。候选数据结构 `ReferenceCandidate` 已包含 `Abstract` 字段，因此可以直接复用现有数据，不新增模型调用，也不伪造缺失信息。

## 目标

- 在找到的文献候选中增加一行概要。
- 概要只来自 `ReferenceCandidate.Abstract`。
- 没有 Abstract 的候选不显示概要行。
- 同时覆盖 TUI 文献确认页面和 candidates Markdown 快照。

## 非目标

- 不为缺少 Abstract 的文献生成概要。
- 不使用 `RelevanceReason` 充当概要。
- 不修改 confirmed references 的结构。
- 不修改最终论文 `final/references.md` 的参考文献条目格式。
- 不引入 LLM 调用或新依赖。

## 展示设计

### References TUI

每条候选文献显示顺序调整为：

```text
> [ ] cand_001 Paper Title
    Author A, Author B | 2024 | semantic_scholar | Journal
    DOI: 10.xxxx/yyyy
    概要: This paper studies ...
    相关度: 0.92
    理由: direct match
```

英文界面使用：

```text
    Summary: This paper studies ...
```

### candidates Markdown 快照

每条候选新增一行：

```markdown
- 概要：This paper studies ...
```

若候选没有 Abstract，则不输出该行。

## 概要格式化规则

新增一个小型格式化 helper，用于 TUI 和 Markdown 快照复用：

1. `strings.TrimSpace` 去掉首尾空白。
2. 将换行、制表符和连续空白压缩为单个空格。
3. 按 rune 截断，默认最多 160 个字符。
4. 超过上限时追加 `...`。
5. 截断逻辑对中文安全，不按 byte 截断。

## 组件改动

### `internal/tui/references/view.go`

- 在候选显示中插入概要行。
- 仅当格式化后的概要非空时显示。
- 使用 i18n key 输出 `概要` / `Summary` 标签。

### `internal/i18n/keys.go` 和 `internal/i18n/messages.go`

新增 key：

- `ReferencesSummary`

中文：`概要`
英文：`Summary`

### `internal/references`

- 找到写出 `references/candidates.md` 的渲染逻辑。
- 复用同一概要格式化 helper，给候选 Markdown 增加概要行。

## 错误处理

- Abstract 为空、全空白或格式化后为空：不显示概要。
- Abstract 很长：截断，不报错。
- Abstract 含换行：压缩为单行，不影响 TUI 布局。

## 测试计划

1. References TUI View 测试：候选含 Abstract 时包含 `概要: ...`。
2. References TUI View 测试：候选无 Abstract 时不输出空概要行。
3. candidates Markdown 测试：候选含 Abstract 时输出 `概要：...`。
4. 格式化 helper 测试：换行压缩、长文本截断、中文不乱码。
5. 运行：
   - `go test ./internal/tui/references ./internal/references ./internal/i18n`
   - `go test ./...`

## 验收标准

- TUI 文献确认页面每条有 Abstract 的候选都显示一行概要。
- `references/candidates.md` 中每条有 Abstract 的候选都显示概要行。
- 无 Abstract 候选不显示概要行。
- 全量测试通过。
