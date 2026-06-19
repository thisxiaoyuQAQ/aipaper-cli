package quality

// PaperQualityPolicyVersion identifies the local runtime policy distilled from
// docs/skills/paper-cli-paper-quality. Runtime code uses this stable helper
// instead of reading docs/skills files at execution time.
const PaperQualityPolicyVersion = "paper-cli-paper-quality-v1"

// PaperQualityPolicy groups short prompt/report sections by runtime scope. The
// slices are intentionally concise and ordered so tests can assert stable output
// without snapshotting large prose blocks.
type PaperQualityPolicy struct {
	Version             string
	CoordinatorSections []string
	ArchitectNarrative  []string
	EvidenceDepth       []string
	SectionPlan         []string
	Writer              []string
	Verifier            []string
	Editor              []string
	Report              []string
}

// DefaultPaperQualityPolicy returns the complete local Paper Quality Skill
// policy. It is pure: no filesystem, Store, network, or model access.
func DefaultPaperQualityPolicy() PaperQualityPolicy {
	return PaperQualityPolicy{
		Version:             PaperQualityPolicyVersion,
		CoordinatorSections: PaperQualityCoordinatorSections(),
		ArchitectNarrative:  PaperQualityArchitectNarrativeSections(),
		EvidenceDepth:       PaperQualityEvidenceDepthSections(),
		SectionPlan:         PaperQualitySectionPlanSections(),
		Writer:              PaperQualityWriterSections(),
		Verifier:            PaperQualityVerifierSections(),
		Editor:              PaperQualityEditorSections(),
		Report:              PaperQualityReportSections(),
	}
}

func PaperQualityCoordinatorSections() []string {
	return []string{
		"Paper Quality Skill policy (" + PaperQualityPolicyVersion + "): use the local runtime policy, not external docs/skills files. Apply it across outline, evidence, writing, verification, review, and export.",
		"Paper quality hard rules: only confirmed references may be cited; citation existence is not claim support; evidence depth controls claim strength; insufficient evidence must become a softened claim, a gap, writer_notes, or human review, never unsupported body text.",
		"Paper quality boundary: Host performs machine checks, Coordinator decides workflow from tool facts, and role agents make semantic judgments within the existing JSON contracts. Do not introduce unsupported fields such as claim_type unless the host contract changes.",
	}
}

func PaperQualityArchitectNarrativeSections() []string {
	return []string{
		"Narrative contract: design the paper around one evidence-bounded thesis or synthesis, not a list of materials. Each chapter needs one clear role in the argument.",
		"Outline quality: avoid duplicate background chapters and avoid chapters whose main body is merely 'evidence is insufficient'. Gaps belong in section quality plans and human review hints.",
	}
}

func PaperQualityEvidenceDepthSections() []string {
	return []string{
		"Evidence depth rubric: metadata_only supports only bibliographic existence or topic relevance; abstract supports high-level summaries; snippet supports only the quoted/local finding; fulltext_excerpt supports only what the excerpt actually covers.",
		"Depth honesty: do not use abstract or metadata evidence for strong causal, comparative, or generalization claims. If the source is shallow, mark limitations, risk flags, gaps, or human review instead of inventing detail.",
	}
}

func PaperQualitySectionPlanSections() []string {
	return []string{
		"SectionPlan rubric: questions must be answerable by the chapter; required_evidence_ids must serve those questions; boundaries prevent overlap; forbidden_generalizations list strong claims the evidence cannot support.",
		"SectionPlan gaps are real evidence gaps, not placeholders for fabrication. Human review hints should flag shallow evidence, uncertain scope, missing full text, or claims needing domain judgment.",
	}
}

func PaperQualityWriterSections() []string {
	return []string{
		"Paper quality writing policy: every important claim must appear in claims[] and bind confirmed evidence_ids; match wording strength to evidence depth and chapter boundaries.",
		"Do not make evidence gaps, 'pending verification', or 'only a framework can be proposed' the body of a chapter. Put real gaps in writer_notes and write only evidence-supported analysis in draft_markdown.",
		"Academic style guard: use specific, restrained prose; avoid generic openings, inflated adjectives, mechanical transitions, unsupported significance claims, and decorative formatting that changes neither evidence nor meaning.",
	}
}

func PaperQualityVerifierSections() []string {
	return []string{
		"Claim support rubric: internally classify each claim as descriptive, comparative, causal, generalization, methodological, limitation, or reproducibility before assigning support. Mention the type in verifier_note when it explains risk.",
		"Verifier evidence test: support requires same object, relation, and scope. A plausible claim without matching evidence is unsupported; a directionally supported but too broad claim is overstated; uncertainty counts as unsupported.",
		"Output only the existing ClaimVerdict contract: claim_id, support, risk_level, verifier_note. Do not add claim_type or other unsupported fields.",
	}
}

func PaperQualityEditorSections() []string {
	return []string{
		"Reviewer rubric: review quality, clarity, significance, synthesis, scope calibration, and citation integrity. Distinguish language problems from evidence problems and structural problems.",
		"Rewrite rubric: every unsupported, overstated, or high-risk claim must receive a concrete required rewrite instruction with location, problem, instruction, relevant suggested_evidence_ids, and claim_id when available.",
		"Human review trigger: if the problem requires new evidence, full-text reading, domain judgment, or experiments unavailable to the Writer, mark human review or a gap rather than asking the Writer to fabricate.",
	}
}

func PaperQualityReportSections() []string {
	return []string{
		"Report policy version: " + PaperQualityPolicyVersion,
		"Report should explain evidence depth, claim support risk, rewrite convergence, and concrete human action items without calling an LLM.",
	}
}

// PaperQualityForbiddenDraftPatterns returns low-content patterns that should be
// injected into WriterChapterInput.ForbiddenDraftPatterns.
func PaperQualityForbiddenDraftPatterns() []string {
	return []string{
		"正文主体反复说明证据不足、待验证、只能提出框架，而没有基于已确认证据展开实质分析",
		"将 metadata_only 或 abstract 级证据写成强因果、强比较或跨场景泛化结论",
		"使用显著、全面、颠覆性、深刻影响等无证据支撑的宣传式评价",
	}
}
