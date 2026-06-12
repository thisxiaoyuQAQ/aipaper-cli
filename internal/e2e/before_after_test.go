package e2e

// Module 31, layer three: before/after structural comparison (spec 7.3).
// The same two-chapter mini review runs twice on the quality-mini materials:
// once as a legacy run without quality artifacts (compatibility mode) and once
// in enhanced mode with the full evidence chain. No real provider is involved;
// the writer/editor outputs are mock data, so the comparison is structural:
// evidence binding, claim support accounting, gate conclusion, and report
// surfaces. The numbers logged here feed docs/QualityEngine验收报告.md.

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/agent"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/artifacts"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
	finalexport "github.com/thisxiaoyuQAQ/aipaper-cli/internal/export"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/quality"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/store"
)

const qualityReportRel = "final/quality-report.md"

type flowMetrics struct {
	claimsTotal        int
	claimsWithEvidence int
	supported          int
	unsupported        int
	verdicts           int
	gateConclusion     string
	qualityReport      bool
	metadata           string
}

// beforeAfterClaims returns the chapter claims for one flow. The texts are
// identical across flows; only the evidence bindings differ.
func beforeAfterClaims(setup qualityMiniSetup, withEvidence bool) (ch01 []contracts.Claim, ch02 []contracts.Claim) {
	evidence := func(ids ...string) []string {
		if !withEvidence {
			return nil
		}
		return ids
	}
	ch01 = []contracts.Claim{{
		ID:            "ch01_claim_001",
		Text:          qmAbsoluteClaim,
		Importance:    "high",
		ReferenceKeys: []string{setup.chenKey},
		EvidenceIDs:   evidence("ev_001"),
	}}
	ch02 = []contracts.Claim{
		{
			ID:            "ch02_claim_001",
			Text:          qmAbsoluteClaim,
			Importance:    "medium",
			ReferenceKeys: []string{setup.garciaKey},
			EvidenceIDs:   evidence("ev_002"),
		},
		{
			ID:            "ch02_claim_002",
			Text:          qmUnsupportedClaim,
			Importance:    "medium",
			ReferenceKeys: []string{setup.garciaKey},
			EvidenceIDs:   evidence("ev_002"),
		},
	}
	return ch01, ch02
}

func acceptAndExport(t *testing.T, s store.Store) (finalexport.Result, finalexport.ExportInput) {
	t.Helper()
	for _, chapterID := range []string{"ch01", "ch02"} {
		review := mockReview(chapterID, 1, true, 86, 94)
		if _, err := artifacts.WriteReview(s, review, "accepted in mock flow"); err != nil {
			t.Fatalf("WriteReview(%s) error = %v", chapterID, err)
		}
		if _, err := artifacts.CommitAccepted(s, chapterID, 1, review); err != nil {
			t.Fatalf("CommitAccepted(%s) error = %v", chapterID, err)
		}
	}
	input, err := finalexport.LoadInput(s)
	if err != nil {
		t.Fatalf("LoadInput() error = %v", err)
	}
	result, err := finalexport.ExportFinal(s, input, finalexport.Options{Now: fixedTime()})
	if err != nil {
		t.Fatalf("ExportFinal() error = %v", err)
	}
	return result, input
}

func collectMetrics(t *testing.T, s store.Store, result finalexport.Result, input finalexport.ExportInput) flowMetrics {
	t.Helper()
	m := flowMetrics{}
	for _, chapter := range input.Chapters {
		for _, claim := range chapter.Claims.Claims {
			m.claimsTotal++
			if len(claim.EvidenceIDs) > 0 {
				m.claimsWithEvidence++
			}
		}
	}
	if input.Quality.Available {
		for _, node := range input.Quality.ClaimGraph.Claims {
			switch node.Support {
			case quality.SupportSupported:
				m.supported++
			case quality.SupportUnsupported:
				m.unsupported++
			}
		}
		m.verdicts = len(input.Quality.VerificationResult.Verdicts)
		m.gateConclusion = input.Quality.GateOutcome.Conclusion
	}
	if _, err := os.Stat(s.Path("final", "quality-report.md")); err == nil {
		m.qualityReport = true
	} else if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("stat quality-report.md: %v", err)
	}
	if conclusion, ok := result.Metadata["quality_conclusion"].(string); ok {
		m.metadata = conclusion
	}
	return m
}

// TestE2EBeforeAfterStructuralComparison runs the legacy flow and the enhanced
// flow on the same materials and asserts the user-visible structural deltas.
func TestE2EBeforeAfterStructuralComparison(t *testing.T) {
	// ---- Before: legacy run, no quality artifacts (compatibility mode).
	legacySetup := setupQualityMini(t)
	ls := legacySetup.store
	// A legacy run predates the quality engine: remove the quality artifacts
	// the shared fixture setup created.
	if err := os.RemoveAll(ls.Path("quality")); err != nil {
		t.Fatalf("remove quality dir: %v", err)
	}
	legacyCh01, legacyCh02 := beforeAfterClaims(legacySetup, false)
	if _, err := artifacts.WriteDraftBundle(ls, qualityDraftBundle("ch01", legacyCh01, []string{legacySetup.chenKey})); err != nil {
		t.Fatalf("legacy WriteDraftBundle(ch01) error = %v", err)
	}
	if _, err := artifacts.WriteDraftBundle(ls, qualityDraftBundle("ch02", legacyCh02, []string{legacySetup.garciaKey})); err != nil {
		t.Fatalf("legacy WriteDraftBundle(ch02) error = %v", err)
	}
	legacyResult, legacyInput := acceptAndExport(t, ls)
	before := collectMetrics(t, ls, legacyResult, legacyInput)

	if before.qualityReport {
		t.Fatalf("legacy flow unexpectedly produced %s", qualityReportRel)
	}
	if !hasIssue(legacyResult.Issues, finalexport.CodeQualityArtifactsMissing) {
		t.Fatalf("legacy issues = %#v, want %s", legacyResult.Issues, finalexport.CodeQualityArtifactsMissing)
	}
	legacyReport := readStoreText(t, ls, "final/report.md")
	if !strings.Contains(legacyReport, "compatibility mode") {
		t.Fatalf("legacy report.md lacks compatibility note:\n%s", legacyReport)
	}
	if before.claimsWithEvidence != 0 {
		t.Fatalf("legacy evidence bindings = %d, want 0", before.claimsWithEvidence)
	}
	var legacyWarnings int
	for _, chapter := range legacyInput.Chapters {
		legacyWarnings += len(artifacts.ClaimEvidenceWarnings(chapter.Claims))
	}
	if legacyWarnings != before.claimsTotal {
		t.Fatalf("legacy compat warnings = %d, want %d", legacyWarnings, before.claimsTotal)
	}

	// ---- After: enhanced run with the full evidence chain.
	enhancedSetup := setupQualityMini(t)
	es := enhancedSetup.store
	enhancedCh01, enhancedCh02 := beforeAfterClaims(enhancedSetup, true)
	if _, err := agent.WriteGuardedDraftBundle(es, qualityDraftBundle("ch01", enhancedCh01, []string{enhancedSetup.chenKey})); err != nil {
		t.Fatalf("enhanced WriteGuardedDraftBundle(ch01) error = %v", err)
	}
	if _, err := agent.WriteGuardedDraftBundle(es, qualityDraftBundle("ch02", enhancedCh02, []string{enhancedSetup.garciaKey})); err != nil {
		t.Fatalf("enhanced WriteGuardedDraftBundle(ch02) error = %v", err)
	}
	if _, _, err := quality.ExtractChapterClaimGraph(es, "ch01", 1, false, fixedTime()); err != nil {
		t.Fatalf("ExtractChapterClaimGraph(ch01) error = %v", err)
	}
	graph, _, err := quality.ExtractChapterClaimGraph(es, "ch02", 1, false, fixedTime())
	if err != nil {
		t.Fatalf("ExtractChapterClaimGraph(ch02) error = %v", err)
	}
	verdicts := []quality.ClaimVerdict{
		{ClaimID: "claim_001", Support: quality.SupportSupported, RiskLevel: quality.RiskHigh,
			VerifierNote: "absolute conclusion resting on abstract-level evidence"},
	}
	for _, node := range graph.Claims {
		if node.ChapterID != "ch02" {
			continue
		}
		if node.Text == qmUnsupportedClaim {
			verdicts = append(verdicts, quality.ClaimVerdict{ClaimID: node.ID, Support: quality.SupportUnsupported,
				VerifierNote: "evidence does not cover shift workers"})
		} else {
			verdicts = append(verdicts, quality.ClaimVerdict{ClaimID: node.ID, Support: quality.SupportSupported, RiskLevel: quality.RiskLow})
		}
	}
	if _, _, _, err := quality.SaveVerificationResult(es, verdicts, fixedTime()); err != nil {
		t.Fatalf("SaveVerificationResult() error = %v", err)
	}
	enhancedResult, enhancedInput := acceptAndExport(t, es)
	after := collectMetrics(t, es, enhancedResult, enhancedInput)

	if !after.qualityReport {
		t.Fatalf("enhanced flow did not produce %s", qualityReportRel)
	}
	qualityReport := readStoreText(t, es, qualityReportRel)
	for _, want := range []string{
		"# Quality Report",
		"Quality mode: `enhanced`",
		"Overall quality status: `needs_revision`",
		qmUnsupportedClaim,
	} {
		if !strings.Contains(qualityReport, want) {
			t.Fatalf("quality-report.md missing %q:\n%s", want, qualityReport)
		}
	}
	enhancedReport := readStoreText(t, es, "final/report.md")
	if !strings.Contains(enhancedReport, "## Quality Summary") || strings.Contains(enhancedReport, "compatibility mode") {
		t.Fatalf("enhanced report.md quality summary wrong:\n%s", enhancedReport)
	}
	if !strings.Contains(after.metadata, "质量门控") {
		t.Fatalf("enhanced metadata = %q, want gate conclusion", after.metadata)
	}
	if after.claimsWithEvidence != after.claimsTotal || after.claimsTotal != 3 {
		t.Fatalf("enhanced evidence bindings = %d/%d, want 3/3", after.claimsWithEvidence, after.claimsTotal)
	}
	if after.supported != 2 || after.unsupported != 1 || after.verdicts != 3 {
		t.Fatalf("enhanced support accounting = %+v, want supported=2 unsupported=1 verdicts=3", after)
	}
	if after.gateConclusion != quality.GateNeedsRevision {
		t.Fatalf("enhanced gate conclusion = %s, want needs_revision (unsupported claim surfaced)", after.gateConclusion)
	}

	// Comparison summary for docs/QualityEngine验收报告.md.
	t.Logf("before/after comparison:")
	t.Logf("  evidence bindings: before %d/%d, after %d/%d", before.claimsWithEvidence, before.claimsTotal, after.claimsWithEvidence, after.claimsTotal)
	t.Logf("  verification verdicts: before %d, after %d", before.verdicts, after.verdicts)
	t.Logf("  support accounting: before unknown, after supported=%d unsupported=%d", after.supported, after.unsupported)
	t.Logf("  gate conclusion: before none (compat), after %s", after.gateConclusion)
	t.Logf("  quality-report.md: before %v, after %v", before.qualityReport, after.qualityReport)
	t.Logf("  metadata: before %q, after %q", before.metadata, after.metadata)
}

func hasIssue(issues []finalexport.Issue, code string) bool {
	for _, issue := range issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}
