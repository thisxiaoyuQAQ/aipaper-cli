package agent

// Module 28 boundary additions: structured rewrite instructions on the editor
// review. The Editor emits per-problem instructions (what to change, which
// evidence to use); the Host validates them against the evidence table and
// claim graph before the review is persisted, injects the previous round's
// instructions into the rewrite Writer input, and refuses to pass a chapter
// whose required instructions are not covered. Existing artifact path/write
// semantics stay in internal/artifacts.

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/artifacts"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/quality"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/store"
)

// Structured error codes for the editor rewrite-instruction guard.
const (
	CodeInstructionInvalid           = "EDITOR_INSTRUCTION_INVALID"
	CodeInstructionUnknownEvidence   = "EDITOR_INSTRUCTION_UNKNOWN_EVIDENCE"
	CodeInstructionUnknownClaim      = "EDITOR_INSTRUCTION_UNKNOWN_CLAIM"
	CodeInstructionMissingForRewrite = "EDITOR_INSTRUCTION_MISSING_FOR_REWRITE"
)

// Rewrite instruction severities, aligned with the existing required/optional
// fixes on the review.
const (
	RewriteSeverityRequired = "required"
	RewriteSeverityOptional = "optional"
)

var reviewVersionPattern = regexp.MustCompile(`^review-v(\d+)\.json$`)

// GuardReviewInstructions hard-validates the structured rewrite instructions
// before a review is written: location/problem/instruction must be present,
// severity must be required or optional, suggested evidence ids must exist in
// the evidence table, and claim ids must exist in the claim graph. Every claim
// explicitly marked needs_rewrite, or verified as unsupported/overstated in this
// chapter, must receive at least one instruction. Reviews without instructions
// in legacy runs with no claim graph pass unchanged.
func GuardReviewInstructions(s store.Store, review contracts.Review) error {
	table, _, err := loadEvidenceTableIfPresent(s)
	if err != nil {
		return err
	}
	evidenceIDs := make(map[string]bool, len(table.Items))
	for _, ev := range table.Items {
		evidenceIDs[ev.ID] = true
	}
	graph, graphExists, err := loadClaimGraphIfPresent(s)
	if err != nil {
		return err
	}
	claimByID := make(map[string]quality.ClaimNode, len(graph.Claims))
	for _, node := range graph.Claims {
		claimByID[node.ID] = node
	}
	covered := map[string]bool{}
	for i, ins := range review.RewriteInstructions {
		at := fmt.Sprintf("rewrite_instructions[%d]", i)
		if ins.Location == "" || ins.Problem == "" || ins.Instruction == "" {
			return editorGuardError(CodeInstructionInvalid,
				fmt.Sprintf("%s: location, problem, and instruction are required", at), review.ChapterID, ins.ClaimID)
		}
		if ins.Severity != RewriteSeverityRequired && ins.Severity != RewriteSeverityOptional {
			return editorGuardError(CodeInstructionInvalid,
				fmt.Sprintf("%s: invalid severity %q (required|optional)", at, ins.Severity), review.ChapterID, ins.ClaimID)
		}
		for _, id := range ins.SuggestedEvidenceIDs {
			if !evidenceIDs[id] {
				return editorGuardError(CodeInstructionUnknownEvidence,
					fmt.Sprintf("%s: suggested evidence %s is not in the evidence table", at, id), review.ChapterID, ins.ClaimID)
			}
		}
		if ins.ClaimID != "" {
			node, found := claimByID[ins.ClaimID]
			if !found {
				return editorGuardError(CodeInstructionUnknownClaim,
					fmt.Sprintf("%s: claim %s does not exist in the claim graph", at, ins.ClaimID), review.ChapterID, ins.ClaimID)
			}
			if node.ChapterID != review.ChapterID {
				return editorGuardError(CodeInstructionUnknownClaim,
					fmt.Sprintf("%s: claim %s belongs to chapter %s, not %s", at, ins.ClaimID, node.ChapterID, review.ChapterID), review.ChapterID, ins.ClaimID)
			}
			covered[ins.ClaimID] = true
		}
	}
	if graphExists {
		for _, node := range graph.Claims {
			if node.ChapterID != review.ChapterID || !claimNeedsRewriteInstruction(node) || covered[node.ID] {
				continue
			}
			return editorGuardError(CodeInstructionMissingForRewrite,
				fmt.Sprintf("claim %s needs rewrite but the review has no instruction for it", node.ID), review.ChapterID, node.ID)
		}
	}
	return nil
}

// WriteGuardedReview runs the instruction guard and only then persists the
// review artifacts. A guard failure returns the structured error and blocks
// the review from being written.
func WriteGuardedReview(s store.Store, review contracts.Review, markdown string) (artifacts.WriteResult, error) {
	if err := GuardReviewInstructions(s, review); err != nil {
		return artifacts.WriteResult{}, err
	}
	return artifacts.WriteReview(s, review, markdown)
}

// ReviewGateWithInstructions is a module-28 compatibility wrapper around the
// artifacts chapter gate, which now treats required rewrite instructions as a
// non-passing review condition directly.
func ReviewGateWithInstructions(review contracts.Review, revisionRounds int) artifacts.GateResult {
	return artifacts.StatusAfterReview(review, revisionRounds)
}

func claimNeedsRewriteInstruction(node quality.ClaimNode) bool {
	return node.NeedsRewrite || node.Support == quality.SupportUnsupported || node.Support == quality.SupportOverstated
}

// attachRewriteInstructions injects the previous round's rewrite instructions
// into the Writer input so the rewrite addresses each one. A chapter without
// review history carries nothing; corrupt review artifacts are hard errors.
func attachRewriteInstructions(s store.Store, input *WriterChapterInput) error {
	review, ok, err := loadLatestChapterReviewIfPresent(s, input.ChapterID)
	if err != nil {
		return err
	}
	if !ok || len(review.RewriteInstructions) == 0 {
		return nil
	}
	input.RewriteInstructions = review.RewriteInstructions
	return nil
}

// loadLatestChapterReviewIfPresent scans drafts/<chapter>/ for the highest
// review-vN.json. Missing chapters or chapters without reviews return ok=false.
func loadLatestChapterReviewIfPresent(s store.Store, chapterID string) (contracts.Review, bool, error) {
	if _, err := artifacts.ReviewPath(chapterID, 1); err != nil {
		// Unsafe or empty chapter ids carry no review history.
		return contracts.Review{}, false, nil
	}
	entries, err := os.ReadDir(s.Path("drafts", chapterID))
	if errors.Is(err, os.ErrNotExist) {
		return contracts.Review{}, false, nil
	}
	if err != nil {
		return contracts.Review{}, false, AgentError(CodeToolExecutionFailed, fmt.Sprintf("scan chapter reviews: %v", err), false)
	}
	latest := 0
	for _, entry := range entries {
		match := reviewVersionPattern.FindStringSubmatch(entry.Name())
		if match == nil {
			continue
		}
		if v, err := strconv.Atoi(match[1]); err == nil && v > latest {
			latest = v
		}
	}
	if latest == 0 {
		return contracts.Review{}, false, nil
	}
	rel, err := artifacts.ReviewPath(chapterID, latest)
	if err != nil {
		return contracts.Review{}, false, err
	}
	var review contracts.Review
	if err := store.ReadJSON(s.Path(filepath.FromSlash(rel)), &review); err != nil {
		return contracts.Review{}, false, AgentError(CodeToolExecutionFailed, fmt.Sprintf("read %s: %v", rel, err), false)
	}
	return review, true, nil
}

// editorQualityPromptSections returns the Editor rewrite-instruction protocol
// for the Coordinator prompt. The semantic judgment of what to rewrite stays
// with the Editor; the Host only enforces the machine-checkable bindings.
func editorQualityPromptSections() []string {
	return []string{
		"Editor rewrite instructions: when reviewing a chapter the Editor reads the claim graph, the verification result, and the chapter quality plan, and emits structured rewrite_instructions in the review. Each instruction carries a location, the problem (unsupported, overstated, generalization, duplicate, weak evidence), a concrete instruction, suggested_evidence_ids from the evidence table, severity required or optional, and optionally the claim_id it fixes. Every claim marked needs_rewrite must receive at least one instruction.",
		"Editor instruction guard: the Host validates rewrite instructions before the review is written. Suggested evidence ids must exist in the evidence table and claim ids in the claim graph; an invalid instruction returns a structured error and blocks the review output.",
		"Rewrite round: writer_run injects the previous round's rewrite instructions into the Writer input, and the Writer must address every required instruction using the suggested evidence. A review that still carries required rewrite instructions cannot pass: the chapter enters another rewrite round, or needs_human_review past the existing two-round limit.",
	}
}

func editorGuardError(code, message, chapterID, claimID string) Error {
	err := AgentError(code, message, false)
	err.Details = map[string]any{"chapter_id": chapterID}
	if claimID != "" {
		err.Details["claim_id"] = claimID
	}
	return err
}

func loadClaimGraphIfPresent(s store.Store) (quality.ClaimGraph, bool, error) {
	if _, err := os.Stat(quality.ClaimGraphJSONPath(s)); os.IsNotExist(err) {
		return quality.ClaimGraph{}, false, nil
	}
	graph, err := quality.LoadClaimGraph(s)
	if err != nil {
		return quality.ClaimGraph{}, false, AgentError(CodeToolExecutionFailed, fmt.Sprintf("load claim graph: %v", err), false)
	}
	return graph, true, nil
}
