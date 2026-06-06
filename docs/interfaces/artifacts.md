# Artifacts 写作产物契约

Architect / Writer / Editor 产出的写作 artifact：大纲、关键论断、引用映射、章节评审，以及章节状态机与版本命名约定。

来源：`internal/contracts/types.go` 的 `ClaimsFile` / `Claim` / `CitationMap` / `CitationMapping` / `Review` / `ReviewScores`；Outline 结构与状态机见 spec 第 3 节。

## 1. Outline（outline.json）

Architect 输出 `outline/outline.json` 与 `outline/outline.md`。spec 第 3 节定义其语义内容（首版无独立 Go 结构体，按下列约定 JSON 落盘）：

```json
{
  "title_suggestion": "生成式 AI 在教育评估中的应用：系统综述",
  "abstract_goal": "概述有效性、公平性与落地挑战",
  "chapters": [
    {
      "chapter_id": "ch01",
      "title": "引言",
      "goal": "界定问题与综述范围",
      "key_questions": ["..."],
      "candidate_reference_keys": ["zhang2024GenerativeAssessment"],
      "target_words": 1200,
      "avoid_overlap": ["与 ch02 的方法学重复"]
    }
  ],
  "chapter_relations": ["ch01→ch02→ch03"]
}
```

包含：论文标题建议、摘要目标、章节列表、每章写作目标、每章关键问题、每章应引用文献候选、章节逻辑关系、字数分配、需避免的重复点。对应 step `create_outline`。

## 2. ClaimsFile / Claim（claims-vN.json）

```go
type ClaimsFile struct {
    ChapterID    string  `json:"chapter_id"`
    DraftVersion int     `json:"draft_version"`
    Claims       []Claim `json:"claims"`
}

type Claim struct {
    ID            string   `json:"id"`
    Text          string   `json:"text"`
    Importance    string   `json:"importance"`
    ReferenceKeys []string `json:"reference_keys"`
    Confidence    float64  `json:"confidence,omitempty"`
}
```

| 字段 | JSON tag | 说明 |
| --- | --- | --- |
| ChapterID | `chapter_id` | 章节 ID，如 `ch01` |
| DraftVersion | `draft_version` | 草稿版本号，从 1 起 |
| Claim.ID | `id` | 论断 ID，如 `ch01_claim_001` |
| Claim.Text | `text` | 论断文本 |
| Claim.Importance | `importance` | 重要性：`high`/`medium`/`low` |
| Claim.ReferenceKeys | `reference_keys` | 支撑文献 key（须为确认文献） |
| Claim.Confidence | `confidence` | 置信度（`omitempty`） |

```json
{
  "chapter_id": "ch01",
  "draft_version": 1,
  "claims": [
    {
      "id": "ch01_claim_001",
      "text": "生成式 AI 可提升自动评分一致性",
      "importance": "high",
      "reference_keys": ["zhang2024GenerativeAssessment"],
      "confidence": 0.82
    }
  ]
}
```

## 3. CitationMap / CitationMapping（citation-map-vN.json）

```go
type CitationMap struct {
    ChapterID    string            `json:"chapter_id"`
    DraftVersion int               `json:"draft_version"`
    Mappings     []CitationMapping `json:"mappings"`
}

type CitationMapping struct {
    ParagraphID   string   `json:"paragraph_id"`
    ClaimIDs      []string `json:"claim_ids"`
    ReferenceKeys []string `json:"reference_keys"`
}
```

将正文段落映射到论断与文献 key：

```json
{
  "chapter_id": "ch01",
  "draft_version": 1,
  "mappings": [
    {
      "paragraph_id": "ch01_p003",
      "claim_ids": ["ch01_claim_001"],
      "reference_keys": ["zhang2024GenerativeAssessment"]
    }
  ]
}
```

## 4. Review / ReviewScores（review-vN.json）

```go
type Review struct {
    ChapterID         string       `json:"chapter_id"`
    DraftVersion      int          `json:"draft_version"`
    Scores            ReviewScores `json:"scores"`
    Passed            bool         `json:"passed"`
    UnsupportedClaims []string     `json:"unsupported_claims"`
    RequiredFixes     []string     `json:"required_fixes"`
    OptionalFixes     []string     `json:"optional_fixes"`
}

type ReviewScores struct {
    Overall             int `json:"overall"`
    CitationConsistency int `json:"citation_consistency"`
    StructureLogic      int `json:"structure_logic"`
    Coverage            int `json:"coverage"`
    Readability         int `json:"readability"`
}
```

| 字段 | JSON tag | 说明 |
| --- | --- | --- |
| Scores.Overall | `overall` | 总分（阈值 ≥ 80） |
| Scores.CitationConsistency | `citation_consistency` | 引用一致性（阈值 ≥ 90） |
| Scores.StructureLogic | `structure_logic` | 结构逻辑 |
| Scores.Coverage | `coverage` | 综述完整度 |
| Scores.Readability | `readability` | 可读性 |
| Passed | `passed` | 是否通过门控 |
| UnsupportedClaims | `unsupported_claims` | 无支撑论断 ID 列表 |
| RequiredFixes | `required_fixes` | 必须修改项 |
| OptionalFixes | `optional_fixes` | 可选优化项 |

```json
{
  "chapter_id": "ch01",
  "draft_version": 1,
  "scores": {
    "overall": 84,
    "citation_consistency": 92,
    "structure_logic": 82,
    "coverage": 80,
    "readability": 85
  },
  "passed": true,
  "unsupported_claims": [],
  "required_fixes": [],
  "optional_fixes": []
}
```

## 5. 章节文件命名（vN 约定）

每章目录 `drafts/chNN/`，按草稿版本编号：

| 文件 | 说明 |
| --- | --- |
| `draft-vN.md` | 第 N 版章节正文（Writer 输出） |
| `claims-vN.json` | 第 N 版关键论断 |
| `citation-map-vN.json` | 第 N 版引用映射 |
| `review-vN.json` | 第 N 版 Editor 评审 |
| `writer_notes.md` | Writer 的 gap / 待补充说明 |
| `accepted.md` | 通过后定稿正文 |

## 6. 章节状态机（spec 第 3 节质量门控）

```text
drafting ──> reviewing ──> accepted ──> committed
                │
                ├─ revision_required ──(重写, ≤2 轮)──> drafting
                └─ needs_human_review（超过重写上限仍未通过）
```

| 状态 | 含义 |
| --- | --- |
| `drafting` | Writer 正在写 |
| `reviewing` | Editor 正在审 |
| `revision_required` | Editor 不通过，触发重写 |
| `accepted` | 章节通过门控 |
| `needs_human_review` | 超过重写上限（2 轮）仍未通过，标记后继续后续章节，不中断流程 |
| `committed` | 章节写入最终稿集合 |
