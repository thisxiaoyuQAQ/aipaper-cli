package agent

// Quality Engine boundary additions for module 24: step names and prompt
// sections for evidence_extraction / section_quality_plan. Existing decision
// semantics in coordinator.go / prompt.go are not changed.

// Quality Engine checkpoint step names. Both steps go through the existing
// checkpoint mechanism (artifacts written before Record, recoverable).
const (
	StepEvidenceExtraction = "evidence_extraction"
	StepSectionQualityPlan = "section_quality_plan"
)

// qualityPromptSections returns the Coordinator/Architect prompt extension for
// the quality steps. Planning rules stay with the LLM; the Host only provides
// validated persistence tools.
func qualityPromptSections() []string {
	return []string{
		"Quality step evidence_extraction: after confirm_references, Architect distills an evidence table from confirmed references and parsed material texts, then persists it with the save_evidence_table tool. Depth must be honest: never claim snippet or fulltext evidence without extracted material text.",
		"Quality step section_quality_plan: with or right after create_outline, Architect produces a per-chapter quality plan (questions, required evidence ids, recommended references, boundaries, forbidden generalizations, gaps, human review hints) and persists it with the save_section_quality_plan tool. Every chapter_id must match the outline and every required evidence id must exist in the evidence table.",
		"Both quality steps are recorded through the existing checkpoint mechanism and must not be repeated when a valid recovery prompt lists their artifacts.",
	}
}
