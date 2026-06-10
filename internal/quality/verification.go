package quality

// Module 27: claim verification result. The Editor (acting as verifier) judges
// every claim's support level; the Host validates the verdicts against the
// claim graph, writes them back onto the reserved ClaimNode fields (Support,
// RiskLevel, VerifierNote), and persists quality/verification-result.json.
// Verdicts merge per claim id; verdicts for claims no longer in the graph
// (chapter re-extraction reassigns ids) are pruned instead of failing.

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/store"
)

// Structured error codes for verification result validation.
const (
	CodeVerificationInvalid        = "verification_invalid"
	CodeVerificationDuplicateClaim = "verification_duplicate_claim"
	CodeVerificationUnknownClaim   = "verification_unknown_claim"
	CodeVerificationIOFailed       = "verification_io_failed"
)

// VerificationResultJSONRel is the path relative to the store root.
const VerificationResultJSONRel = "quality/verification-result.json"

// VerificationResult is the persisted verifier output for the claim graph.
type VerificationResult struct {
	UpdatedAt time.Time      `json:"updated_at"`
	Verdicts  []ClaimVerdict `json:"verdicts"`
}

// ClaimVerdict is the verifier judgment for one claim. Support is required:
// an uncertain verifier must mark unsupported instead of leaving it empty
// (F9: uncertainty never passes).
type ClaimVerdict struct {
	ClaimID      string `json:"claim_id"`             // must exist in the claim graph
	Support      string `json:"support"`              // supported|partially_supported|unsupported|overstated|skipped
	RiskLevel    string `json:"risk_level,omitempty"` // high|medium|low, empty keeps the graph grade
	VerifierNote string `json:"verifier_note,omitempty"`
}

// VerificationResultJSONPath returns the absolute JSON path inside the store.
func VerificationResultJSONPath(s store.Store) string {
	return s.Path(filepath.FromSlash(VerificationResultJSONRel))
}

// ValidateVerdicts checks every verdict against the claim graph: claim ids
// must exist, support must be a non-empty valid level, and risk levels must be
// valid. Returns a structured Error on first failure.
func ValidateVerdicts(graph ClaimGraph, verdicts []ClaimVerdict) error {
	ids := make(map[string]bool, len(graph.Claims))
	for _, node := range graph.Claims {
		ids[node.ID] = true
	}
	seen := make(map[string]bool, len(verdicts))
	for i, verdict := range verdicts {
		at := fmt.Sprintf("verdicts[%d]", i)
		if verdict.ClaimID == "" {
			return NewError(CodeVerificationInvalid, at+": claim_id is required", false)
		}
		if seen[verdict.ClaimID] {
			return NewError(CodeVerificationDuplicateClaim, at+": duplicate verdict for claim "+verdict.ClaimID, false)
		}
		seen[verdict.ClaimID] = true
		if !ids[verdict.ClaimID] {
			return NewError(CodeVerificationUnknownClaim,
				at+": claim "+verdict.ClaimID+" does not exist in the claim graph", false)
		}
		if verdict.Support == "" || !validSupport(verdict.Support) {
			return NewError(CodeVerificationInvalid, at+": invalid support "+quoted(verdict.Support), false)
		}
		if !validRiskLevel(verdict.RiskLevel) {
			return NewError(CodeVerificationInvalid, at+": invalid risk_level "+quoted(verdict.RiskLevel), false)
		}
	}
	return nil
}

// ApplyVerification writes the verdicts back onto the matching claim nodes.
// Support and verifier_note are overwritten; an empty verdict risk_level keeps
// the existing graph grade (e.g. the duplicate-detection low mark).
func ApplyVerification(graph ClaimGraph, verdicts []ClaimVerdict, updatedAt time.Time) ClaimGraph {
	byClaim := make(map[string]ClaimVerdict, len(verdicts))
	for _, verdict := range verdicts {
		byClaim[verdict.ClaimID] = verdict
	}
	graph.UpdatedAt = updatedAt
	for i, node := range graph.Claims {
		verdict, ok := byClaim[node.ID]
		if !ok {
			continue
		}
		graph.Claims[i].Support = verdict.Support
		graph.Claims[i].VerifierNote = verdict.VerifierNote
		if verdict.RiskLevel != "" {
			graph.Claims[i].RiskLevel = verdict.RiskLevel
		}
	}
	return graph
}

// MergeVerificationVerdicts merges new verdicts into an existing result:
// verdicts replace earlier ones for the same claim id, verdicts for claims no
// longer present in the graph are pruned, and the output is sorted by claim id.
func MergeVerificationVerdicts(existing VerificationResult, verdicts []ClaimVerdict, graph ClaimGraph, updatedAt time.Time) VerificationResult {
	ids := make(map[string]bool, len(graph.Claims))
	for _, node := range graph.Claims {
		ids[node.ID] = true
	}
	byClaim := make(map[string]ClaimVerdict, len(existing.Verdicts)+len(verdicts))
	for _, verdict := range existing.Verdicts {
		if ids[verdict.ClaimID] {
			byClaim[verdict.ClaimID] = verdict
		}
	}
	for _, verdict := range verdicts {
		byClaim[verdict.ClaimID] = verdict
	}
	merged := VerificationResult{UpdatedAt: updatedAt, Verdicts: make([]ClaimVerdict, 0, len(byClaim))}
	for _, verdict := range byClaim {
		merged.Verdicts = append(merged.Verdicts, verdict)
	}
	sort.Slice(merged.Verdicts, func(i, j int) bool {
		return merged.Verdicts[i].ClaimID < merged.Verdicts[j].ClaimID
	})
	return merged
}

// SaveVerificationResult validates the verdicts against the persisted claim
// graph, writes them back onto the graph (Support/RiskLevel/VerifierNote),
// saves the updated graph, and persists the merged verification result.
// Returns the merged result, the updated graph, and the written paths.
func SaveVerificationResult(s store.Store, verdicts []ClaimVerdict, now time.Time) (VerificationResult, ClaimGraph, []string, error) {
	graph, err := LoadClaimGraph(s)
	if err != nil {
		return VerificationResult{}, ClaimGraph{}, nil, err
	}
	if err := ValidateVerdicts(graph, verdicts); err != nil {
		return VerificationResult{}, ClaimGraph{}, nil, err
	}
	existing := VerificationResult{}
	if _, statErr := os.Stat(VerificationResultJSONPath(s)); !os.IsNotExist(statErr) {
		// Only a missing result starts empty; a corrupt file must surface
		// instead of silently dropping earlier verdicts.
		existing, err = LoadVerificationResult(s)
		if err != nil {
			return VerificationResult{}, ClaimGraph{}, nil, err
		}
	}
	graph = ApplyVerification(graph, verdicts, now)
	outputs, err := SaveClaimGraph(s, graph)
	if err != nil {
		return VerificationResult{}, ClaimGraph{}, nil, err
	}
	merged := MergeVerificationVerdicts(existing, verdicts, graph, now)
	if _, err := store.WriteJSON(VerificationResultJSONPath(s), merged, store.Overwrite); err != nil {
		return VerificationResult{}, ClaimGraph{}, nil,
			NewError(CodeVerificationIOFailed, fmt.Sprintf("write %s: %v", VerificationResultJSONRel, err), true)
	}
	return merged, graph, append([]string{VerificationResultJSONRel}, outputs...), nil
}

// LoadVerificationResult reads quality/verification-result.json with strict
// JSON parsing. A missing file returns a structured error.
func LoadVerificationResult(s store.Store) (VerificationResult, error) {
	var result VerificationResult
	err := store.ReadJSON(VerificationResultJSONPath(s), &result)
	if errors.Is(err, os.ErrNotExist) {
		return VerificationResult{}, NewError(CodeVerificationIOFailed, "verification result not found: "+VerificationResultJSONRel, false)
	}
	if err != nil {
		return VerificationResult{}, NewError(CodeVerificationIOFailed, fmt.Sprintf("read %s: %v", VerificationResultJSONRel, err), false)
	}
	return result, nil
}
