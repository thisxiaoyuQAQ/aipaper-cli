package agent

// Module 26 boundary additions: the claim_extraction step name and the prompt
// section for the post-writing claim graph. Existing decision semantics in
// coordinator.go / prompt.go are not changed.

// StepClaimExtraction runs after each chapter writer completes; it goes
// through the existing checkpoint mechanism (artifacts written before Record,
// recoverable).
const StepClaimExtraction = "claim_extraction"

// claimGraphPromptSections returns the Coordinator prompt extension for the
// claim_extraction step. Projection and merge are deterministic Host logic;
// the Coordinator only decides when to run the tool.
func claimGraphPromptSections() []string {
	return []string{
		"Quality step claim_extraction: after each chapter writer completes, call the extract_chapter_claims tool with the chapter id and draft version to project the chapter claims and citation map into the claim graph. The Host merges chapter by chapter (never a full overwrite), flags cross-chapter duplicate claims as graded risk, and machine-validates every reference key, evidence id, and chapter id. Pass fast_mode in fast quality mode so support is marked skipped instead of awaiting verification.",
		"The claim_extraction step is recorded through the existing checkpoint mechanism and must not be repeated when a valid recovery prompt lists quality/claim-graph.json for the chapter.",
	}
}
