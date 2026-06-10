package quality

import (
	"context"
	"encoding/json"
	"time"

	"github.com/voocel/agentcore"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/store"
)

// NewSaveClaimGraphTool persists a validated claim graph to the store.
func NewSaveClaimGraphTool(s store.Store) agentcore.Tool {
	return jsonTool{
		name:        "save_claim_graph",
		description: "Validate and atomically save the claim graph to quality/claim-graph.json and .md. Every chapter_id must match an outline chapter, every reference_key must be confirmed, and every evidence id must exist in the evidence table.",
		schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"graph": map[string]any{
					"type":        "object",
					"description": "ClaimGraph JSON: {updated_at: RFC3339 UTC, claims: [ClaimNode]}",
				},
			},
			"required": []string{"graph"},
		},
		execute: func(_ context.Context, args json.RawMessage) ToolResponse {
			var payload struct {
				Graph ClaimGraph `json:"graph"`
			}
			if err := strictUnmarshal(args, &payload); err != nil {
				return toolError(NewError(CodeClaimGraphInvalid, "invalid save_claim_graph arguments: "+err.Error(), false))
			}
			outputs, err := SaveClaimGraph(s, payload.Graph)
			if err != nil {
				if qErr, ok := AsError(err); ok {
					return toolError(qErr)
				}
				return toolError(NewError(CodeClaimGraphIOFailed, err.Error(), false))
			}
			return ToolResponse{OK: true, Data: map[string]any{
				"outputs": outputs,
				"claims":  len(payload.Graph.Claims),
			}}
		},
	}
}

// NewLoadClaimGraphTool reads the claim graph from the store.
func NewLoadClaimGraphTool(s store.Store) agentcore.Tool {
	return jsonTool{
		name:        "load_claim_graph",
		description: "Load quality/claim-graph.json from the Store and return the structured claim graph.",
		schema: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		execute: func(context.Context, json.RawMessage) ToolResponse {
			graph, err := LoadClaimGraph(s)
			if err != nil {
				if qErr, ok := AsError(err); ok {
					return toolError(qErr)
				}
				return toolError(NewError(CodeClaimGraphIOFailed, err.Error(), false))
			}
			return ToolResponse{OK: true, Data: graph}
		},
	}
}

// NewExtractChapterClaimsTool runs the deterministic claim_extraction step for
// one chapter: it projects the chapter claims.json + citation_map.json into
// claim nodes, merges them into the existing graph chapter-by-chapter, and
// saves the validated result. fast_mode marks support as skipped.
func NewExtractChapterClaimsTool(s store.Store) agentcore.Tool {
	return jsonTool{
		name:        "extract_chapter_claims",
		description: "Project one chapter's claims.json and citation-map.json into the claim graph (chapter-level incremental merge with cross-chapter duplicate detection) and save quality/claim-graph.json and .md. Set fast_mode to mark support as skipped.",
		schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"chapter_id":    map[string]any{"type": "string"},
				"draft_version": map[string]any{"type": "integer", "description": "chapter draft version, >= 1"},
				"fast_mode":     map[string]any{"type": "boolean"},
			},
			"required": []string{"chapter_id", "draft_version"},
		},
		execute: func(_ context.Context, args json.RawMessage) ToolResponse {
			var payload struct {
				ChapterID    string `json:"chapter_id"`
				DraftVersion int    `json:"draft_version"`
				FastMode     bool   `json:"fast_mode"`
			}
			if err := strictUnmarshal(args, &payload); err != nil {
				return toolError(NewError(CodeClaimGraphInvalid, "invalid extract_chapter_claims arguments: "+err.Error(), false))
			}
			graph, outputs, err := ExtractChapterClaimGraph(s, payload.ChapterID, payload.DraftVersion, payload.FastMode, time.Now().UTC())
			if err != nil {
				if qErr, ok := AsError(err); ok {
					return toolError(qErr)
				}
				return toolError(NewError(CodeClaimGraphIOFailed, err.Error(), false))
			}
			duplicates := 0
			chapterClaims := 0
			for _, node := range graph.Claims {
				if len(node.DuplicateOf) > 0 {
					duplicates++
				}
				if node.ChapterID == payload.ChapterID {
					chapterClaims++
				}
			}
			return ToolResponse{OK: true, Data: map[string]any{
				"outputs":          outputs,
				"chapter_id":       payload.ChapterID,
				"chapter_claims":   chapterClaims,
				"total_claims":     len(graph.Claims),
				"duplicate_claims": duplicates,
			}}
		},
	}
}
