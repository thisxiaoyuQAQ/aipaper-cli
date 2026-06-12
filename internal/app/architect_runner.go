package app

// B3 (BUG-20260613-01 plan B): real Architect LLM runner. The architect_run
// tool dispatches three tasks: outline, evidence_extraction, and
// section_quality_plan. The model only produces content; the Host assigns
// stable ids, validates through the existing quality save paths, persists
// atomically, and records the step checkpoint.

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	aipagent "github.com/thisxiaoyuQAQ/aipaper-cli/internal/agent"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/checkpoint"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/quality"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/store"
)

type ArchitectLLMRunner struct {
	rc *roleChat
}

func NewArchitectRunner(opts RoleRunnerOptions) (*ArchitectLLMRunner, error) {
	rc, err := newRoleChat(aipagent.RoleArchitect, opts)
	if err != nil {
		return nil, err
	}
	return &ArchitectLLMRunner{rc: rc}, nil
}

// RunArchitect implements agent.ArchitectRunner.
func (a *ArchitectLLMRunner) RunArchitect(ctx context.Context, args json.RawMessage) (any, error) {
	var payload struct {
		Task string `json:"task"`
	}
	if err := json.Unmarshal(args, &payload); err != nil {
		return nil, fmt.Errorf("parse architect_run args: %w", err)
	}
	switch payload.Task {
	case "outline":
		return a.runOutline(ctx)
	case aipagent.StepEvidenceExtraction:
		return a.runEvidenceExtraction(ctx)
	case aipagent.StepSectionQualityPlan:
		return a.runSectionQualityPlan(ctx)
	default:
		return nil, fmt.Errorf("unsupported architect task %q (outline|evidence_extraction|section_quality_plan)", payload.Task)
	}
}

// ---------------------------------------------------------------------------
// Task: outline
// ---------------------------------------------------------------------------

type outlineModelOut struct {
	TitleSuggestion string `json:"title_suggestion"`
	Chapters        []struct {
		Title string `json:"title"`
	} `json:"chapters"`
}

func (a *ArchitectLLMRunner) runOutline(ctx context.Context) (any, error) {
	s := a.rc.store
	req, err := loadRequirementsDoc(s)
	if err != nil {
		return nil, err
	}
	confirmed, err := loadConfirmedRefs(s)
	if err != nil {
		return nil, err
	}

	var b strings.Builder
	fmt.Fprintf(&b, "You are the Architect of an academic review on the topic %q.\n", req.Topic)
	if len(req.ResearchQuestions) > 0 {
		fmt.Fprintf(&b, "Research questions: %s\n", strings.Join(req.ResearchQuestions, " | "))
	}
	if req.Scope != "" {
		fmt.Fprintf(&b, "Scope: %s\n", req.Scope)
	}
	if req.TargetWords > 0 {
		fmt.Fprintf(&b, "Target length: about %d words total.\n", req.TargetWords)
	}
	if len(req.ChapterPreferences) > 0 {
		fmt.Fprintf(&b, "Chapter preferences from the user (honor them): %s\n", strings.Join(req.ChapterPreferences, " | "))
	}
	if len(req.Constraints) > 0 {
		fmt.Fprintf(&b, "Constraints: %s\n", strings.Join(req.Constraints, " | "))
	}
	b.WriteString("Confirmed references (the outline must be coverable by these):\n")
	b.WriteString(strings.Join(confirmedRefLines(confirmed), "\n"))
	b.WriteString(`
Design the chapter outline. Keep it small and focused: every chapter must be supportable by the confirmed references.
Return ONLY JSON: {"title_suggestion":"...","chapters":[{"title":"..."}]}`)

	var out outlineModelOut
	if err := a.rc.askJSON(ctx, "", b.String(), &out); err != nil {
		return nil, err
	}
	if len(out.Chapters) == 0 {
		return nil, fmt.Errorf("architect outline has no chapters")
	}

	// The Host assigns chapter ids so every downstream artifact references a
	// stable, path-safe id.
	outline := outlineDoc{TitleSuggestion: strings.TrimSpace(out.TitleSuggestion)}
	var md strings.Builder
	fmt.Fprintf(&md, "# %s\n\n", outline.TitleSuggestion)
	chapterIDs := make([]string, 0, len(out.Chapters))
	for i, ch := range out.Chapters {
		id := fmt.Sprintf("ch%02d", i+1)
		outline.Chapters = append(outline.Chapters, outlineChapter{ChapterID: id, Title: strings.TrimSpace(ch.Title)})
		chapterIDs = append(chapterIDs, id)
		fmt.Fprintf(&md, "## %s %s\n\n", id, strings.TrimSpace(ch.Title))
	}

	jsonRes, err := store.WriteJSON(s.Path("outline", "outline.json"), outline, store.Overwrite)
	if err != nil {
		return nil, err
	}
	mdRes, err := store.WriteFile(s.Path("outline", "outline.md"), []byte(md.String()), store.Overwrite)
	if err != nil {
		return nil, err
	}
	outputs := []checkpoint.OutputArtifact{
		{Kind: "outline", Path: "outline/outline.json", SHA256: jsonRes.SHA256},
		{Kind: "outline_markdown", Path: "outline/outline.md", SHA256: mdRes.SHA256},
	}
	if err := a.rc.recordStepCheckpoint("create_outline", aipagent.StepEvidenceExtraction, outputs, func(p *contracts.Progress) {
		p.PendingChapters = chapterIDs
		p.CurrentChapter = ""
	}); err != nil {
		return nil, err
	}
	for _, id := range chapterIDs {
		a.rc.chapterStatus(id, "pending", nil)
	}
	return map[string]any{
		"title_suggestion": outline.TitleSuggestion,
		"chapters":         outline.Chapters,
		"outputs":          []string{"outline/outline.json", "outline/outline.md"},
	}, nil
}

// ---------------------------------------------------------------------------
// Task: evidence_extraction
// ---------------------------------------------------------------------------

type evidenceModelOut struct {
	Items []struct {
		ReferenceKey string   `json:"reference_key"`
		MaterialID   string   `json:"material_id,omitempty"`
		Depth        string   `json:"depth"`
		Topics       []string `json:"topics"`
		KeyFindings  []string `json:"key_findings"`
		Method       string   `json:"method,omitempty"`
		Subjects     string   `json:"subjects,omitempty"`
		Limitations  []string `json:"limitations,omitempty"`
		Excerpt      string   `json:"excerpt,omitempty"`
		Confidence   string   `json:"confidence"`
	} `json:"items"`
}

func (a *ArchitectLLMRunner) runEvidenceExtraction(ctx context.Context) (any, error) {
	s := a.rc.store
	confirmed, err := loadConfirmedRefs(s)
	if err != nil {
		return nil, err
	}
	if len(confirmed.Items) == 0 {
		return nil, fmt.Errorf("evidence extraction requires confirmed references")
	}
	materials := loadMaterialTexts(s)

	var b strings.Builder
	b.WriteString("You are the Architect distilling an evidence table for an academic review.\n")
	b.WriteString("Confirmed references:\n")
	b.WriteString(strings.Join(confirmedRefLines(confirmed), "\n"))
	b.WriteString("\n\nParsed source material texts (material_id -> text):\n")
	b.WriteString(materialDigest(materials))
	b.WriteString(`
For each confirmed reference produce one or more evidence entries.
Depth honesty (hard rule): use depth "snippet" or "fulltext_excerpt" ONLY when you quote an excerpt taken from the parsed material text above and set material_id; use "abstract" when only the abstract supports the finding; use "metadata_only" otherwise.
Return ONLY JSON: {"items":[{"reference_key":"...","material_id":"material_001 or omit","depth":"metadata_only|abstract|snippet|fulltext_excerpt","topics":["..."],"key_findings":["..."],"method":"...","subjects":"...","limitations":["..."],"excerpt":"verbatim quote for snippet+ depth or omit","confidence":"high|medium|low"}]}
reference_key must be one of the confirmed keys above.`)

	attempt := b.String()
	var savedOutputs []string
	var table quality.EvidenceTable
	for try := 1; ; try++ {
		var out evidenceModelOut
		if err := a.rc.askJSON(ctx, "", attempt, &out); err != nil {
			return nil, err
		}
		table = quality.EvidenceTable{GeneratedAt: a.rc.now()}
		for i, item := range out.Items {
			table.Items = append(table.Items, quality.Evidence{
				ID:           fmt.Sprintf("ev_%03d", i+1),
				ReferenceKey: item.ReferenceKey,
				MaterialID:   item.MaterialID,
				Depth:        item.Depth,
				Topics:       item.Topics,
				KeyFindings:  item.KeyFindings,
				Method:       item.Method,
				Subjects:     item.Subjects,
				Limitations:  item.Limitations,
				Excerpt:      item.Excerpt,
				Confidence:   item.Confidence,
			})
		}
		outputs, err := quality.SaveEvidenceTable(s, table)
		if err == nil {
			savedOutputs = outputs
			break
		}
		if try >= 2 {
			return nil, fmt.Errorf("evidence table validation failed: %w", err)
		}
		attempt = b.String() + "\n\nYour previous evidence table was rejected by validation: " + err.Error() +
			"\nFix the listed problems and reply with ONLY the corrected JSON object."
	}

	outputs, err := artifactsFromPaths(s, "evidence_table", savedOutputs)
	if err != nil {
		return nil, err
	}
	if err := a.rc.recordStepCheckpoint(aipagent.StepEvidenceExtraction, aipagent.StepSectionQualityPlan, outputs, nil); err != nil {
		return nil, err
	}
	return map[string]any{"evidence_count": len(table.Items), "outputs": savedOutputs}, nil
}

// ---------------------------------------------------------------------------
// Task: section_quality_plan
// ---------------------------------------------------------------------------

type planModelOut struct {
	Sections []struct {
		ChapterID                string   `json:"chapter_id"`
		Questions                []string `json:"questions"`
		RequiredEvidenceIDs      []string `json:"required_evidence_ids"`
		RecommendedReferenceKeys []string `json:"recommended_reference_keys,omitempty"`
		Boundaries               []string `json:"boundaries,omitempty"`
		ForbiddenGeneralizations []string `json:"forbidden_generalizations,omitempty"`
		Gaps                     []string `json:"gaps,omitempty"`
		HumanReviewHints         []string `json:"human_review_hints,omitempty"`
	} `json:"sections"`
}

func (a *ArchitectLLMRunner) runSectionQualityPlan(ctx context.Context) (any, error) {
	s := a.rc.store
	outline, err := loadOutlineDoc(s)
	if err != nil {
		return nil, err
	}
	table, err := quality.LoadEvidenceTable(s)
	if err != nil {
		return nil, fmt.Errorf("section quality plan requires the evidence table: %w", err)
	}

	var b strings.Builder
	b.WriteString("You are the Architect producing a per-chapter section quality plan for an academic review.\n")
	b.WriteString("Outline chapters:\n")
	for _, ch := range outline.Chapters {
		fmt.Fprintf(&b, "- %s: %s\n", ch.ChapterID, ch.Title)
	}
	b.WriteString("Evidence table (the ONLY evidence ids you may require):\n")
	for _, ev := range table.Items {
		fmt.Fprintf(&b, "- %s (reference %s, depth=%s): %s\n", ev.ID, ev.ReferenceKey, ev.Depth, strings.Join(ev.KeyFindings, "; "))
	}
	b.WriteString(`
For EVERY outline chapter produce one section entry: 1-3 guiding questions, the evidence ids the chapter must use, boundaries stating what the evidence cannot support, and forbidden generalizations.
Return ONLY JSON: {"sections":[{"chapter_id":"ch01","questions":["..."],"required_evidence_ids":["ev_001"],"recommended_reference_keys":["..."],"boundaries":["..."],"forbidden_generalizations":["..."],"gaps":["..."],"human_review_hints":["..."]}]}
chapter_id must exactly match the outline chapter ids.`)

	attempt := b.String()
	var savedOutputs []string
	var plan quality.SectionQualityPlan
	for try := 1; ; try++ {
		var out planModelOut
		if err := a.rc.askJSON(ctx, "", attempt, &out); err != nil {
			return nil, err
		}
		plan = quality.SectionQualityPlan{GeneratedAt: a.rc.now()}
		for _, sec := range out.Sections {
			plan.Sections = append(plan.Sections, quality.SectionPlan{
				ChapterID:                sec.ChapterID,
				Questions:                sec.Questions,
				RequiredEvidenceIDs:      sec.RequiredEvidenceIDs,
				RecommendedReferenceKeys: sec.RecommendedReferenceKeys,
				Boundaries:               sec.Boundaries,
				ForbiddenGeneralizations: sec.ForbiddenGeneralizations,
				Gaps:                     sec.Gaps,
				HumanReviewHints:         sec.HumanReviewHints,
			})
		}
		outputs, err := quality.SaveSectionQualityPlan(s, plan)
		if err == nil {
			savedOutputs = outputs
			break
		}
		if try >= 2 {
			return nil, fmt.Errorf("section quality plan validation failed: %w", err)
		}
		attempt = b.String() + "\n\nYour previous plan was rejected by validation: " + err.Error() +
			"\nFix the listed problems and reply with ONLY the corrected JSON object."
	}

	outputs, err := artifactsFromPaths(s, "section_quality_plan", savedOutputs)
	if err != nil {
		return nil, err
	}
	if err := a.rc.recordStepCheckpoint(aipagent.StepSectionQualityPlan, "write_chapter", outputs, nil); err != nil {
		return nil, err
	}
	return map[string]any{"sections": len(plan.Sections), "outputs": savedOutputs}, nil
}

func artifactsFromPaths(s store.Store, kind string, paths []string) ([]checkpoint.OutputArtifact, error) {
	outputs := make([]checkpoint.OutputArtifact, 0, len(paths))
	for _, p := range paths {
		sum, err := store.FileSHA256(s.Path(filepath.FromSlash(p)))
		if err != nil {
			return nil, fmt.Errorf("hash %s: %w", p, err)
		}
		outputs = append(outputs, checkpoint.OutputArtifact{Kind: kind, Path: p, SHA256: sum})
	}
	return outputs, nil
}
