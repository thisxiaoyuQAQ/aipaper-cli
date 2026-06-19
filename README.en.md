# Paper-Cli (English)

`Paper-Cli` is an AI Agent command-line tool for writing academic / literature review surveys. Through an interactive TUI it collects writing requirements, parses user materials, searches academic literature, and confirms citations, then orchestrates multi-role AI Agents to complete an "outline → draft → review → rewrite → export" pipeline, finally producing a traceable, re-verifiable review draft.

The code entry point lives at `cmd/aipaper-cli`, the default config file is `aipaper.json`, and the default output directory is `output/aipaper/`.

[中文文档](README.md)

## Core Features

### Interactive Workflow

- Run with no arguments to enter the TUI, which guides you through configuration, requirements, materials, search, reference confirmation, writing, and export.
- On first launch or when config is missing, the ConfigWizard starts; it supports OpenAI, Anthropic, Ollama, and custom providers.
- WritingProgress supports `Esc` to pause at a safe point and `Enter` to resume; on the next launch it detects a checkpoint and enters RecoverPrompt.

### Materials & Literature

- Parses PDF, Markdown, TXT, and BibTeX materials; DOCX, URL, and CSV follow a degraded parsing path.
- Built-in academic search providers: Semantic Scholar, Crossref, arXiv, and PubMed.
- Candidate references must be confirmed by the user before the AI may cite them; confirmations are written to `references/confirmed.json` and `references/confirmed.bib`.

### Multi-Role AI Collaboration

- `Coordinator`: the master scheduler that drives phase decisions and tool calls.
- `Architect`: generates the outline, evidence table, and section quality plans.
- `Writer`: drafts content per chapter; every claim binds to evidence and a reference key.
- `Editor`: reviews citation consistency, claim support, and readability, then triggers rewrites.

### Quality Assurance

- Quality modes: `fast`, `enhanced` (default), `strict`.
- The Writer Guard validates, before a draft is written, whether a claim binds evidence, whether the evidence exists, and whether the reference key comes from confirmed references.
- Editor verification results are written to the Claim Graph and `quality/verification-result.json`.
- The export stage produces a citation trace, a report, and an optional quality-engine report.

### Quality Engine & Paper Quality Policy

A local paper quality policy (`paper-cli-paper-quality-v1`) is baked into the runtime via `internal/quality/paper_quality_policy.go`. At execution time it is injected into each role's prompt rather than read from external `docs/skills` files:

- `Coordinator`: hard rules and role boundaries (the Host performs machine checks, the Coordinator makes workflow decisions from tool facts, and role agents make semantic judgments within existing JSON contracts).
- `Architect`: the narrative contract (design the paper around one evidence-bounded thesis, not a list of materials), outline dedup, and evidence-depth constraints.
- `Writer`: every important claim must appear in `claims[]` and bind confirmed `evidence_ids`; wording strength matches evidence depth; "insufficient evidence / pending verification / only a framework can be proposed" must never become body text.
- `Verifier`: claim-support judgment (descriptive / comparative / causal / generalization / methodological / limitation / reproducibility); support requires the same object, relation, and scope; only the existing `ClaimVerdict` contract is output.
- `Editor`: distinguish language problems from evidence problems and structural problems; every unsupported / overstated / high-risk claim must get a concrete rewrite instruction with location, problem, instruction, and `suggested_evidence_ids`; when new evidence or domain judgment is needed, mark human review or a gap.

Hard gating cannot be silently bypassed in any mode: unconfirmed citations, claims missing an evidence id, or evidence pointing to a nonexistent or unconfirmed reference key. `quality/verification-result.json` records verifier verdicts; `quality/claim-graph.*` records the claim graph.

## Quick Start

### Prerequisites

- Go 1.25.0 or higher
- At least one LLM provider: OpenAI, Anthropic, a local Ollama service, or an OpenAI-compatible custom service

### Build

```bash
git clone <repo-url>
cd Paper-Cli
go build -o paper-cli ./cmd/aipaper-cli
```

Windows:

```powershell
go build -o paper-cli.exe ./cmd/aipaper-cli
```

You may also name the binary `aipaper-cli` if you prefer to keep the module name.

### Start the TUI

```bash
./paper-cli
```

Windows:

```powershell
.\paper-cli.exe
```

On first launch the ConfigWizard starts; after configuration the main flow runs:

```text
ConfigWizard -> Requirements -> MaterialsScan -> SearchProgress -> References -> WritingProgress -> ExportSummary -> Done
```

If an unfinished checkpoint exists, the launch first enters RecoverPrompt, letting you continue, restart, or quit.

### CLI Commands

```bash
paper-cli init     [--workdir DIR] [--config FILE]
paper-cli status   [--workdir DIR]
paper-cli recover  [--workdir DIR]
paper-cli config   [--workdir DIR] [--config FILE]
```

The `config` command shows the merged config and masks the API key.

## Configuration

ConfigWizard ships with the following templates:

| Provider  | Type          | Default Model      | Base URL                      | API Key                |
| --------- | ------------- | ------------------ | ----------------------------- | ---------------------- |
| OpenAI    | `openai`    | `gpt-5.5`        | `https://api.openai.com/v1` | `OPENAI_API_KEY`     |
| Anthropic | `anthropic` | `claude-opus-4-8` | `https://api.anthropic.com` | `ANTHROPIC_API_KEY`  |
| Ollama    | `ollama`    | `llama3`          | `http://localhost:11434`    | not required           |
| Custom    | custom        | custom             | custom                        | `CUSTOM_LLM_API_KEY` |

Prefer `env:VAR_NAME` to reference environment variables and avoid putting keys in plaintext in `aipaper.json`. The runtime resolves `env:` values; if the variable is absent it returns a clear error.

Example:

```json
{
  "provider": "openai",
  "model": "gpt-5.5",
  "default_language": "zh-CN",
  "citation_style": "gbt7714",
  "providers": {
    "openai": {
      "type": "openai",
      "api_key": "env:OPENAI_API_KEY",
      "base_url": "https://api.openai.com/v1"
    }
  },
  "roles": {
    "coordinator": { "provider": "openai", "max_turns": 12 },
    "architect": { "provider": "openai" },
    "writer": { "provider": "openai" },
    "editor": { "provider": "openai" }
  }
}
```

Config load precedence (later overrides earlier): global `~/.aipaper/config.json`, project `./aipaper.json`, CLI `--config FILE`.

## Writing Requirements

The Requirements form produces a structured `requirements.json`; the main fields:

| Field                   | Description          | Example                        |
| ----------------------- | -------------------- | ------------------------------ |
| `topic`               | Review topic         | `LLMs for code generation`   |
| `research_questions`  | Research questions   | Multi-line input               |
| `scope`               | Review scope         | `Focus on 2022-2024 research` |
| `language`            | Target language      | `zh-CN` or `en`            |
| `citation_style`      | Citation style       | `gbt7714` or `apa`         |
| `quality_mode`        | Quality mode         | `fast`, `enhanced`, `strict` |
| `target_words`        | Target word count    | `8000`                       |
| `material_dir`        | Materials directory  | `./materials`                |
| `allow_online_search` | Allow online search  | `true` or `false`          |
| `search_providers`    | Search sources       | `semantic_scholar, crossref` |

Defaults: `zh-CN`, `gbt7714`, `8000` words, `./materials`, online search enabled, default sources Semantic Scholar and Crossref. An empty `quality_mode` is treated as `enhanced`.

### Quality Modes

| Mode        | Behavior                                                                                                    |
| ----------- | ----------------------------------------------------------------------------------------------------------- |
| `fast`    | Skip per-claim verification; risk items are mostly warnings.                                                |
| `enhanced` | Default mode; verify claim support per claim; unsupported or overstated claims trigger a rewrite.           |
| `strict`   | Stricter gating on top of enhanced; medium risks such as strong claims on shallow evidence are escalated.   |

All modes obey hard gating: unconfirmed citations, claims missing an evidence id, or evidence pointing to a nonexistent or unconfirmed reference key cannot silently pass.

## Workflow

### 1. MaterialsScan

Scans `material_dir` and writes the material manifest and parse artifacts. BibTeX entries are converted into candidate citations, merged with later search results.

Typical output:

```text
materials/manifest.json
materials/extracted/
materials/parsed/
```

### 2. SearchProgress

Queries academic search providers per the requirements and writes candidate references to:

```text
references/candidates.json
```

If search fails, you may skip it and use BibTeX candidates extracted from materials; but at least one reference must be confirmed before writing.

### 3. References

Confirm or reject candidate references in the TUI. Confirmed references are the only trusted source of reference keys for subsequent citation and evidence checks.

```text
references/confirmed.json
references/confirmed.bib
references/rejected.json
```

### 4. WritingProgress

WritingProgress shows run metrics, an event log, a streaming body preview, and chapter status. The core process:

1. Architect generates `outline/outline.json`.
2. In enhanced/strict mode, Architect generates `quality/evidence-table.json`, `quality/evidence-table.md`, `quality/section-quality-plan.json`, `quality/section-quality-plan.md`.
3. Writer writes per chapter: `drafts/{chapter_id}/draft-v{N}.md`, `claims-v{N}.json`, `citation-map-v{N}.json`.
4. The Writer Guard validates claims, evidence, and reference keys before the draft bundle is written.
5. Editor writes `reviews/{chapter_id}/review-v{N}.json`, and `review-v{N}.md` when applicable.
6. After Editor verifies claims, it updates `quality/claim-graph.json`, `quality/claim-graph.md`, and `quality/verification-result.json`.
7. Chapters that pass review are committed to `accepted/{chapter_id}.md`.

### 5. ExportSummary

The export stage produces the final file list, export issues, a quality summary, and items needing human review. DOCX export failure does not block Markdown, references, citation trace, or report output.

## Output File Structure

All run artifacts live under `output/aipaper/` by default.

```text
output/aipaper/
├── run.json
├── progress.json
├── requirements.json
├── materials/
│   ├── manifest.json
│   ├── extracted/
│   └── parsed/
├── references/
│   ├── candidates.json
│   ├── confirmed.json
│   ├── confirmed.bib
│   └── rejected.json
├── outline/
│   └── outline.json
├── drafts/
│   └── {chapter_id}/
│       ├── draft-v1.md
│       ├── claims-v1.json
│       ├── citation-map-v1.json
│       └── writer-notes.md
├── reviews/
│   └── {chapter_id}/
│       ├── review-v1.json
│       └── review-v1.md
├── accepted/
│   └── {chapter_id}.md
├── quality/
│   ├── evidence-table.json
│   ├── evidence-table.md
│   ├── section-quality-plan.json
│   ├── section-quality-plan.md
│   ├── claim-graph.json
│   ├── claim-graph.md
│   └── verification-result.json
├── checkpoints/
│   ├── latest.json
│   └── checkpoint-*.json
└── final/
    ├── paper.md
    ├── paper.docx
    ├── references.md
    ├── citation-trace.json
    ├── report.md
    └── quality-report.md
```

Some quality files are only generated on the enhanced/strict path or when quality artifacts are available; on legacy projects or compatibility mode, `final/quality-report.md` may be absent, but `final/report.md` records a compatibility note.

### `final/quality-report.md`

The quality-engine report, rendered locally from loaded quality artifacts (no LLM call). The header records the `Paper Quality policy` version (`paper-cli-paper-quality-v1`), quality mode, overall status, claim count, and verifier verdict count; the body contains the following sections:

- **Hard Gate Summary** — hard blockers and risk-finding table.
- **Evidence Depth Distribution** — counts for `metadata_only` / `abstract` / `snippet` / `fulltext_excerpt`.
- **Claim Support Summary** — counts for `supported` / `partially_supported` / `unsupported` / `overstated` / `skipped` / `unverified`.
- **Unsupported / Overstated Claims** — each problem claim with its verifier note.
- **Evidence Sufficiency and Content Signals** — insufficient evidence, required-evidence-unused, metadata_only sole support, low content signal.
- **Human Action Items** — actionable human steps (rewrite / soften / add evidence / human review).
- **Needs Human Review** — strict-mode top priority, chapters pending review, and the findings table.
- **Rewrite Summary** — per-chapter rewrite rounds, required/optional instruction counts, and convergence status (`converged` / `needs_revision` / `needs_human_review`).
- **Suggested Next Human Edits** — recommended human edits before external use.

### `final/citation-trace.json`

The citation trace is a flat list; each record describes the relationship between a claim in a paragraph and a confirmed reference:

```json
{
  "version": "export-20260613T103000Z",
  "generated_at": "2026-06-13T10:30:00Z",
  "items": [
    {
      "chapter_id": "ch01",
      "paragraph_id": "p001",
      "claim_id": "claim_001",
      "reference_key": "vaswani2017",
      "source_type": "confirmed_reference",
      "editor_verified": true,
      "needs_human_review": false
    }
  ]
}
```

### `quality/evidence-table.json`

The evidence table records the relationship between evidence and references, materials, topics, findings, evidence depth, and confidence:

```json
{
  "generated_at": "2026-06-13T10:30:00Z",
  "items": [
    {
      "id": "ev_001",
      "reference_key": "vaswani2017",
      "material_id": "material_003",
      "depth": "snippet",
      "topics": ["transformer", "attention"],
      "key_findings": ["Self-attention enables parallel sequence modeling"],
      "excerpt": "The Transformer model architecture relies entirely on attention mechanisms...",
      "confidence": "high"
    }
  ]
}
```

## Checkpoint Recovery

In WritingProgress, pressing `Esc` requests a pause at the nearest safe point; once paused, `Enter` resumes from the latest checkpoint. The bottom input box accepts extra instructions at any time; they are queued and injected into subsequent generation at the nearest safe boundary. `Ctrl+C` quits / confirms exit; do not force-close the terminal — letting the pause or exit flow complete reduces the risk of corrupting progress.

On the next launch, if an unfinished checkpoint is detected, RecoverPrompt appears:

```text
[c] continue  [r] restart  [q] quit
```

You can also verify recovery state via command:

```bash
./paper-cli recover --workdir .
```

This checks whether the artifacts pointed to by `checkpoints/latest.json` exist and hash-match; a non-zero exit code means it is not recoverable.

## Development & Testing

```bash
go build ./...
go test ./...
go test -v ./internal/e2e
```

The real-LLM smoke tool lives at `tools/real-tui-smoke/`; it injects keys via `SMOKE_API_KEY`, `SMOKE_BASE_URL`, `SMOKE_MODEL`, etc., and never writes real keys to config files.

Common modules:

| Path | Responsibility |
|---|---|
| `cmd/aipaper-cli/` | CLI/TUI entry |
| `internal/cli/` | `init`, `status`, `recover`, `config` commands |
| `internal/tui/` | ConfigWizard, Requirements, MaterialsScan, SearchProgress, References, WritingProgress, etc. |
| `internal/app/` | Bootstrap, AgentRuntime, role runners |
| `internal/config/` | config load, merge, validate, mask |
| `internal/store/` | store paths, layout, atomic writes, hashing |
| `internal/materials/` | material scan and parse |
| `internal/search/` | academic search providers |
| `internal/references/` | candidate, confirmed, rejected reference management |
| `internal/agent/` | Coordinator, role tools, quality policy |
| `internal/quality/` | Evidence, Section Plan, Claim Graph, Verification, Gate |
| `internal/artifacts/` | draft, claim, citation map, review, accepted-chapter writes |
| `internal/export/` | Markdown, DOCX, citation trace, report export |

## FAQ

### API Key authentication failed

1. Confirm the env var is set, e.g. `echo $OPENAI_API_KEY`.
2. Confirm the config uses the correct `env:VAR_NAME`.
3. Run `./paper-cli config` to view the effective config; the key is masked.

### Empty materials directory

Put PDF, Markdown, TXT, or BibTeX files into `materials/` and rescan. You may also skip material scanning and rely on academic search or BibTeX candidates.

### Academic search failed

Check the network and provider availability. If search fails you may skip it, but the References stage still needs at least one confirmed reference to enter writing.

### Context too long

Reduce the number or size of materials, lower the target word count, or assign Writer/Editor a model with a larger context window in `roles`.

### Word document export failed

DOCX uses a basic exporter. On failure, `final/paper.md`, `final/references.md`, `final/citation-trace.json`, and `final/report.md` are still available; convert from Markdown with Pandoc or similar.

### Windows: window closes on double-click

Run `paper-cli.exe` in PowerShell or CMD to see the error, e.g. missing config, env var, or local service unavailable.

## Project Boundaries

The current version does not promise:

- A submission-ready final paper; human review and polishing are still required.
- Perfect handling of all GB/T 7714 and APA formatting details; DOCX is a basic format.
- OCR of scanned PDFs or deep understanding of images, tables, or formulas.
- A web UI, multi-user backend, or cloud sync.
- Automatic endorsement of online references' authenticity; references still need user confirmation.
- Guaranteed correctness of all academic claims; quality gating checks the evidence chain and citation consistency, not a substitute for human academic judgment.

## Documentation Index

- [User Guide (detailed)](docs/user-guide.en.md): full install, configuration, material prep, generation, export, and recovery notes.
- [Paper Quality Skill runtime design](docs/superpowers/specs/2026-06-20-paper-quality-skill-runtime-design.md): local policy injection and quality-engine design.
- [Quality interface contracts](docs/interfaces/quality.md): Evidence, Section Plan, Claim Graph, Verification, Gate contracts.
- [Interface contract index](docs/interfaces/_index.md): JSON schemas and contract definitions.

## License

This project is open-sourced under the [MIT License](LICENSE). Anyone may freely use, copy, modify, merge, publish, distribute, sublicense, or sell the software, provided the copyright notice and this license notice are retained in all copies.

Copyright © 2026 thisxiaoyuQAQ.

## Acknowledgements

This project uses Bubble Tea, agentcore, and the Semantic Scholar, Crossref, arXiv, and PubMed academic retrieval services.
