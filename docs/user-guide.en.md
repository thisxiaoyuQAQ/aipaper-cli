# Paper-Cli User Guide (English)

## 1. Product Overview

`Paper-Cli` is an AI Agent command-line tool for writing academic / literature review surveys. Through an interactive TUI it collects writing requirements, parses user materials, searches academic literature, and confirms citations, then orchestrates the Coordinator, Architect, Writer, and Editor roles to complete an "outline → draft → review → rewrite → export" pipeline.

The code entry point lives at `cmd/aipaper-cli`; the config file is `aipaper.json` by default; run artifacts are written to `output/aipaper/` by default.

Core capabilities:

- Interactive TUI: run with no arguments to enter the full guided flow.
- Multi-source material parsing: PDF, Markdown, TXT, BibTeX supported; DOCX, URL, CSV follow a degraded path.
- Academic search: Semantic Scholar, Crossref, arXiv, PubMed built in.
- Human reference confirmation: candidate references must be confirmed by the user before they may be cited.
- Multi-role writing: the Coordinator schedules Architect, Writer, and Editor.
- Quality gating: claims, evidence, reference keys, and editor verdicts are checked in a chain.
- Checkpoint recovery: WritingProgress can pause at a safe point with `Esc` and resume from a checkpoint.
- Structured export: Markdown, DOCX, references, citation trace, run report, and quality report.

## 2. Installation & Launch

### Requirements

- Go 1.25.0 or higher
- At least one LLM provider's API key, or a reachable Ollama local service

### Build

```bash
git clone <repo-url>
cd Paper-Cli
go build -o paper-cli ./cmd/aipaper-cli
```

Windows:

```powershell
go build -o paper-cli.exe ./cmd/aipaper-cli
go build -o aipaper-cli.exe ./cmd/aipaper-cli
```

To keep the module name as the binary, you may also build `aipaper-cli`:

```bash
go build -o aipaper-cli ./cmd/aipaper-cli
```

### Start the TUI

```bash
./paper-cli
```

Windows:

```powershell
.\paper-cli.exe
```

Running with no arguments enters the TUI. On first launch or when config is missing, the ConfigWizard starts first; if an unfinished checkpoint exists, RecoverPrompt starts first.

### CLI Commands

```text
paper-cli init     [--workdir DIR] [--config FILE]   initialize the working directory
paper-cli status   [--workdir DIR]                    show current status
paper-cli recover  [--workdir DIR]                    verify checkpoint recoverability
paper-cli config   [--workdir DIR] [--config FILE]    show merged config (API key masked)
```

## 3. First-Time LLM Configuration

On first TUI launch, if no valid config is found, the ConfigWizard starts automatically.

### Choose a Provider Template

| Provider | Type | Default Model | Base URL | API Key |
|---|---|---|---|---|
| OpenAI | `openai` | `gpt-5.5` | `https://api.openai.com/v1` | `OPENAI_API_KEY` |
| Anthropic | `anthropic` | `claude-opus-4-8` | `https://api.anthropic.com` | `ANTHROPIC_API_KEY` |
| Ollama | `ollama` | `llama3` | `http://localhost:11434` | not required |
| Custom | custom | custom | custom | `CUSTOM_LLM_API_KEY` |

Ollama users must ensure the local service is running first. The Custom template suits OpenAI-compatible APIs or new provider types added within the project.

### API Key Safety

Prefer `env:VAR_NAME` in the config:

```json
{
  "api_key": "env:OPENAI_API_KEY"
}
```

The runtime reads the corresponding environment variable. If it is missing or empty, the program returns a clear error. The `config` command, config summary, and logs mask the key.

### Manual Config Example

You can manually create a project-level `aipaper.json`:

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

Config load precedence (later overrides earlier):

1. Global config: `~/.aipaper/config.json`
2. Project config: `./aipaper.json`
3. CLI-specified: `--config FILE`

## 4. Prepare Materials

Create a `materials/` folder under the project directory and put reference materials in it:

```text
materials/
  survey-on-llm.pdf
  notes.md
  related-work.txt
  references.bib
```

Supported formats:

| Format | Support Level | Notes |
|---|---|---|
| PDF | Full | Extract text and metadata |
| Markdown | Full | Preserve body and heading structure |
| TXT | Full | Treated as plain text |
| BibTeX | Full | Auto-extract candidate references |
| DOCX | Degraded | Basic text extraction |
| URL | Degraded | Depends on network availability |
| CSV | Degraded | Parsed line by line |

MaterialsScan automatically scans `material_dir`. If the directory does not exist, the TUI creates an empty one and prompts you to add materials. A single file's parse failure does not block other files.

## 5. Generate the Paper

The TUI main flow:

```text
ConfigWizard -> Requirements -> MaterialsScan -> SearchProgress -> References -> WritingProgress -> ExportSummary -> Done
```

In recovery scenarios, RecoverPrompt appears before the main flow.

### Requirements

The Requirements form collects topic, research questions, review scope, target language, citation style, quality mode, target word count, materials directory, and search settings.

Common fields:

| Field | Description | Values / Example |
|---|---|---|
| `topic` | Review topic | `LLMs for code generation` |
| `research_questions` | Research questions | Multi-line input |
| `scope` | Scope constraints | `Focus on 2022-2024 research` |
| `language` | Writing language | `zh-CN`, `en` |
| `citation_style` | Citation style | `gbt7714`, `apa` |
| `quality_mode` | Quality mode | `fast`, `enhanced`, `strict` |
| `target_words` | Target word count | `8000` |
| `material_dir` | Materials directory | `./materials` |
| `allow_online_search` | Allow online search | `true`, `false` |
| `search_providers` | Search providers | `semantic_scholar, crossref` |

Defaults: `zh-CN`, `gbt7714`, `8000`, `./materials`, online search enabled, default sources Semantic Scholar and Crossref. An empty `quality_mode` is treated as `enhanced`.

#### Quality Modes

| Mode | Behavior |
|---|---|
| `fast` | Skip per-claim verification; risk items are mostly warnings. |
| `enhanced` | Default mode; verify claim support per claim; unsupported or overstated claims trigger a rewrite. |
| `strict` | Stricter gating on top of enhanced; medium risks may also be escalated to needs-revision. |

In any mode, bottom-line issues are hard-blocked: citing unconfirmed references, claims without an evidence id, or evidence/claims pointing to a nonexistent reference key.

#### Paper Quality Policy

A local paper quality policy (`paper-cli-paper-quality-v1`, see `internal/quality/paper_quality_policy.go`) is injected into each role's prompt at execution time, rather than read from external `docs/skills` files at runtime:

- `Coordinator`: hard rules and role boundaries (the Host performs machine checks, the Coordinator makes workflow decisions from tool facts, and role agents make semantic judgments within existing JSON contracts).
- `Architect`: design the paper around one evidence-bounded thesis rather than listing materials; outline dedup; evidence-depth constraints.
- `Writer`: every important claim must appear in `claims[]` and bind confirmed `evidence_ids`; wording strength matches evidence depth; "insufficient evidence / pending verification / only a framework can be proposed" must never become body text.
- `Verifier`: claim-support judgment (support requires the same object, relation, and scope); only the existing `ClaimVerdict` contract is output.
- `Editor`: distinguish language, evidence, and structural problems; every unsupported / overstated / high-risk claim must get a rewrite instruction with location and `suggested_evidence_ids`; when new evidence or domain judgment is needed, mark human review or a gap.

### MaterialsScan

MaterialsScan scans the materials directory and writes:

```text
materials/manifest.json
materials/extracted/
materials/parsed/
```

BibTeX entries are converted into candidate citations, merged later with online search results.

### SearchProgress

Performs academic search per the writing requirements. Default needs enable Semantic Scholar and Crossref; the project also includes arXiv and PubMed providers.

Search results are written to:

```text
references/candidates.json
```

If search fails, you may skip it and continue with BibTeX candidates; but at least one reference must be confirmed before the writing stage.

### References

Candidate references are shown in the TUI, where you can search, confirm, or reject. Confirmed reference keys are the trusted source for subsequent citation, evidence, and quality checks.

Output:

```text
references/confirmed.json
references/confirmed.bib
references/rejected.json
```

### WritingProgress

WritingProgress shows run metrics, an event log, a streaming content preview, and chapter status.

Writing flow:

1. The Coordinator decides the next tool call.
2. Architect generates `outline/outline.json`.
3. In enhanced/strict mode, Architect generates the Evidence Table and Section Quality Plan.
4. Writer generates per-chapter drafts, claims, and citation map.
5. The Writer Guard validates evidence and reference keys before writing.
6. Editor reviews the chapter and writes a review.
7. After Editor verifies claims, it updates the Claim Graph and Verification Result.
8. Chapters that do not pass enter a rewrite loop, up to a limited number of rounds; passing chapters are committed as accepted chapters.
9. After all chapters finish, ExportSummary starts.

Main intermediate artifacts:

```text
outline/outline.json
quality/evidence-table.json
quality/evidence-table.md
quality/section-quality-plan.json
quality/section-quality-plan.md
drafts/{chapter_id}/draft-v{N}.md
drafts/{chapter_id}/claims-v{N}.json
drafts/{chapter_id}/citation-map-v{N}.json
reviews/{chapter_id}/review-v{N}.json
reviews/{chapter_id}/review-v{N}.md
quality/claim-graph.json
quality/claim-graph.md
quality/verification-result.json
accepted/{chapter_id}.md
```

### ExportSummary

The export stage shows the final files, export issues, quality summary, and items needing human review. DOCX failure does not block other final artifacts.

### Done

The done page shows the output directory and follow-up suggestions. Human review of `final/report.md`, `final/quality-report.md`, and `final/citation-trace.json` is recommended before submission or publication.

## 6. Output Files

All output files live under `output/aipaper/` by default.

### Top-Level State Files

| File | Description |
|---|---|
| `run.json` | Run metadata, provider, model, cost estimate, etc. |
| `progress.json` | Current phase, chapter status, update time. |
| `requirements.json` | Structured writing requirements. |

### Final Artifacts

| File | Description |
|---|---|
| `final/paper.md` | Markdown review draft. |
| `final/paper.docx` | Word document, basic OOXML format. |
| `final/references.md` | Reference list. |
| `final/citation-trace.json` | Citation trace linking chapter, paragraph, claim, and reference key. |
| `final/report.md` | Export report: outputs, issues, quality summary, compatibility notes. |
| `final/quality-report.md` | Quality-engine report; only generated when quality artifacts are available. |

`final/quality-report.md` is rendered locally (no LLM call). Its header records the `Paper Quality policy` version (`paper-cli-paper-quality-v1`), quality mode, overall status, claim count, and verifier verdict count; the body contains: Hard Gate Summary (hard blockers and risk findings), Evidence Depth Distribution, Claim Support Summary (support counts), Unsupported / Overstated Claims, Evidence Sufficiency and Content Signals, Human Action Items, Needs Human Review (strict top priority + chapters pending review), Rewrite Summary (per-chapter rewrite rounds and convergence status), Suggested Next Human Edits.

`final/citation-trace.json` example:

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

### Materials & References

| Path | Description |
|---|---|
| `materials/manifest.json` | Material scan manifest. |
| `materials/extracted/` | Extracted text. |
| `materials/parsed/` | Structured parse results. |
| `references/candidates.json` | Candidate references. |
| `references/confirmed.json` | Confirmed references. |
| `references/confirmed.bib` | Confirmed references BibTeX. |
| `references/rejected.json` | Rejected references. |

### Drafts, Reviews, and Quality Artifacts

| Path | Description |
|---|---|
| `outline/outline.json` | Architect-generated outline. |
| `drafts/{chapter_id}/draft-v{N}.md` | Writer's Nth chapter draft. |
| `drafts/{chapter_id}/claims-v{N}.json` | Chapter claim list. |
| `drafts/{chapter_id}/citation-map-v{N}.json` | Mapping of paragraph, claim, and reference key. |
| `drafts/{chapter_id}/writer-notes.md` | Writer notes; only when notes exist. |
| `reviews/{chapter_id}/review-v{N}.json` | Editor review result. |
| `reviews/{chapter_id}/review-v{N}.md` | Editor review Markdown; only when content exists. |
| `accepted/{chapter_id}.md` | Accepted chapter body. |
| `quality/evidence-table.json` | Evidence Table. |
| `quality/evidence-table.md` | Evidence Table Markdown view. |
| `quality/section-quality-plan.json` | Section Quality Plan. |
| `quality/section-quality-plan.md` | Section Quality Plan Markdown view. |
| `quality/claim-graph.json` | Claim Graph. |
| `quality/claim-graph.md` | Claim Graph Markdown view. |
| `quality/verification-result.json` | Editor verifier's claim verdicts. |

`quality/evidence-table.json` example:

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

## 7. Interruption & Recovery

### Safe Pause & Exit

In the WritingProgress view:

1. Press `Esc` to request a pause at the nearest safe point.
2. The system waits for the current safe boundary / checkpoint to finish, then enters Paused.
3. In Paused, press `Enter` to resume generation.
4. The bottom input box accepts extra instructions; they are queued and injected at the nearest safe boundary.
5. `Ctrl+C` quits / confirms exit.

Do not force-close the terminal window; letting the pause or exit flow complete reduces checkpoint corruption risk.

### Resume a Run

On the next TUI launch, if an unfinished checkpoint is detected, RecoverPrompt appears:

```text
[c] continue  [r] restart  [q] quit
```

Options:

- Continue: resume from the checkpoint and enter WritingProgress.
- Restart: after a second confirmation, return to an earlier step; existing files are not deleted proactively.
- Quit: end the TUI.

### CLI Verification

```bash
./paper-cli recover --workdir .
```

This checks whether the artifacts pointed to by `checkpoints/latest.json` are complete and hash-match. A non-zero exit code means not recoverable.

## 8. FAQ

### API Key errors

Symptom: WritingProgress reports authentication or a missing environment variable.

Fix:

1. Check the env var, e.g. `echo $OPENAI_API_KEY`.
2. Check that `api_key` in `aipaper.json` uses the correct `env:VAR_NAME`.
3. Run `./paper-cli config` to view the effective config; the key is masked.

### Ollama unavailable

Symptom: local model calls fail or connecting to `http://localhost:11434` fails.

Fix:

1. Confirm the Ollama service is running.
2. Confirm the base URL in config matches the local service.
3. Confirm the selected model has been pulled locally.

### materials empty

Symptom: MaterialsScan reports no files found.

Fix: Put PDF, Markdown, TXT, or BibTeX into `materials/` and rescan. You may also skip material scanning and rely on online search or existing candidates.

### Academic search failed

Symptom: all search providers fail in SearchProgress.

Fix: Check the network, proxy, and target service availability; you may also skip search and continue with BibTeX candidates. At least one reference must still be confirmed before writing.

### Context too long

Symptom: the writing stage reports context length exceeded.

Fix: Reduce the number or size of materials, lower the target word count, or assign Writer/Editor a model with a larger context window in `roles`.

### Quality gating too strict

Symptom: chapters are repeatedly rewritten or marked as needing revision.

Fix: Prioritize `final/quality-report.md` or `quality/claim-graph.md` for unsupported, overstated, partially_supported items. When evidence is genuinely insufficient, add materials or adjust wording; for a quick draft, set `quality_mode` to `fast` and rerun.

### DOCX export failed

Symptom: ExportSummary reports Word document generation failed.

Fix: This does not affect other files. `final/paper.md`, `final/references.md`, `final/citation-trace.json`, and `final/report.md` are still available. For complex formatting, convert from Markdown with Pandoc or similar.

### Windows: window closes on double-click

Fix: Run the executable in PowerShell or CMD to see the error:

```powershell
cd path\to\Paper-Cli
.\paper-cli.exe
```

## 9. Advanced Commands

### init

```bash
paper-cli init --workdir .
paper-cli init --workdir . --config ./aipaper.json
```

Initializes the Store layout and creates the required directories and state files under `output/aipaper/`.

### status

```bash
paper-cli status --workdir .
```

Outputs the current phase, step, chapter progress, and update time. When uninitialized, it reports the uninitialized state.

### recover

```bash
paper-cli recover --workdir .
```

Validates the checkpoint and artifact hashes to decide whether recovery is possible.

### config

```bash
paper-cli config --workdir . --config ./aipaper.json
```

Outputs the list of loaded config files and the merged config object. The API key is shown masked.

## 10. Development & Testing

Common checks:

```bash
go build ./...
go test ./...
go test -v ./internal/e2e
```

Real-LLM smoke tool:

```bash
go run ./tools/real-tui-smoke
```

It uses `SMOKE_API_KEY`, `SMOKE_BASE_URL`, `SMOKE_MODEL`, etc., and writes the key in config as `env:SMOKE_API_KEY` to avoid persisting real keys.

## 11. Environment Variable Reference

Common variables:

| Variable | Description |
|---|---|
| `OPENAI_API_KEY` | OpenAI API key |
| `ANTHROPIC_API_KEY` | Anthropic API key |
| `CUSTOM_LLM_API_KEY` | Custom provider API key |
| `SMOKE_API_KEY` | Real-LLM smoke test API key |
| `SMOKE_BASE_URL` | Real-LLM smoke test base URL |
| `SMOKE_MODEL` | Real-LLM smoke test model |

If the project ships a `.env.example`, use the variable list there as the source of truth.
