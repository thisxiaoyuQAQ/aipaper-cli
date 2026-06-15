package agent

// Module 25 boundary additions: writer evidence protocol. The writer_run tool
// input gains the chapter quality plan plus its required evidence, and a host
// guard hard-validates claims before chapter artifacts are persisted. Existing
// decision and tool semantics in coordinator.go / tools.go are not changed.

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/artifacts"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/quality"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/store"
)

// Structured error codes for the writer evidence guard.
const (
	CodeClaimMissingEvidence      = "WRITER_CLAIM_MISSING_EVIDENCE"
	CodeClaimUnknownEvidence      = "WRITER_CLAIM_UNKNOWN_EVIDENCE"
	CodeClaimUnconfirmedReference = "WRITER_CLAIM_UNCONFIRMED_REFERENCE"
)

// WriterChapterInput is the per-chapter input handed to the writer runner.
// QualityPlan and Evidence come from the quality artifacts written by the
// Architect steps; Warnings records compatibility-mode gaps (old runs without
// quality artifacts keep running instead of being blocked).
type WriterChapterInput struct {
	ContractVersion        string               `json:"contract_version,omitempty"`
	ChapterID              string               `json:"chapter_id"`
	ArticleTemplate        string               `json:"article_template,omitempty"`
	CitationStyle          string               `json:"citation_style,omitempty"`
	TemplateGuidance       []string             `json:"template_guidance,omitempty"`
	ForbiddenDraftPatterns []string             `json:"forbidden_draft_patterns,omitempty"`
	MinEvidencePerChapter  int                  `json:"min_evidence_per_chapter,omitempty"`
	QualityPlan            *quality.SectionPlan `json:"quality_plan,omitempty"`
	Evidence               []quality.Evidence   `json:"evidence,omitempty"`
	// RewriteInstructions carry the previous review round's structured editor
	// directives so a rewrite addresses each one (module 28).
	RewriteInstructions []contracts.RewriteInstruction `json:"rewrite_instructions,omitempty"`
	Warnings            []string                       `json:"warnings,omitempty"`
}

// BuildWriterChapterInput resolves the chapter quality plan and the contents
// of its required evidence. Missing quality artifacts degrade to warnings so
// recovering old runs is never blocked; unreadable artifacts are hard errors.
func BuildWriterChapterInput(s store.Store, raw json.RawMessage) (WriterChapterInput, error) {
	var args struct {
		ChapterID string `json:"chapter_id"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return WriterChapterInput{}, AgentError(CodeInvalidJSON, fmt.Sprintf("parse writer_run args: %v", err), false)
	}
	input := WriterChapterInput{ChapterID: args.ChapterID}
	attachWriterPolicy(s, &input)
	if err := attachRewriteInstructions(s, &input); err != nil {
		return WriterChapterInput{}, err
	}

	plan, ok, err := loadSectionQualityPlanIfPresent(s)
	if err != nil {
		return WriterChapterInput{}, err
	}
	if !ok {
		input.Warnings = append(input.Warnings, "section quality plan is missing; writing in compatibility mode without a chapter quality plan")
		return input, nil
	}
	var chapterPlan *quality.SectionPlan
	for i := range plan.Sections {
		if plan.Sections[i].ChapterID == args.ChapterID {
			chapterPlan = &plan.Sections[i]
			break
		}
	}
	if chapterPlan == nil {
		input.Warnings = append(input.Warnings, fmt.Sprintf("section quality plan has no entry for chapter %q", args.ChapterID))
		return input, nil
	}
	input.QualityPlan = chapterPlan

	table, ok, err := loadEvidenceTableIfPresent(s)
	if err != nil {
		return WriterChapterInput{}, err
	}
	byID := map[string]quality.Evidence{}
	if ok {
		for _, ev := range table.Items {
			byID[ev.ID] = ev
		}
	}
	for _, id := range chapterPlan.RequiredEvidenceIDs {
		ev, found := byID[id]
		if !found {
			input.Warnings = append(input.Warnings, fmt.Sprintf("required evidence %s is not in the evidence table", id))
			continue
		}
		input.Evidence = append(input.Evidence, ev)
	}
	return input, nil
}

func attachWriterPolicy(s store.Store, input *WriterChapterInput) {
	input.ContractVersion = "paper-standard-v1"
	var req contracts.Requirements
	if err := store.ReadJSON(s.RequirementsPath(), &req); err != nil {
		input.ArticleTemplate = quality.ArticleTemplateZhCoursePaper
		input.CitationStyle = "gbt7714"
		input.Warnings = append(input.Warnings, "requirements are unavailable; using default paper template and GB/T 7714 citation policy")
	} else {
		template := quality.ResolveArticleTemplate(req.ArticleTemplate)
		input.ArticleTemplate = template.ID
		input.CitationStyle = req.CitationStyle
		if input.CitationStyle == "" {
			input.CitationStyle = template.DefaultCitationStyle
		}
		input.TemplateGuidance = append([]string(nil), template.WriterGuidance...)
		input.ForbiddenDraftPatterns = append([]string(nil), template.ForbiddenDraftPatterns...)
		input.MinEvidencePerChapter = template.MinEvidencePerChapter
		return
	}
	template := quality.ResolveArticleTemplate(input.ArticleTemplate)
	input.TemplateGuidance = append([]string(nil), template.WriterGuidance...)
	input.ForbiddenDraftPatterns = append([]string(nil), template.ForbiddenDraftPatterns...)
	input.MinEvidencePerChapter = template.MinEvidencePerChapter
}

// GuardWriterClaims hard-validates writer claims before chapter artifacts are
// written: every claim must bind at least one evidence id that exists in the
// evidence table, and may only cite confirmed reference keys (existing rule).
func GuardWriterClaims(s store.Store, claims contracts.ClaimsFile) error {
	confirmed, err := readConfirmedReferences(s)
	if err != nil {
		return AgentError(CodeToolExecutionFailed, fmt.Sprintf("read confirmed references: %v", err), false)
	}
	confirmedKeys := make(map[string]bool, len(confirmed.Items))
	for _, item := range confirmed.Items {
		confirmedKeys[item.Key] = true
	}
	table, _, err := loadEvidenceTableIfPresent(s)
	if err != nil {
		return err
	}
	evidenceByID := make(map[string]quality.Evidence, len(table.Items))
	for _, ev := range table.Items {
		evidenceByID[ev.ID] = ev
	}
	for _, claim := range claims.Claims {
		if len(claim.EvidenceIDs) == 0 {
			return writerGuardError(CodeClaimMissingEvidence,
				fmt.Sprintf("claim %q must bind at least one evidence id", claim.ID), claims.ChapterID, claim.ID)
		}
		for _, key := range claim.ReferenceKeys {
			if !confirmedKeys[key] {
				return writerGuardError(CodeClaimUnconfirmedReference,
					fmt.Sprintf("claim %q cites reference key %q that is not in references/confirmed.json", claim.ID, key), claims.ChapterID, claim.ID)
			}
		}
		for _, id := range claim.EvidenceIDs {
			ev, ok := evidenceByID[id]
			if !ok {
				return writerGuardError(CodeClaimUnknownEvidence,
					fmt.Sprintf("claim %q binds evidence %s that is not in the evidence table", claim.ID, id), claims.ChapterID, claim.ID)
			}
			if len(claim.ReferenceKeys) > 0 && !containsString(claim.ReferenceKeys, ev.ReferenceKey) {
				return writerGuardError(CodeClaimUnknownEvidence,
					fmt.Sprintf("claim %q binds evidence %s from reference %q but cites %v", claim.ID, id, ev.ReferenceKey, claim.ReferenceKeys), claims.ChapterID, claim.ID)
			}
		}
	}
	return nil
}

// WriteGuardedDraftBundle runs the writer guard and only then persists the
// chapter artifacts. A guard failure returns the structured error and blocks
// the whole bundle (draft, claims, citation map) from being written.
func WriteGuardedDraftBundle(s store.Store, bundle artifacts.DraftBundle) (artifacts.WriteResult, error) {
	if err := GuardWriterClaims(s, bundle.Claims); err != nil {
		return artifacts.WriteResult{}, err
	}
	confirmed, err := readConfirmedReferences(s)
	if err != nil {
		return artifacts.WriteResult{}, AgentError(CodeToolExecutionFailed, fmt.Sprintf("read confirmed references: %v", err), false)
	}
	for _, issue := range artifacts.ValidateDraftArtifacts(bundle.Claims, bundle.CitationMap, confirmed) {
		switch issue.Code {
		case artifacts.CodeUnconfirmedRef:
			return artifacts.WriteResult{}, writerGuardError(CodeClaimUnconfirmedReference, issue.Message, bundle.ChapterID, issue.ID)
		case artifacts.CodeMissingClaims, artifacts.CodeMissingCitationMap, artifacts.CodeUnknownClaim, artifacts.CodeHighClaimMissingRef:
			return artifacts.WriteResult{}, writerGuardError(CodeToolExecutionFailed, issue.Message, bundle.ChapterID, issue.ID)
		}
	}
	return artifacts.WriteDraftBundle(s, bundle)
}

// writerQualityPromptSections returns the writer evidence protocol for the
// Coordinator/Writer prompt. Content rules stay with the LLM; the Host only
// enforces the machine-checkable bindings.
func writerQualityPromptSections() []string {
	return []string{
		"Writer chapter input: writer_run injects the chapter quality plan and the contents of its required evidence. Key claims must follow the quality plan and cite the evidence bound to this chapter.",
		"Writer evidence hard rule: every claim in claims.json must carry evidence_ids (at least one) that exist in the evidence table, and may only cite confirmed reference keys. When material is insufficient, state the gap explicitly in writer notes instead of padding; never fabricate sources, evidence, or reference keys.",
		"Writer guard: the Host validates claims before chapter artifacts are written. Missing or unknown evidence ids and unconfirmed reference keys return a structured error and block the chapter output.",
	}
}

func writerGuardError(code, message, chapterID, claimID string) Error {
	err := AgentError(code, message, false)
	err.Details = map[string]any{"chapter_id": chapterID, "claim_id": claimID}
	return err
}

func loadSectionQualityPlanIfPresent(s store.Store) (quality.SectionQualityPlan, bool, error) {
	if _, err := os.Stat(quality.SectionQualityPlanJSONPath(s)); os.IsNotExist(err) {
		return quality.SectionQualityPlan{}, false, nil
	}
	plan, err := quality.LoadSectionQualityPlan(s)
	if err != nil {
		return quality.SectionQualityPlan{}, false, AgentError(CodeToolExecutionFailed, fmt.Sprintf("load section quality plan: %v", err), false)
	}
	return plan, true, nil
}

func loadEvidenceTableIfPresent(s store.Store) (quality.EvidenceTable, bool, error) {
	if _, err := os.Stat(quality.EvidenceTableJSONPath(s)); os.IsNotExist(err) {
		return quality.EvidenceTable{}, false, nil
	}
	table, err := quality.LoadEvidenceTable(s)
	if err != nil {
		return quality.EvidenceTable{}, false, AgentError(CodeToolExecutionFailed, fmt.Sprintf("load evidence table: %v", err), false)
	}
	return table, true, nil
}

func containsString(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}
