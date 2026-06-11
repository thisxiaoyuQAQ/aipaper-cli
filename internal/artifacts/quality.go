package artifacts

import (
	"fmt"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
)

const (
	StatusDrafting          = "drafting"
	StatusReviewing         = "reviewing"
	StatusRevisionRequired  = "revision_required"
	StatusAccepted          = "accepted"
	StatusNeedsHumanReview  = "needs_human_review"
	StatusCommitted         = "committed"
	MinOverallScore         = 80
	MinCitationConsistency  = 90
	MaxRevisionRounds       = 2
	CodeMissingClaims       = "ARTIFACT_MISSING_CLAIMS"
	CodeMissingCitationMap  = "ARTIFACT_MISSING_CITATION_MAP"
	CodeUnknownClaim        = "ARTIFACT_UNKNOWN_CLAIM"
	CodeUnconfirmedRef      = "ARTIFACT_UNCONFIRMED_REFERENCE"
	CodeHighClaimMissingRef = "ARTIFACT_HIGH_CLAIM_MISSING_REFERENCE"
)

type GateResult struct {
	Passed  bool     `json:"passed"`
	Status  string   `json:"status"`
	Reasons []string `json:"reasons,omitempty"`
}

type ValidationIssue struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	ID      string `json:"id,omitempty"`
}

func EvaluateReview(review contracts.Review) GateResult {
	var reasons []string
	if !review.Passed {
		reasons = append(reasons, "review marked not passed")
	}
	if review.Scores.Overall < MinOverallScore {
		reasons = append(reasons, fmt.Sprintf("overall score %d < %d", review.Scores.Overall, MinOverallScore))
	}
	if review.Scores.CitationConsistency < MinCitationConsistency {
		reasons = append(reasons, fmt.Sprintf("citation consistency %d < %d", review.Scores.CitationConsistency, MinCitationConsistency))
	}
	if len(review.UnsupportedClaims) > 0 {
		reasons = append(reasons, "unsupported claims present")
	}
	if required := countRequiredRewriteInstructions(review); required > 0 {
		reasons = append(reasons, fmt.Sprintf("%d required rewrite instructions present", required))
	}
	if len(reasons) > 0 {
		return GateResult{Passed: false, Status: StatusRevisionRequired, Reasons: reasons}
	}
	return GateResult{Passed: true, Status: StatusAccepted}
}

func countRequiredRewriteInstructions(review contracts.Review) int {
	count := 0
	for _, instruction := range review.RewriteInstructions {
		if instruction.Severity == "required" {
			count++
		}
	}
	return count
}

func StatusAfterReview(review contracts.Review, revisionRounds int) GateResult {
	gate := EvaluateReview(review)
	if gate.Passed {
		return gate
	}
	if revisionRounds >= MaxRevisionRounds {
		gate.Status = StatusNeedsHumanReview
		return gate
	}
	gate.Status = StatusRevisionRequired
	return gate
}

func ValidateDraftArtifacts(claims contracts.ClaimsFile, citationMap contracts.CitationMap, confirmed contracts.ConfirmedReferences) []ValidationIssue {
	var issues []ValidationIssue
	if len(claims.Claims) == 0 {
		issues = append(issues, ValidationIssue{Code: CodeMissingClaims, Message: "claims file has no claims"})
	}
	if len(citationMap.Mappings) == 0 {
		issues = append(issues, ValidationIssue{Code: CodeMissingCitationMap, Message: "citation map has no mappings"})
	}

	claimIDs := map[string]contracts.Claim{}
	for _, claim := range claims.Claims {
		claimIDs[claim.ID] = claim
		if claim.Importance == "high" && len(claim.ReferenceKeys) == 0 {
			issues = append(issues, ValidationIssue{Code: CodeHighClaimMissingRef, Message: "high importance claim has no references", ID: claim.ID})
		}
		for _, key := range claim.ReferenceKeys {
			if !confirmedReferenceExists(confirmed, key) {
				issues = append(issues, ValidationIssue{Code: CodeUnconfirmedRef, Message: fmt.Sprintf("claim references unconfirmed key %q", key), ID: claim.ID})
			}
		}
	}

	for _, mapping := range citationMap.Mappings {
		for _, claimID := range mapping.ClaimIDs {
			if _, ok := claimIDs[claimID]; !ok {
				issues = append(issues, ValidationIssue{Code: CodeUnknownClaim, Message: fmt.Sprintf("citation map references unknown claim %q", claimID), ID: mapping.ParagraphID})
			}
		}
		for _, key := range mapping.ReferenceKeys {
			if !confirmedReferenceExists(confirmed, key) {
				issues = append(issues, ValidationIssue{Code: CodeUnconfirmedRef, Message: fmt.Sprintf("citation map references unconfirmed key %q", key), ID: mapping.ParagraphID})
			}
		}
	}
	return issues
}

func confirmedReferenceExists(confirmed contracts.ConfirmedReferences, key string) bool {
	for _, ref := range confirmed.Items {
		if ref.Key == key {
			return true
		}
	}
	return false
}
