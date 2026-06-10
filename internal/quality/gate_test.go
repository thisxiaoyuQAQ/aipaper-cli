package quality

import (
	"testing"
	"time"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/artifacts"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
)

func gateEvidence() map[string]Evidence {
	return map[string]Evidence{
		"ev_meta":     {ID: "ev_meta", ReferenceKey: "doe2024deep", Depth: DepthMetadataOnly},
		"ev_abstract": {ID: "ev_abstract", ReferenceKey: "doe2024deep", Depth: DepthAbstract},
		"ev_snippet":  {ID: "ev_snippet", ReferenceKey: "lee2023graph", Depth: DepthSnippet},
	}
}

func gateConfirmed() map[string]bool {
	return map[string]bool{"doe2024deep": true, "lee2023graph": true}
}

func gateClaim(id, support, risk string, evidence ...string) ClaimNode {
	return ClaimNode{
		ID: id, Text: "Claim " + id, ChapterID: "ch01",
		ReferenceKeys: []string{"doe2024deep"},
		EvidenceIDs:   evidence,
		Support:       support,
		RiskLevel:     risk,
	}
}

func gateInput(mode string, claims ...ClaimNode) GateInput {
	return GateInput{
		Mode:          mode,
		Graph:         ClaimGraph{UpdatedAt: time.Date(2026, 6, 10, 16, 0, 0, 0, time.UTC), Claims: claims},
		ConfirmedKeys: gateConfirmed(),
		EvidenceByID:  gateEvidence(),
	}
}

func findIssue(issues []GateIssue, code string) *GateIssue {
	for i := range issues {
		if issues[i].Code == code {
			return &issues[i]
		}
	}
	return nil
}

func TestNormalizeMode(t *testing.T) {
	for input, want := range map[string]string{
		"": ModeEnhanced, ModeFast: ModeFast, ModeEnhanced: ModeEnhanced, ModeStrict: ModeStrict,
	} {
		got, err := NormalizeMode(input)
		if err != nil || got != want {
			t.Fatalf("NormalizeMode(%q) = %q, %v", input, got, err)
		}
	}
	_, err := NormalizeMode("paranoid")
	assertQualityError(t, err, CodeGateInvalidMode)
}

// TestGateHardBlocksEveryMode runs the four bottom-line violations one by one
// and asserts every mode blocks: unconfirmed/fabricated reference key, claim
// without evidence binding, evidence id missing from the table, and evidence
// pointing at an unconfirmed reference.
func TestGateHardBlocksEveryMode(t *testing.T) {
	cases := []struct {
		name     string
		claim    ClaimNode
		input    func(GateInput) GateInput
		wantCode string
	}{
		{
			name: "unconfirmed or fabricated reference key",
			claim: ClaimNode{ID: "claim_001", Text: "Bad key.", ChapterID: "ch01",
				ReferenceKeys: []string{"fake2020key"}, EvidenceIDs: []string{"ev_abstract"}, Support: SupportSupported},
			wantCode: CodeGateUnconfirmedReference,
		},
		{
			name: "claim without evidence binding",
			claim: ClaimNode{ID: "claim_001", Text: "No evidence.", ChapterID: "ch01",
				ReferenceKeys: []string{"doe2024deep"}, Support: SupportSupported},
			wantCode: CodeGateClaimMissingEvidence,
		},
		{
			name:     "evidence id missing from the table",
			claim:    gateClaim("claim_001", SupportSupported, "", "ev_404"),
			wantCode: CodeGateUnknownEvidence,
		},
		{
			name:  "evidence pointing at unconfirmed reference",
			claim: gateClaim("claim_001", SupportSupported, "", "ev_abstract"),
			input: func(in GateInput) GateInput {
				in.ConfirmedKeys = map[string]bool{"doe2024deep": true}
				in.EvidenceByID = map[string]Evidence{
					"ev_abstract": {ID: "ev_abstract", ReferenceKey: "gone2019ref", Depth: DepthAbstract},
				}
				return in
			},
			wantCode: CodeGateEvidenceUnconfirmedRef,
		},
	}
	for _, tc := range cases {
		for _, mode := range []string{ModeFast, ModeEnhanced, ModeStrict} {
			t.Run(tc.name+"/"+mode, func(t *testing.T) {
				input := gateInput(mode, tc.claim)
				if tc.input != nil {
					input = tc.input(input)
				}
				outcome := EvaluateQualityGate(input)
				if outcome.Conclusion != GateBlocked {
					t.Fatalf("conclusion = %q, want blocked", outcome.Conclusion)
				}
				issue := findIssue(outcome.Blockers, tc.wantCode)
				if issue == nil || issue.Severity != SeverityBlocked {
					t.Fatalf("blockers = %#v, want %s", outcome.Blockers, tc.wantCode)
				}
			})
		}
	}
}

// TestGateMatrixAcrossModes feeds the same graded-risk inputs through fast,
// enhanced, and strict and asserts the conclusions follow the matrix in
// docs/interfaces/quality.md section 5 row by row.
func TestGateMatrixAcrossModes(t *testing.T) {
	cases := []struct {
		name  string
		claim ClaimNode
		code  string
		want  map[string]string // mode -> conclusion
	}{
		{
			name:  "abstract evidence supporting a strong claim",
			claim: gateClaim("claim_001", SupportSupported, RiskHigh, "ev_abstract"),
			code:  CodeGateShallowEvidenceStrongClaim,
			want: map[string]string{
				ModeFast: GatePassWithWarnings, ModeEnhanced: GatePassWithWarnings, ModeStrict: GateNeedsRevision,
			},
		},
		{
			name:  "metadata_only as sole support of a key claim",
			claim: gateClaim("claim_001", SupportSupported, RiskHigh, "ev_meta"),
			code:  CodeGateMetadataOnlySoleSupport,
			want: map[string]string{
				ModeFast: GatePassWithWarnings, ModeEnhanced: GatePassWithWarnings, ModeStrict: GateNeedsRevision,
			},
		},
		{
			name:  "unsupported claim",
			claim: gateClaim("claim_001", SupportUnsupported, "", "ev_snippet"),
			code:  CodeGateUnsupportedClaim,
			want: map[string]string{
				ModeFast: GatePassWithWarnings, ModeEnhanced: GateNeedsRevision, ModeStrict: GateNeedsRevision,
			},
		},
		{
			name:  "overstated claim",
			claim: gateClaim("claim_001", SupportOverstated, "", "ev_snippet"),
			code:  CodeGateOverstatedClaim,
			want: map[string]string{
				ModeFast: GatePassWithWarnings, ModeEnhanced: GateNeedsRevision, ModeStrict: GateNeedsRevision,
			},
		},
		{
			name:  "partially supported claim",
			claim: gateClaim("claim_001", SupportPartiallySupported, "", "ev_snippet"),
			code:  CodeGatePartiallySupportedClaim,
			want: map[string]string{
				ModeFast: GatePassWithWarnings, ModeEnhanced: GatePassWithWarnings, ModeStrict: GateNeedsRevision,
			},
		},
		{
			name: "cross-chapter duplicate claim",
			claim: func() ClaimNode {
				node := gateClaim("claim_002", SupportSupported, RiskLow, "ev_snippet")
				node.DuplicateOf = []string{"claim_001"}
				return node
			}(),
			code: CodeGateDuplicateClaim,
			want: map[string]string{
				ModeFast: GatePassWithWarnings, ModeEnhanced: GatePassWithWarnings, ModeStrict: GatePassWithWarnings,
			},
		},
		{
			name:  "unverified claim treated as not passing",
			claim: gateClaim("claim_001", "", "", "ev_snippet"),
			code:  CodeGateUnverifiedClaim,
			want: map[string]string{
				ModeFast: GatePassWithWarnings, ModeEnhanced: GateNeedsRevision, ModeStrict: GateNeedsRevision,
			},
		},
	}
	for _, tc := range cases {
		for mode, want := range tc.want {
			t.Run(tc.name+"/"+mode, func(t *testing.T) {
				outcome := EvaluateQualityGate(gateInput(mode, tc.claim))
				if outcome.Conclusion != want {
					t.Fatalf("conclusion = %q, want %q (findings %#v)", outcome.Conclusion, want, outcome.Findings)
				}
				if findIssue(outcome.Findings, tc.code) == nil {
					t.Fatalf("findings missing %s: %#v", tc.code, outcome.Findings)
				}
				if len(outcome.Blockers) != 0 {
					t.Fatalf("graded risk produced blockers: %#v", outcome.Blockers)
				}
			})
		}
	}
}

func TestGateSkippedAndSupportedPass(t *testing.T) {
	// fast mode: skipped support with clean machine checks passes outright.
	outcome := EvaluateQualityGate(gateInput(ModeFast, gateClaim("claim_001", SupportSkipped, "", "ev_snippet")))
	if outcome.Conclusion != GatePass || len(outcome.Findings) != 0 {
		t.Fatalf("fast skipped outcome = %#v", outcome)
	}
	// snippet-supported strong claim is fine even in strict.
	outcome = EvaluateQualityGate(gateInput(ModeStrict, gateClaim("claim_001", SupportSupported, RiskHigh, "ev_snippet")))
	if outcome.Conclusion != GatePass {
		t.Fatalf("strict snippet outcome = %#v", outcome)
	}
	// abstract evidence on a non-high-risk claim raises no depth finding.
	outcome = EvaluateQualityGate(gateInput(ModeStrict, gateClaim("claim_001", SupportSupported, RiskMedium, "ev_abstract")))
	if outcome.Conclusion != GatePass {
		t.Fatalf("strict medium-risk abstract outcome = %#v", outcome)
	}
}

func TestGateRewriteOverrunNeedsHumanReviewWithoutInterrupting(t *testing.T) {
	for _, mode := range []string{ModeFast, ModeEnhanced, ModeStrict} {
		input := gateInput(mode, gateClaim("claim_001", SupportSupported, "", "ev_snippet"))
		input.RewriteRounds = map[string]int{"ch01": 3, "ch02": 2}
		outcome := EvaluateQualityGate(input)
		if outcome.Conclusion != GateNeedsHumanReview {
			t.Fatalf("%s conclusion = %q, want needs_human_review", mode, outcome.Conclusion)
		}
		issue := findIssue(outcome.Findings, CodeGateRewriteRoundsExceeded)
		if issue == nil || issue.ChapterID != "ch01" {
			t.Fatalf("%s findings = %#v", mode, outcome.Findings)
		}
		if got, want := issue.TopPriority, mode == ModeStrict; got != want {
			t.Fatalf("%s top_priority = %v, want %v", mode, got, want)
		}
		// ch02 stayed within the limit and produces no finding.
		count := 0
		for _, finding := range outcome.Findings {
			if finding.Code == CodeGateRewriteRoundsExceeded {
				count++
			}
		}
		if count != 1 {
			t.Fatalf("%s overrun findings = %d", mode, count)
		}
	}
}

func TestGateStrictTopPriorityIssueSortsFirst(t *testing.T) {
	input := gateInput(ModeStrict,
		gateClaim("claim_001", SupportUnsupported, "", "ev_snippet"))
	input.RewriteRounds = map[string]int{"ch01": 3}
	outcome := EvaluateQualityGate(input)
	if len(outcome.Findings) < 2 || !outcome.Findings[0].TopPriority ||
		outcome.Findings[0].Code != CodeGateRewriteRoundsExceeded {
		t.Fatalf("strict findings order = %#v", outcome.Findings)
	}
}

func TestCombineConclusionsParallelGates(t *testing.T) {
	cases := []struct {
		conclusions []string
		want        string
	}{
		{[]string{GatePass, GatePass}, GatePass},
		{[]string{GatePass, GatePassWithWarnings}, GatePassWithWarnings},
		{[]string{GatePassWithWarnings, GateNeedsRevision}, GateNeedsRevision},
		{[]string{GateNeedsHumanReview, GateNeedsRevision}, GateNeedsHumanReview},
		{[]string{GatePass, GateBlocked}, GateBlocked}, // any blocked blocks
	}
	for _, tc := range cases {
		if got := CombineConclusions(tc.conclusions...); got != tc.want {
			t.Fatalf("CombineConclusions(%v) = %q, want %q", tc.conclusions, got, tc.want)
		}
	}
}

func TestChapterGateConclusionMapsArtifactsGate(t *testing.T) {
	passing := artifacts.EvaluateReview(contracts.Review{
		Passed: true,
		Scores: contracts.ReviewScores{Overall: 90, CitationConsistency: 95},
	})
	if got := ChapterGateConclusion(passing); got != GatePass {
		t.Fatalf("passing chapter gate = %q", got)
	}
	failing := artifacts.StatusAfterReview(contracts.Review{
		Passed: false,
		Scores: contracts.ReviewScores{Overall: 70, CitationConsistency: 95},
	}, 0)
	if got := ChapterGateConclusion(failing); got != GateNeedsRevision {
		t.Fatalf("failing chapter gate = %q", got)
	}
	exceeded := artifacts.StatusAfterReview(contracts.Review{
		Passed: false,
		Scores: contracts.ReviewScores{Overall: 70, CitationConsistency: 95},
	}, artifacts.MaxRevisionRounds)
	if got := ChapterGateConclusion(exceeded); got != GateNeedsHumanReview {
		t.Fatalf("exceeded chapter gate = %q", got)
	}
}

func TestRunQualityGateLoadsStoreFacts(t *testing.T) {
	s := setupClaimGraphStore(t)
	now := time.Date(2026, 6, 10, 16, 0, 0, 0, time.UTC)

	// Missing claim graph is an io failure, and an invalid mode fails first.
	_, err := RunQualityGate(s, "paranoid", nil, now)
	assertQualityError(t, err, CodeGateInvalidMode)
	_, err = RunQualityGate(s, ModeEnhanced, nil, now)
	assertQualityError(t, err, CodeClaimGraphIOFailed)

	graph := twoClaimGraph()
	graph.Claims[0].Support = SupportSupported
	graph.Claims[1].Support = SupportUnsupported
	if _, err := SaveClaimGraph(s, graph); err != nil {
		t.Fatalf("save graph: %v", err)
	}
	outcome, err := RunQualityGate(s, "", nil, now)
	if err != nil {
		t.Fatalf("RunQualityGate() error = %v", err)
	}
	if outcome.Mode != ModeEnhanced || outcome.Conclusion != GateNeedsRevision || !outcome.CheckedAt.Equal(now) {
		t.Fatalf("outcome = %#v", outcome)
	}
}

func TestQualityGateCheckToolReturnsOutcome(t *testing.T) {
	s := setupClaimGraphStore(t)
	graph := twoClaimGraph()
	graph.Claims[0].Support = SupportSupported
	graph.Claims[1].Support = SupportSupported
	if _, err := SaveClaimGraph(s, graph); err != nil {
		t.Fatalf("save graph: %v", err)
	}
	tool := NewQualityGateCheckTool(s)
	response := execTool(t, tool, `{"mode":"strict","rewrite_rounds":{"ch01":3}}`)
	if !response.OK {
		t.Fatalf("gate tool response = %#v", response)
	}
	data, ok := response.Data.(map[string]any)
	if !ok || data["conclusion"] != GateNeedsHumanReview || data["mode"] != ModeStrict {
		t.Fatalf("gate tool data = %#v", response.Data)
	}

	response = execTool(t, tool, `{"mode":"paranoid"}`)
	if response.OK || response.Error == nil || response.Error.Code != CodeGateInvalidMode {
		t.Fatalf("invalid mode response = %#v", response)
	}
	response = execTool(t, tool, `{"mode":"fast","extra":1}`)
	if response.OK || response.Error == nil || response.Error.Code != CodeGateInvalidMode {
		t.Fatalf("strict args response = %#v", response)
	}
}
