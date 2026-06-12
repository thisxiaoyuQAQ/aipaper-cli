package e2e

// Module 31, layer two: evidence-chain fixture acceptance (spec 7.2).
// Uses fixtures/quality-mini with deliberately bad samples, all mock, no
// network. Asserts the hard-block bottom line in every mode, the graded risk
// matrix across fast/enhanced/strict, cross-chapter duplicate marking, and
// rewrite-overrun escalation without interrupting the flow.

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/agent"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/app"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/artifacts"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/config"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/materials"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/quality"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/references"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/store"
)

const (
	qmAbsoluteClaim    = "CBT-I eliminates insomnia for all adults."
	qmUnsupportedClaim = "Digital interventions never work for shift workers."
)

// qualityMiniSetup bootstraps a store on the quality-mini fixture: materials
// parsed, two of three BibTeX candidates confirmed (lee2025 deliberately left
// unconfirmed), a two-chapter outline, and an evidence table with one
// abstract-level and one snippet-level item.
type qualityMiniSetup struct {
	store      store.Store
	chenKey    string // confirmed, abstract-level evidence ev_001
	garciaKey  string // confirmed, snippet-level evidence ev_002
	snippetMat string // material id backing the snippet evidence
}

func setupQualityMini(t *testing.T) qualityMiniSetup {
	t.Helper()
	workDir := t.TempDir()
	if _, err := app.Bootstrap(workDir, config.Config{Provider: "mock", Model: "quality-mini"}); err != nil {
		t.Fatalf("Bootstrap() error = %v", err)
	}
	s := store.New(workDir)

	req := contracts.Requirements{
		Topic:             "Behavioral and digital treatment of insomnia",
		ResearchQuestions: []string{"How durable are CBT-I effects across delivery modes?"},
		Scope:             "quality mini review",
		Language:          "en",
		CitationStyle:     "APA",
		QualityMode:       quality.ModeEnhanced,
		TargetWords:       600,
		MaterialDir:       "../../fixtures/quality-mini/materials",
		AllowOnlineSearch: false,
	}
	if _, err := store.WriteJSON(s.RequirementsPath(), req, store.Overwrite); err != nil {
		t.Fatalf("write requirements: %v", err)
	}

	materialResult, err := materials.ProcessDir(req.MaterialDir, s)
	if err != nil {
		t.Fatalf("ProcessDir() error = %v", err)
	}
	if len(materialResult.Candidates) != 3 {
		t.Fatalf("bibtex candidates = %d, want 3", len(materialResult.Candidates))
	}
	snippetMat := ""
	for _, item := range materialResult.Manifest.Items {
		if strings.Contains(item.Path, "digital_sleep") && item.Status == "parsed" {
			snippetMat = item.ID
		}
	}
	if snippetMat == "" {
		t.Fatalf("digital_sleep material not parsed: %#v", materialResult.Manifest.Items)
	}

	candidates := contracts.ReferenceCandidates{
		Items: references.AssignCandidateIDs(references.DedupeCandidates(materialResult.Candidates), 1),
	}
	if _, err := store.WriteJSON(s.Path("references", "candidates.json"), candidates, store.Overwrite); err != nil {
		t.Fatalf("write candidates: %v", err)
	}
	var confirmIDs []string
	for _, candidate := range candidates.Items {
		// lee2025 stays unconfirmed on purpose: citing it must hard-block.
		if strings.Contains(candidate.DOI, "wearables-sleep") {
			continue
		}
		confirmIDs = append(confirmIDs, candidate.ID)
	}
	if len(confirmIDs) != 2 {
		t.Fatalf("confirm ids = %v, want 2 of 3", confirmIDs)
	}
	confirmation, err := references.ConfirmCandidates(s, candidates, references.ConfirmationDecision{
		ConfirmedIDs: confirmIDs,
		ConfirmedAt:  fixedTime(),
	})
	if err != nil {
		t.Fatalf("ConfirmCandidates() error = %v", err)
	}
	setup := qualityMiniSetup{store: s, snippetMat: snippetMat}
	for _, ref := range confirmation.Confirmed.Items {
		switch ref.DOI {
		case "10.1000/cbti-outcomes":
			setup.chenKey = ref.Key
		case "10.1000/digital-sleep":
			setup.garciaKey = ref.Key
		}
	}
	if setup.chenKey == "" || setup.garciaKey == "" {
		t.Fatalf("confirmed keys missing: %#v", confirmation.Confirmed.Items)
	}

	outline := map[string]any{
		"title_suggestion": "Insomnia Treatment Mini Review",
		"chapters": []map[string]any{
			{"chapter_id": "ch01", "title": "Therapist Guided CBT-I"},
			{"chapter_id": "ch02", "title": "Digital Delivery"},
		},
	}
	if _, err := store.WriteJSON(s.Path("outline", "outline.json"), outline, store.Overwrite); err != nil {
		t.Fatalf("write outline.json: %v", err)
	}

	table := quality.EvidenceTable{
		GeneratedAt: fixedTime(),
		Items: []quality.Evidence{
			{
				ID:           "ev_001",
				ReferenceKey: setup.chenKey,
				Depth:        quality.DepthAbstract,
				Topics:       []string{"cbt-i"},
				KeyFindings:  []string{"improved sleep onset latency in a randomized trial"},
				Confidence:   "medium",
			},
			{
				ID:           "ev_002",
				ReferenceKey: setup.garciaKey,
				MaterialID:   snippetMat,
				Depth:        quality.DepthSnippet,
				Topics:       []string{"digital sleep"},
				KeyFindings:  []string{"app delivered programs retain partial efficacy"},
				Excerpt:      "Reported effect sizes vary widely across studies.",
				Confidence:   "medium",
			},
		},
	}
	if _, err := quality.SaveEvidenceTable(s, table); err != nil {
		t.Fatalf("SaveEvidenceTable() error = %v", err)
	}
	return setup
}

func qualityDraftBundle(chapterID string, claims []contracts.Claim, citedKeys []string) artifacts.DraftBundle {
	claimIDs := make([]string, 0, len(claims))
	for _, claim := range claims {
		claimIDs = append(claimIDs, claim.ID)
	}
	return artifacts.DraftBundle{
		ChapterID:     chapterID,
		Version:       1,
		DraftMarkdown: "## " + chapterID + "\n\nDraft body for the quality mini fixture.",
		Claims: contracts.ClaimsFile{
			ChapterID:    chapterID,
			DraftVersion: 1,
			Claims:       claims,
		},
		CitationMap: contracts.CitationMap{
			ChapterID:    chapterID,
			DraftVersion: 1,
			Mappings: []contracts.CitationMapping{{
				ParagraphID:   chapterID + "_p001",
				ClaimIDs:      claimIDs,
				ReferenceKeys: citedKeys,
			}},
		},
		WriterNotes: "mock writer output",
	}
}

func assertGuardBlocks(t *testing.T, s store.Store, bundle artifacts.DraftBundle, wantCode string) {
	t.Helper()
	_, err := agent.WriteGuardedDraftBundle(s, bundle)
	agentErr, ok := agent.AsError(err)
	if !ok || agentErr.Code != wantCode {
		t.Fatalf("guard error = %v, want code %s", err, wantCode)
	}
	rel, pathErr := artifacts.DraftPath(bundle.ChapterID, bundle.Version)
	if pathErr != nil {
		t.Fatalf("DraftPath() error = %v", pathErr)
	}
	if _, statErr := os.Stat(s.Path(filepath.FromSlash(rel))); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("blocked draft %s was written anyway (stat err = %v)", rel, statErr)
	}
}

func findGateIssue(issues []quality.GateIssue, code string) (quality.GateIssue, bool) {
	for _, issue := range issues {
		if issue.Code == code {
			return issue, true
		}
	}
	return quality.GateIssue{}, false
}

// TestQualityMiniHardBlocksAtWriterGuard: the three bottom-line bad samples
// (unconfirmed key, claim without evidence, forged key) are blocked by the
// Host before any chapter artifact is written.
func TestQualityMiniHardBlocksAtWriterGuard(t *testing.T) {
	setup := setupQualityMini(t)
	s := setup.store

	unconfirmed := qualityDraftBundle("ch01", []contracts.Claim{{
		ID:            "ch01_claim_001",
		Text:          "Wearables match polysomnography accuracy.",
		Importance:    "high",
		ReferenceKeys: []string{"lee2025sleeptracking"}, // candidate exists but was never confirmed
		EvidenceIDs:   []string{"ev_001"},
	}}, []string{"lee2025sleeptracking"})
	assertGuardBlocks(t, s, unconfirmed, agent.CodeClaimUnconfirmedReference)

	missingEvidence := qualityDraftBundle("ch01", []contracts.Claim{{
		ID:            "ch01_claim_001",
		Text:          "CBT-I improves sleep onset latency.",
		Importance:    "high",
		ReferenceKeys: []string{setup.chenKey},
	}}, []string{setup.chenKey})
	assertGuardBlocks(t, s, missingEvidence, agent.CodeClaimMissingEvidence)

	forged := qualityDraftBundle("ch01", []contracts.Claim{{
		ID:            "ch01_claim_001",
		Text:          "A fabricated source supports this claim.",
		Importance:    "high",
		ReferenceKeys: []string{"ghost2099fabricated"},
		EvidenceIDs:   []string{"ev_001"},
	}}, []string{"ghost2099fabricated"})
	assertGuardBlocks(t, s, forged, agent.CodeClaimUnconfirmedReference)
}

// TestQualityMiniGateMatrixAndDuplicates drives the good-path chapters through
// guard, claim extraction, verification, and the gate in all three modes:
// abstract-level evidence behind an absolute conclusion is warning in enhanced
// and needs_revision in strict; an unsupported claim downgrades to a warning
// only in fast; the cross-chapter duplicate is marked but never blocks; and a
// rewrite overrun escalates to needs_human_review without an error.
func TestQualityMiniGateMatrixAndDuplicates(t *testing.T) {
	setup := setupQualityMini(t)
	s := setup.store

	ch01 := qualityDraftBundle("ch01", []contracts.Claim{{
		ID:            "ch01_claim_001",
		Text:          qmAbsoluteClaim, // absolute conclusion on abstract-level ev_001
		Importance:    "high",
		ReferenceKeys: []string{setup.chenKey},
		EvidenceIDs:   []string{"ev_001"},
	}}, []string{setup.chenKey})
	if _, err := agent.WriteGuardedDraftBundle(s, ch01); err != nil {
		t.Fatalf("WriteGuardedDraftBundle(ch01) error = %v", err)
	}
	ch02 := qualityDraftBundle("ch02", []contracts.Claim{
		{
			ID:            "ch02_claim_001",
			Text:          qmAbsoluteClaim, // verbatim repeat of the ch01 claim
			Importance:    "medium",
			ReferenceKeys: []string{setup.garciaKey},
			EvidenceIDs:   []string{"ev_002"},
		},
		{
			ID:            "ch02_claim_002",
			Text:          qmUnsupportedClaim,
			Importance:    "medium",
			ReferenceKeys: []string{setup.garciaKey},
			EvidenceIDs:   []string{"ev_002"},
		},
	}, []string{setup.garciaKey})
	if _, err := agent.WriteGuardedDraftBundle(s, ch02); err != nil {
		t.Fatalf("WriteGuardedDraftBundle(ch02) error = %v", err)
	}

	if _, _, err := quality.ExtractChapterClaimGraph(s, "ch01", 1, false, fixedTime()); err != nil {
		t.Fatalf("ExtractChapterClaimGraph(ch01) error = %v", err)
	}
	graph, _, err := quality.ExtractChapterClaimGraph(s, "ch02", 1, false, fixedTime())
	if err != nil {
		t.Fatalf("ExtractChapterClaimGraph(ch02) error = %v", err)
	}
	if len(graph.Claims) != 3 {
		t.Fatalf("claim graph size = %d, want 3", len(graph.Claims))
	}
	byText := map[string]quality.ClaimNode{}
	for _, node := range graph.Claims {
		if node.ChapterID == "ch02" {
			byText[node.Text] = node
		}
	}
	dup := byText[qmAbsoluteClaim]
	if len(dup.DuplicateOf) != 1 || dup.DuplicateOf[0] != "claim_001" {
		t.Fatalf("duplicate_of = %#v, want [claim_001]", dup)
	}

	verdicts := []quality.ClaimVerdict{
		{ClaimID: "claim_001", Support: quality.SupportSupported, RiskLevel: quality.RiskHigh,
			VerifierNote: "absolute conclusion resting on abstract-level evidence"},
		{ClaimID: dup.ID, Support: quality.SupportSupported, RiskLevel: quality.RiskLow},
		{ClaimID: byText[qmUnsupportedClaim].ID, Support: quality.SupportUnsupported,
			VerifierNote: "evidence does not cover shift workers"},
	}
	result, _, _, err := quality.SaveVerificationResult(s, verdicts, fixedTime())
	if err != nil {
		t.Fatalf("SaveVerificationResult() error = %v", err)
	}
	if len(result.Verdicts) != 3 {
		t.Fatalf("verification verdicts = %d, want 3", len(result.Verdicts))
	}

	enhanced, err := quality.RunQualityGate(s, quality.ModeEnhanced, nil, fixedTime())
	if err != nil {
		t.Fatalf("RunQualityGate(enhanced) error = %v", err)
	}
	if len(enhanced.Blockers) != 0 || enhanced.Conclusion != quality.GateNeedsRevision {
		t.Fatalf("enhanced outcome = %#v", enhanced)
	}
	shallow, ok := findGateIssue(enhanced.Findings, quality.CodeGateShallowEvidenceStrongClaim)
	if !ok || shallow.Severity != quality.SeverityWarning {
		t.Fatalf("enhanced shallow finding = %#v ok=%v, want warning", shallow, ok)
	}
	if unsupported, ok := findGateIssue(enhanced.Findings, quality.CodeGateUnsupportedClaim); !ok || unsupported.Severity != quality.SeverityNeedsRevision {
		t.Fatalf("enhanced unsupported finding = %#v ok=%v, want needs_revision", unsupported, ok)
	}
	if duplicate, ok := findGateIssue(enhanced.Findings, quality.CodeGateDuplicateClaim); !ok || duplicate.Severity != quality.SeverityWarning {
		t.Fatalf("enhanced duplicate finding = %#v ok=%v, want warning", duplicate, ok)
	}

	strict, err := quality.RunQualityGate(s, quality.ModeStrict, nil, fixedTime())
	if err != nil {
		t.Fatalf("RunQualityGate(strict) error = %v", err)
	}
	if strict.Conclusion != quality.GateNeedsRevision {
		t.Fatalf("strict conclusion = %s, want needs_revision", strict.Conclusion)
	}
	if shallowStrict, ok := findGateIssue(strict.Findings, quality.CodeGateShallowEvidenceStrongClaim); !ok || shallowStrict.Severity != quality.SeverityNeedsRevision {
		t.Fatalf("strict shallow finding = %#v ok=%v, want needs_revision", shallowStrict, ok)
	}

	fast, err := quality.RunQualityGate(s, quality.ModeFast, nil, fixedTime())
	if err != nil {
		t.Fatalf("RunQualityGate(fast) error = %v", err)
	}
	if fast.Conclusion != quality.GatePassWithWarnings || len(fast.Blockers) != 0 {
		t.Fatalf("fast outcome = %#v, want warnings-only pass", fast)
	}
	for _, finding := range fast.Findings {
		if finding.Severity != quality.SeverityWarning {
			t.Fatalf("fast finding escalated past warning: %#v", finding)
		}
	}

	// Rewrite overrun: ch02 spent 3 rounds (> MaxRevisionRounds). The gate
	// escalates to needs_human_review but returns normally - no interruption.
	overrun, err := quality.RunQualityGate(s, quality.ModeStrict, map[string]int{"ch02": 3}, fixedTime())
	if err != nil {
		t.Fatalf("RunQualityGate(overrun) error = %v", err)
	}
	if overrun.Conclusion != quality.GateNeedsHumanReview {
		t.Fatalf("overrun conclusion = %s, want needs_human_review", overrun.Conclusion)
	}
	exceeded, ok := findGateIssue(overrun.Findings, quality.CodeGateRewriteRoundsExceeded)
	if !ok || exceeded.Severity != quality.SeverityNeedsHumanReview || !exceeded.TopPriority {
		t.Fatalf("overrun finding = %#v ok=%v, want strict top-priority needs_human_review", exceeded, ok)
	}
	if len(overrun.Findings) == 0 || overrun.Findings[0].Code != quality.CodeGateRewriteRoundsExceeded {
		t.Fatalf("strict overrun is not pinned first: %#v", overrun.Findings)
	}

	if readStoreText(t, s, quality.VerificationResultJSONRel) == "" {
		t.Fatalf("verification-result.json is empty")
	}
	if readStoreText(t, s, quality.ClaimGraphJSONRel) == "" {
		t.Fatalf("claim-graph.json is empty")
	}
}

// TestQualityMiniGateHardBlocksEveryMode feeds the gate a graph that escaped
// upstream guards (stale confirmations, missing evidence) and asserts every
// mode - including fast - still blocks on the four bottom-line violations.
func TestQualityMiniGateHardBlocksEveryMode(t *testing.T) {
	input := quality.GateInput{
		Graph: quality.ClaimGraph{
			UpdatedAt: fixedTime(),
			Claims: []quality.ClaimNode{
				{ID: "claim_001", Text: "cites a forged key", ChapterID: "ch01",
					ReferenceKeys: []string{"ghost2099fabricated"}, EvidenceIDs: []string{"ev_001"},
					Support: quality.SupportSupported},
				{ID: "claim_002", Text: "has no evidence binding", ChapterID: "ch01",
					ReferenceKeys: []string{"chen2024cbti"},
					Support:       quality.SupportSupported},
				{ID: "claim_003", Text: "binds unknown evidence", ChapterID: "ch02",
					ReferenceKeys: []string{"chen2024cbti"}, EvidenceIDs: []string{"ev_999"},
					Support: quality.SupportSupported},
			},
		},
		ConfirmedKeys: map[string]bool{"chen2024cbti": true},
		EvidenceByID: map[string]quality.Evidence{
			"ev_001": {ID: "ev_001", ReferenceKey: "chen2024cbti", Depth: quality.DepthAbstract, Confidence: "medium"},
		},
	}
	wantCodes := []string{
		quality.CodeGateUnconfirmedReference,
		quality.CodeGateClaimMissingEvidence,
		quality.CodeGateUnknownEvidence,
	}
	for _, mode := range []string{quality.ModeFast, quality.ModeEnhanced, quality.ModeStrict} {
		input.Mode = mode
		outcome := quality.EvaluateQualityGate(input)
		if outcome.Conclusion != quality.GateBlocked {
			t.Fatalf("mode %s conclusion = %s, want blocked", mode, outcome.Conclusion)
		}
		for _, code := range wantCodes {
			if _, ok := findGateIssue(outcome.Blockers, code); !ok {
				t.Fatalf("mode %s missing blocker %s: %#v", mode, code, outcome.Blockers)
			}
		}
	}
}
