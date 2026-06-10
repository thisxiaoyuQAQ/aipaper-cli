package agent

// Module 27 boundary additions: the claim_verification step name and the
// prompt sections for the verifier protocol and the quality gate. Existing
// decision semantics in coordinator.go / prompt.go are not changed; the gate
// matrix itself is pure Host logic in internal/quality/gate.go.

// StepClaimVerification runs after claim_extraction and before review_chapter;
// it goes through the existing checkpoint mechanism (artifacts written before
// Record, recoverable).
const StepClaimVerification = "claim_verification"

// claimVerificationPromptSections returns the Coordinator prompt extension for
// the claim_verification step and the quality gate. The Editor (as verifier)
// owns the semantic support judgment; the Host validates, records, and gates.
func claimVerificationPromptSections() []string {
	return []string{
		"Quality step claim_verification: after claim_extraction and before review_chapter, the Editor acts as verifier and judges every claim in the claim graph as supported, partially_supported, unsupported, or overstated against its bound evidence, then persists the verdicts with the save_verification_result tool (claim_id + support, optional risk_level and verifier_note). When uncertain, mark unsupported: uncertainty never passes. In fast quality mode skip this semantic judgment entirely; support stays skipped from extraction and Host hard checks still apply.",
		"Quality gate: after verification call the quality_gate_check tool with the quality mode and per-chapter rewrite round counts. The Host alone resolves pass, pass_with_warnings, needs_revision, needs_human_review, or blocked. Hard blocks (unconfirmed or fabricated reference keys, claims without evidence, evidence pointing at unconfirmed references) fire in every mode. The gate runs in parallel with the existing chapter review gate: either one blocking blocks the chapter. Rewrite at most two rounds as before; an exceeded chapter becomes needs_human_review without interrupting the run.",
		"The claim_verification step is recorded through the existing checkpoint mechanism and must not be repeated when a valid recovery prompt lists quality/verification-result.json.",
	}
}
