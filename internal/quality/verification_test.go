package quality

import (
	"os"
	"testing"
	"time"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/store"
)

func twoClaimGraph() ClaimGraph {
	return ClaimGraph{
		UpdatedAt: time.Date(2026, 6, 10, 13, 0, 0, 0, time.UTC),
		Claims: []ClaimNode{
			{
				ID: "claim_001", Text: "Deep models improve survey coverage.", ChapterID: "ch01",
				ReferenceKeys: []string{"doe2024deep"}, EvidenceIDs: []string{"ev_001"},
			},
			{
				ID: "claim_002", Text: "Graph methods complement deep models.", ChapterID: "ch02",
				ReferenceKeys: []string{"lee2023graph"}, EvidenceIDs: []string{"ev_002"},
				RiskLevel: RiskLow,
			},
		},
	}
}

func TestValidateVerdictsFailures(t *testing.T) {
	graph := twoClaimGraph()
	cases := []struct {
		name     string
		verdicts []ClaimVerdict
		wantCode string
	}{
		{"missing claim_id", []ClaimVerdict{{Support: SupportSupported}}, CodeVerificationInvalid},
		{"unknown claim", []ClaimVerdict{{ClaimID: "claim_404", Support: SupportSupported}}, CodeVerificationUnknownClaim},
		{"duplicate claim", []ClaimVerdict{
			{ClaimID: "claim_001", Support: SupportSupported},
			{ClaimID: "claim_001", Support: SupportUnsupported},
		}, CodeVerificationDuplicateClaim},
		{"empty support", []ClaimVerdict{{ClaimID: "claim_001"}}, CodeVerificationInvalid},
		{"invalid support", []ClaimVerdict{{ClaimID: "claim_001", Support: "definitely"}}, CodeVerificationInvalid},
		{"invalid risk", []ClaimVerdict{{ClaimID: "claim_001", Support: SupportSupported, RiskLevel: "extreme"}}, CodeVerificationInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertQualityError(t, ValidateVerdicts(graph, tc.verdicts), tc.wantCode)
		})
	}
	if err := ValidateVerdicts(graph, []ClaimVerdict{
		{ClaimID: "claim_001", Support: SupportSupported, RiskLevel: RiskHigh, VerifierNote: "ok"},
		{ClaimID: "claim_002", Support: SupportSkipped},
	}); err != nil {
		t.Fatalf("valid verdicts rejected: %v", err)
	}
}

func TestApplyVerificationWritesBackReservedFields(t *testing.T) {
	graph := twoClaimGraph()
	updated := time.Date(2026, 6, 10, 15, 0, 0, 0, time.UTC)
	applied := ApplyVerification(graph, []ClaimVerdict{
		{ClaimID: "claim_001", Support: SupportOverstated, RiskLevel: RiskHigh, VerifierNote: "stronger than evidence"},
		{ClaimID: "claim_002", Support: SupportSupported}, // empty risk keeps the graph grade
	}, updated)
	if applied.UpdatedAt != updated {
		t.Fatalf("updated_at = %v", applied.UpdatedAt)
	}
	first := applied.Claims[0]
	if first.Support != SupportOverstated || first.RiskLevel != RiskHigh || first.VerifierNote != "stronger than evidence" {
		t.Fatalf("claim_001 after apply = %#v", first)
	}
	second := applied.Claims[1]
	if second.Support != SupportSupported || second.RiskLevel != RiskLow {
		t.Fatalf("claim_002 after apply = %#v (risk grade must be kept)", second)
	}
}

func TestMergeVerificationVerdictsReplacesAndPrunes(t *testing.T) {
	graph := twoClaimGraph()
	existing := VerificationResult{
		UpdatedAt: time.Date(2026, 6, 10, 14, 0, 0, 0, time.UTC),
		Verdicts: []ClaimVerdict{
			{ClaimID: "claim_001", Support: SupportUnsupported},
			{ClaimID: "claim_099", Support: SupportSupported}, // stale: re-extraction removed it
		},
	}
	updated := time.Date(2026, 6, 10, 15, 0, 0, 0, time.UTC)
	merged := MergeVerificationVerdicts(existing, []ClaimVerdict{
		{ClaimID: "claim_001", Support: SupportSupported},
		{ClaimID: "claim_002", Support: SupportPartiallySupported},
	}, graph, updated)
	if merged.UpdatedAt != updated || len(merged.Verdicts) != 2 {
		t.Fatalf("merged = %#v", merged)
	}
	if merged.Verdicts[0].ClaimID != "claim_001" || merged.Verdicts[0].Support != SupportSupported {
		t.Fatalf("claim_001 verdict not replaced: %#v", merged.Verdicts[0])
	}
	if merged.Verdicts[1].ClaimID != "claim_002" {
		t.Fatalf("verdicts not sorted: %#v", merged.Verdicts)
	}
}

func TestSaveVerificationResultRoundTripAndCheckpointRecovery(t *testing.T) {
	s := setupClaimGraphStore(t)
	if _, err := SaveClaimGraph(s, twoClaimGraph()); err != nil {
		t.Fatalf("save graph: %v", err)
	}
	now := time.Date(2026, 6, 10, 15, 0, 0, 0, time.UTC)
	result, graph, outputs, err := SaveVerificationResult(s, []ClaimVerdict{
		{ClaimID: "claim_001", Support: SupportSupported},
	}, now)
	if err != nil {
		t.Fatalf("SaveVerificationResult() error = %v", err)
	}
	if len(outputs) != 3 || outputs[0] != VerificationResultJSONRel {
		t.Fatalf("outputs = %v", outputs)
	}
	if len(result.Verdicts) != 1 || graph.Claims[0].Support != SupportSupported {
		t.Fatalf("result = %#v graph = %#v", result, graph.Claims[0])
	}

	// Interruption: a fresh call loads the persisted result, merges the second
	// chapter's verdict, and keeps the first one.
	result, graph, _, err = SaveVerificationResult(s, []ClaimVerdict{
		{ClaimID: "claim_002", Support: SupportUnsupported, RiskLevel: RiskHigh},
	}, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("second SaveVerificationResult() error = %v", err)
	}
	if len(result.Verdicts) != 2 {
		t.Fatalf("merged verdicts = %#v", result.Verdicts)
	}
	if graph.Claims[0].Support != SupportSupported || graph.Claims[1].Support != SupportUnsupported {
		t.Fatalf("graph supports = %q %q", graph.Claims[0].Support, graph.Claims[1].Support)
	}
	loaded, err := LoadVerificationResult(s)
	if err != nil || len(loaded.Verdicts) != 2 {
		t.Fatalf("persisted result = %#v err %v", loaded, err)
	}
	persistedGraph, err := LoadClaimGraph(s)
	if err != nil || persistedGraph.Claims[1].RiskLevel != RiskHigh {
		t.Fatalf("persisted graph = %#v err %v", persistedGraph, err)
	}
}

func TestSaveVerificationResultRejectsInvalidVerdicts(t *testing.T) {
	s := setupClaimGraphStore(t)
	now := time.Date(2026, 6, 10, 15, 0, 0, 0, time.UTC)

	// Missing claim graph is an io failure.
	_, _, _, err := SaveVerificationResult(s, []ClaimVerdict{{ClaimID: "claim_001", Support: SupportSupported}}, now)
	assertQualityError(t, err, CodeClaimGraphIOFailed)

	if _, err := SaveClaimGraph(s, twoClaimGraph()); err != nil {
		t.Fatalf("save graph: %v", err)
	}
	_, _, _, err = SaveVerificationResult(s, []ClaimVerdict{{ClaimID: "claim_404", Support: SupportSupported}}, now)
	assertQualityError(t, err, CodeVerificationUnknownClaim)

	// Nothing was persisted by the failed save.
	if _, statErr := os.Stat(VerificationResultJSONPath(s)); !os.IsNotExist(statErr) {
		t.Fatalf("verification result persisted after failure: %v", statErr)
	}
}

func TestLoadVerificationResultStrictAndMissing(t *testing.T) {
	s := setupClaimGraphStore(t)
	_, err := LoadVerificationResult(s)
	assertQualityError(t, err, CodeVerificationIOFailed)

	raw := `{"updated_at":"2026-06-10T15:00:00Z","verdicts":[],"unexpected":true}`
	if _, err := store.WriteFile(VerificationResultJSONPath(s), []byte(raw), store.Overwrite); err != nil {
		t.Fatalf("write raw result: %v", err)
	}
	_, err = LoadVerificationResult(s)
	assertQualityError(t, err, CodeVerificationIOFailed)
}

func TestSaveVerificationResultToolValidatesAndPersists(t *testing.T) {
	s := setupClaimGraphStore(t)
	if _, err := SaveClaimGraph(s, twoClaimGraph()); err != nil {
		t.Fatalf("save graph: %v", err)
	}
	tool := NewSaveVerificationResultTool(s)
	response := execTool(t, tool, `{"verdicts":[{"claim_id":"claim_001","support":"supported"},{"claim_id":"claim_002","support":"unsupported","risk_level":"high","verifier_note":"no evidence covers the comparison"}]}`)
	if !response.OK {
		t.Fatalf("save tool response = %#v", response)
	}
	data, ok := response.Data.(map[string]any)
	if !ok || data["verdicts"] != float64(2) || data["total_claims"] != float64(2) {
		t.Fatalf("tool data = %#v", response.Data)
	}

	response = execTool(t, tool, `{"verdicts":[{"claim_id":"claim_404","support":"supported"}]}`)
	if response.OK || response.Error == nil || response.Error.Code != CodeVerificationUnknownClaim {
		t.Fatalf("unknown claim response = %#v", response)
	}
	response = execTool(t, tool, `{"verdicts":[],"extra":1}`)
	if response.OK || response.Error == nil || response.Error.Code != CodeVerificationInvalid {
		t.Fatalf("strict args response = %#v", response)
	}
}
