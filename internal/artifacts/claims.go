package artifacts

// Module 25 boundary extension: evidence binding compatibility for claims.json.
// Existing artifact rules in quality.go / write.go are not changed.

import (
	"fmt"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
)

const CodeClaimMissingEvidence = "ARTIFACT_CLAIM_MISSING_EVIDENCE"

// ClaimEvidenceWarnings reports claims that carry no evidence bindings.
// Legacy claims.json written before the quality engine has no evidence_ids
// field and still loads; callers reading old runs surface these issues as
// warnings and must not block on them. New writer output is hard-validated
// separately by the writer guard before it is persisted.
func ClaimEvidenceWarnings(claims contracts.ClaimsFile) []ValidationIssue {
	var issues []ValidationIssue
	for _, claim := range claims.Claims {
		if len(claim.EvidenceIDs) == 0 {
			issues = append(issues, ValidationIssue{
				Code:    CodeClaimMissingEvidence,
				Message: fmt.Sprintf("claim %q has no evidence bindings (legacy claims.json reads in compatibility mode)", claim.ID),
				ID:      claim.ID,
			})
		}
	}
	return issues
}
