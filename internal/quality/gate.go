package quality

// Module 27: quality_gate_check, pure Host logic. Input is the claim graph
// (with verification verdicts already applied), the evidence table, the
// confirmed reference keys, the quality mode, and the per-chapter rewrite
// round counts. Hard blocks fire in every mode; graded risks escalate per
// mode following the matrix in docs/interfaces/quality.md section 5.
//
// "Strong conclusion / key claim" is a semantic judgment: the verifier grades
// such claims risk_level=high, and the Host combines that grade with the
// machine-checked evidence depth. The Host never judges claim semantics.

import (
	"fmt"
	"sort"
	"time"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/artifacts"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/store"
)

// Quality modes. An empty mode defaults to enhanced.
const (
	ModeFast     = "fast"
	ModeEnhanced = "enhanced"
	ModeStrict   = "strict"
)

// Gate conclusions, ordered by severity.
const (
	GatePass             = "pass"
	GatePassWithWarnings = "pass_with_warnings"
	GateNeedsRevision    = "needs_revision"
	GateNeedsHumanReview = "needs_human_review"
	GateBlocked          = "blocked"
)

// Issue severities inside a gate outcome. Blocked and needs_* reuse the
// conclusion names; warning never escalates the conclusion past
// pass_with_warnings.
const (
	SeverityWarning          = "warning"
	SeverityNeedsRevision    = GateNeedsRevision
	SeverityNeedsHumanReview = GateNeedsHumanReview
	SeverityBlocked          = GateBlocked
)

// Hard block issue codes (all modes). A reference key that exists nowhere in
// confirmed.json covers both unconfirmed and fabricated keys: the machine
// check cannot tell them apart and blocks both.
const (
	CodeGateUnconfirmedReference       = "gate_unconfirmed_reference"
	CodeGateClaimMissingEvidence       = "gate_claim_missing_evidence"
	CodeGateUnknownEvidence            = "gate_unknown_evidence"
	CodeGateEvidenceUnconfirmedRef     = "gate_evidence_unconfirmed_reference"
	CodeGateInvalidMode                = "gate_invalid_mode"
	CodeGateShallowEvidenceStrongClaim = "gate_shallow_evidence_strong_claim"
	CodeGateMetadataOnlySoleSupport    = "gate_metadata_only_sole_support"
	CodeGateUnsupportedClaim           = "gate_unsupported_claim"
	CodeGateOverstatedClaim            = "gate_overstated_claim"
	CodeGatePartiallySupportedClaim    = "gate_partially_supported_claim"
	CodeGateDuplicateClaim             = "gate_duplicate_claim"
	CodeGateRewriteRoundsExceeded      = "gate_rewrite_rounds_exceeded"
	CodeGateUnverifiedClaim            = "gate_unverified_claim"
)

// GateIssue is one finding (hard block or graded risk) with its resolved
// severity for the active mode. TopPriority marks strict-mode rewrite
// overruns that the quality report must surface first.
type GateIssue struct {
	Code        string `json:"code"`
	ClaimID     string `json:"claim_id,omitempty"`
	ChapterID   string `json:"chapter_id,omitempty"`
	Message     string `json:"message"`
	Severity    string `json:"severity"`
	TopPriority bool   `json:"top_priority,omitempty"`
}

// GateOutcome is the quality_gate_check result.
type GateOutcome struct {
	Conclusion string      `json:"conclusion"`
	Mode       string      `json:"mode"`
	CheckedAt  time.Time   `json:"checked_at"`
	Blockers   []GateIssue `json:"blockers,omitempty"`
	Findings   []GateIssue `json:"findings,omitempty"`
}

// GateInput is the pure-evaluation input. ConfirmedKeys and EvidenceByID are
// machine facts loaded from the store; RewriteRounds maps chapter id to the
// rewrite rounds already spent.
type GateInput struct {
	Mode          string
	Graph         ClaimGraph
	ConfirmedKeys map[string]bool
	EvidenceByID  map[string]Evidence
	RewriteRounds map[string]int
}

var conclusionRank = map[string]int{
	GatePass:             0,
	GatePassWithWarnings: 1,
	GateNeedsRevision:    2,
	GateNeedsHumanReview: 3,
	GateBlocked:          4,
}

var depthRank = map[string]int{
	DepthMetadataOnly:    0,
	DepthAbstract:        1,
	DepthSnippet:         2,
	DepthFulltextExcerpt: 3,
}

// NormalizeMode validates the quality mode; empty defaults to enhanced.
func NormalizeMode(mode string) (string, error) {
	switch mode {
	case "":
		return ModeEnhanced, nil
	case ModeFast, ModeEnhanced, ModeStrict:
		return mode, nil
	}
	return "", NewError(CodeGateInvalidMode, "invalid quality mode "+quoted(mode), false)
}

// RunQualityGate loads the claim graph, evidence table, and confirmed keys
// from the store and evaluates the gate. The claim graph must exist; a missing
// evidence table or confirmed.json degrades to empty sets so stale bindings
// surface as hard blocks instead of passing silently.
func RunQualityGate(s store.Store, mode string, rewriteRounds map[string]int, now time.Time) (GateOutcome, error) {
	normalized, err := NormalizeMode(mode)
	if err != nil {
		return GateOutcome{}, err
	}
	graph, err := LoadClaimGraph(s)
	if err != nil {
		return GateOutcome{}, err
	}
	confirmed, err := loadConfirmedKeys(s)
	if err != nil {
		return GateOutcome{}, err
	}
	evidenceByID := map[string]Evidence{}
	table, tableErr := LoadEvidenceTable(s)
	if tableErr == nil {
		for _, item := range table.Items {
			evidenceByID[item.ID] = item
		}
	}
	outcome := EvaluateQualityGate(GateInput{
		Mode:          normalized,
		Graph:         graph,
		ConfirmedKeys: confirmed,
		EvidenceByID:  evidenceByID,
		RewriteRounds: rewriteRounds,
	})
	outcome.CheckedAt = now.UTC()
	return outcome, nil
}

// EvaluateQualityGate is the deterministic gate matrix. It never reads the
// filesystem; callers supply every fact.
func EvaluateQualityGate(input GateInput) GateOutcome {
	mode := input.Mode
	if mode == "" {
		mode = ModeEnhanced
	}
	outcome := GateOutcome{Conclusion: GatePass, Mode: mode}
	for _, node := range input.Graph.Claims {
		outcome.Blockers = append(outcome.Blockers, hardBlocks(node, input)...)
		outcome.Findings = append(outcome.Findings, gradedRisks(node, input, mode)...)
	}
	outcome.Findings = append(outcome.Findings, rewriteOverruns(input.RewriteRounds, mode)...)
	sortIssues(outcome.Blockers)
	sortIssues(outcome.Findings)
	if len(outcome.Blockers) > 0 {
		outcome.Conclusion = GateBlocked
		return outcome
	}
	for _, finding := range outcome.Findings {
		outcome.Conclusion = worseConclusion(outcome.Conclusion, severityConclusion(finding.Severity))
	}
	return outcome
}

// hardBlocks returns the bottom-line violations that block in every mode:
// unconfirmed or fabricated reference keys on the claim, a claim without any
// evidence binding, evidence ids missing from the table, and evidence whose
// own reference is no longer confirmed.
func hardBlocks(node ClaimNode, input GateInput) []GateIssue {
	var issues []GateIssue
	block := func(code, message string) {
		issues = append(issues, GateIssue{
			Code: code, ClaimID: node.ID, ChapterID: node.ChapterID,
			Message: message, Severity: SeverityBlocked,
		})
	}
	for _, key := range node.ReferenceKeys {
		if !input.ConfirmedKeys[key] {
			block(CodeGateUnconfirmedReference,
				fmt.Sprintf("claim %s cites reference key %q that is not in references/confirmed.json (unconfirmed or fabricated)", node.ID, key))
		}
	}
	if len(node.EvidenceIDs) == 0 {
		block(CodeGateClaimMissingEvidence,
			fmt.Sprintf("claim %s has no evidence binding", node.ID))
	}
	for _, evidenceID := range node.EvidenceIDs {
		ev, ok := input.EvidenceByID[evidenceID]
		if !ok {
			block(CodeGateUnknownEvidence,
				fmt.Sprintf("claim %s binds evidence %s that does not exist in the evidence table", node.ID, evidenceID))
			continue
		}
		if !input.ConfirmedKeys[ev.ReferenceKey] {
			block(CodeGateEvidenceUnconfirmedRef,
				fmt.Sprintf("claim %s binds evidence %s whose reference %q is not confirmed", node.ID, evidenceID, ev.ReferenceKey))
		}
	}
	return issues
}

// gradedRisks resolves the per-mode risk matrix for one claim.
func gradedRisks(node ClaimNode, input GateInput, mode string) []GateIssue {
	var issues []GateIssue
	risk := func(code, message string, severity string) {
		issues = append(issues, GateIssue{
			Code: code, ClaimID: node.ID, ChapterID: node.ChapterID,
			Message: message, Severity: severity,
		})
	}
	switch node.Support {
	case SupportUnsupported:
		risk(CodeGateUnsupportedClaim,
			fmt.Sprintf("claim %s is unsupported by its evidence", node.ID),
			supportSeverity(mode))
	case SupportOverstated:
		risk(CodeGateOverstatedClaim,
			fmt.Sprintf("claim %s overstates its evidence", node.ID),
			supportSeverity(mode))
	case SupportPartiallySupported:
		risk(CodeGatePartiallySupportedClaim,
			fmt.Sprintf("claim %s is only partially supported", node.ID),
			strictOnlyRevision(mode))
	case "":
		// F9: an unverified claim never passes silently. Fast mode marks
		// skipped at extraction, so empty support means verification was
		// expected and did not happen.
		risk(CodeGateUnverifiedClaim,
			fmt.Sprintf("claim %s has no verification verdict; uncertainty counts as unsupported", node.ID),
			supportSeverity(mode))
	}
	if node.RiskLevel == RiskHigh && len(node.EvidenceIDs) > 0 {
		maxDepth, allKnown := maxEvidenceDepth(node.EvidenceIDs, input.EvidenceByID)
		if allKnown {
			if maxDepth == depthRank[DepthMetadataOnly] {
				risk(CodeGateMetadataOnlySoleSupport,
					fmt.Sprintf("key claim %s is supported only by metadata_only evidence", node.ID),
					strictOnlyRevision(mode))
			} else if maxDepth <= depthRank[DepthAbstract] {
				risk(CodeGateShallowEvidenceStrongClaim,
					fmt.Sprintf("strong claim %s is supported only by abstract-level evidence", node.ID),
					strictOnlyRevision(mode))
			}
		}
	}
	if len(node.DuplicateOf) > 0 {
		risk(CodeGateDuplicateClaim,
			fmt.Sprintf("claim %s repeats earlier claims in other chapters: %v", node.ID, node.DuplicateOf),
			SeverityWarning)
	}
	return issues
}

// rewriteOverruns marks chapters whose rewrite loop exceeded the existing
// artifacts.MaxRevisionRounds limit. Every mode escalates to
// needs_human_review without interrupting the run; strict pins the issue to
// the top of the quality report.
func rewriteOverruns(rounds map[string]int, mode string) []GateIssue {
	chapters := make([]string, 0, len(rounds))
	for chapterID := range rounds {
		chapters = append(chapters, chapterID)
	}
	sort.Strings(chapters)
	var issues []GateIssue
	for _, chapterID := range chapters {
		if rounds[chapterID] <= artifacts.MaxRevisionRounds {
			continue
		}
		issues = append(issues, GateIssue{
			Code:      CodeGateRewriteRoundsExceeded,
			ChapterID: chapterID,
			Message: fmt.Sprintf("chapter %s exceeded %d rewrite rounds (%d); needs human review without interrupting the run",
				chapterID, artifacts.MaxRevisionRounds, rounds[chapterID]),
			Severity:    SeverityNeedsHumanReview,
			TopPriority: mode == ModeStrict,
		})
	}
	return issues
}

// supportSeverity resolves the unsupported/overstated/unverified row:
// fast=warning, enhanced=needs_revision, strict=needs_revision.
func supportSeverity(mode string) string {
	if mode == ModeFast {
		return SeverityWarning
	}
	return SeverityNeedsRevision
}

// strictOnlyRevision resolves the rows that only strict escalates
// (shallow evidence, metadata-only sole support, partially_supported):
// fast=warning, enhanced=warning, strict=needs_revision.
func strictOnlyRevision(mode string) string {
	if mode == ModeStrict {
		return SeverityNeedsRevision
	}
	return SeverityWarning
}

// maxEvidenceDepth returns the highest depth rank among the bound evidence.
// allKnown is false when any id is missing from the table (already a hard
// block, so depth risks stay quiet for it).
func maxEvidenceDepth(ids []string, byID map[string]Evidence) (int, bool) {
	max := -1
	for _, id := range ids {
		ev, ok := byID[id]
		if !ok {
			return 0, false
		}
		if rank := depthRank[ev.Depth]; rank > max {
			max = rank
		}
	}
	return max, true
}

func severityConclusion(severity string) string {
	if severity == SeverityWarning {
		return GatePassWithWarnings
	}
	return severity
}

func worseConclusion(a, b string) string {
	if conclusionRank[b] > conclusionRank[a] {
		return b
	}
	return a
}

// CombineConclusions returns the most severe conclusion, implementing the
// parallel composition with the existing internal/artifacts chapter gate:
// any blocked blocks.
func CombineConclusions(conclusions ...string) string {
	combined := GatePass
	for _, c := range conclusions {
		combined = worseConclusion(combined, c)
	}
	return combined
}

// ChapterGateConclusion maps the existing artifacts chapter gate result
// (overall >= 80, citation consistency >= 90) into a gate conclusion so both
// gates compose through CombineConclusions.
func ChapterGateConclusion(result artifacts.GateResult) string {
	if result.Passed {
		return GatePass
	}
	if result.Status == artifacts.StatusNeedsHumanReview {
		return GateNeedsHumanReview
	}
	return GateNeedsRevision
}

func sortIssues(issues []GateIssue) {
	sort.SliceStable(issues, func(i, j int) bool {
		if issues[i].TopPriority != issues[j].TopPriority {
			return issues[i].TopPriority
		}
		return false
	})
}
