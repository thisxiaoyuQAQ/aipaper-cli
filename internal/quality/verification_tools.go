package quality

import (
	"context"
	"encoding/json"
	"time"

	"github.com/voocel/agentcore"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/store"
)

// NewSaveVerificationResultTool persists verifier verdicts: it validates them
// against the claim graph, writes support/risk/notes back onto the graph, and
// saves quality/verification-result.json (per-claim merge, stale verdicts for
// re-extracted chapters pruned).
func NewSaveVerificationResultTool(s store.Store) agentcore.Tool {
	return jsonTool{
		name:        "save_verification_result",
		description: "Validate and save claim verification verdicts. Each verdict needs claim_id (must exist in the claim graph) and support (supported|partially_supported|unsupported|overstated|skipped; uncertainty must be marked unsupported, never left empty). Optional risk_level and verifier_note. Verdicts are written back onto the claim graph and merged into quality/verification-result.json.",
		schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"verdicts": map[string]any{
					"type":        "array",
					"description": "ClaimVerdict list: [{claim_id, support, risk_level?, verifier_note?}]",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"claim_id":      map[string]any{"type": "string"},
							"support":       map[string]any{"type": "string"},
							"risk_level":    map[string]any{"type": "string"},
							"verifier_note": map[string]any{"type": "string"},
						},
						"required": []string{"claim_id", "support"},
					},
				},
			},
			"required": []string{"verdicts"},
		},
		execute: func(_ context.Context, args json.RawMessage) ToolResponse {
			var payload struct {
				Verdicts []ClaimVerdict `json:"verdicts"`
			}
			if err := strictUnmarshal(args, &payload); err != nil {
				return toolError(NewError(CodeVerificationInvalid, "invalid save_verification_result arguments: "+err.Error(), false))
			}
			result, graph, outputs, err := SaveVerificationResult(s, payload.Verdicts, time.Now().UTC())
			if err != nil {
				if qErr, ok := AsError(err); ok {
					return toolError(qErr)
				}
				return toolError(NewError(CodeVerificationIOFailed, err.Error(), false))
			}
			return ToolResponse{OK: true, Data: map[string]any{
				"outputs":      outputs,
				"verdicts":     len(result.Verdicts),
				"total_claims": len(graph.Claims),
			}}
		},
	}
}

// NewQualityGateCheckTool runs the deterministic quality gate over the claim
// graph, evidence table, and confirmed references. Pure Host logic: the mode
// only changes severity escalation, never the artifact structure.
func NewQualityGateCheckTool(s store.Store) agentcore.Tool {
	return jsonTool{
		name:        "quality_gate_check",
		description: "Run the Host quality gate over the claim graph and verification verdicts. Input: quality mode (fast|enhanced|strict, default enhanced) and optional per-chapter rewrite round counts. Output: pass | pass_with_warnings | needs_revision | needs_human_review | blocked with structured issues. Hard blocks (unconfirmed or fabricated reference keys, claims without evidence, evidence pointing at unconfirmed references) fire in every mode.",
		schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"mode": map[string]any{
					"type":        "string",
					"description": "fast | enhanced | strict; empty defaults to enhanced",
				},
				"rewrite_rounds": map[string]any{
					"type":        "object",
					"description": "chapter_id -> rewrite rounds already spent",
				},
			},
		},
		execute: func(_ context.Context, args json.RawMessage) ToolResponse {
			var payload struct {
				Mode          string         `json:"mode"`
				RewriteRounds map[string]int `json:"rewrite_rounds"`
			}
			if err := strictUnmarshal(args, &payload); err != nil {
				return toolError(NewError(CodeGateInvalidMode, "invalid quality_gate_check arguments: "+err.Error(), false))
			}
			outcome, err := RunQualityGate(s, payload.Mode, payload.RewriteRounds, time.Now().UTC())
			if err != nil {
				if qErr, ok := AsError(err); ok {
					return toolError(qErr)
				}
				return toolError(NewError(CodeClaimGraphIOFailed, err.Error(), false))
			}
			return ToolResponse{OK: true, Data: outcome}
		},
	}
}
