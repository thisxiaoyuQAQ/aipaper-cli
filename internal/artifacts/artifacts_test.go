package artifacts

import (
	"os"
	"strings"
	"testing"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/store"
)

func TestWriteDraftBundleWritesVersionedArtifactsAndDetectsConflict(t *testing.T) {
	s := store.NewAt(t.TempDir())
	bundle := sampleDraftBundle()

	result, err := WriteDraftBundle(s, bundle)
	if err != nil {
		t.Fatalf("WriteDraftBundle() error = %v", err)
	}
	if len(result.Outputs) != 4 {
		t.Fatalf("outputs = %#v", result.Outputs)
	}
	for _, rel := range []string{
		"drafts/ch01/draft-v1.md",
		"drafts/ch01/claims-v1.json",
		"drafts/ch01/citation-map-v1.json",
		"drafts/ch01/writer_notes.md",
	} {
		if _, err := os.Stat(s.Path(rel)); err != nil {
			t.Fatalf("stat %s: %v", rel, err)
		}
	}

	if _, err := WriteDraftBundle(s, bundle); err != nil {
		t.Fatalf("WriteDraftBundle(idempotent) error = %v", err)
	}
	bundle.DraftMarkdown = "changed"
	_, err = WriteDraftBundle(s, bundle)
	if err == nil || !strings.Contains(err.Error(), "artifact conflict") {
		t.Fatalf("WriteDraftBundle(conflict) error = %v", err)
	}
}

func TestWriteReviewAndCommitAccepted(t *testing.T) {
	s := store.NewAt(t.TempDir())
	if _, err := WriteDraftBundle(s, sampleDraftBundle()); err != nil {
		t.Fatalf("WriteDraftBundle() error = %v", err)
	}
	review := passingReview()

	result, err := WriteReview(s, review, "looks good")
	if err != nil {
		t.Fatalf("WriteReview() error = %v", err)
	}
	if len(result.Outputs) != 2 {
		t.Fatalf("review outputs = %#v", result.Outputs)
	}
	accepted, err := CommitAccepted(s, "ch01", 1, review)
	if err != nil {
		t.Fatalf("CommitAccepted() error = %v", err)
	}
	if accepted.Path != "drafts/ch01/accepted.md" {
		t.Fatalf("accepted = %#v", accepted)
	}

	failing := review
	failing.Passed = false
	if _, err := CommitAccepted(s, "ch01", 1, failing); err == nil {
		t.Fatalf("CommitAccepted(failing review) unexpectedly succeeded")
	}
}

func TestQualityGateAndChapterStateTransitions(t *testing.T) {
	state, err := NewChapterState("ch01")
	if err != nil {
		t.Fatalf("NewChapterState() error = %v", err)
	}
	state, err = state.AfterDraft(1)
	if err != nil {
		t.Fatalf("AfterDraft() error = %v", err)
	}
	low := passingReview()
	low.Passed = false
	low.Scores.Overall = 60

	state, gate, err := state.AfterReview(low)
	if err != nil {
		t.Fatalf("AfterReview(low) error = %v", err)
	}
	if gate.Passed || state.Status != StatusRevisionRequired || state.RevisionRounds != 1 {
		t.Fatalf("low review state = %#v gate = %#v", state, gate)
	}

	state, err = state.NextDraft()
	if err != nil {
		t.Fatalf("NextDraft() error = %v", err)
	}
	state, err = state.AfterDraft(2)
	if err != nil {
		t.Fatalf("AfterDraft(v2) error = %v", err)
	}
	state.RevisionRounds = MaxRevisionRounds
	low.DraftVersion = 2
	state, gate, err = state.AfterReview(low)
	if err != nil {
		t.Fatalf("AfterReview(max rounds) error = %v", err)
	}
	if state.Status != StatusNeedsHumanReview || gate.Status != StatusNeedsHumanReview {
		t.Fatalf("max rounds state = %#v gate = %#v", state, gate)
	}

	state = ChapterState{ChapterID: "ch01", Status: StatusReviewing, DraftVersion: 1}
	state, gate, err = state.AfterReview(passingReview())
	if err != nil {
		t.Fatalf("AfterReview(pass) error = %v", err)
	}
	if !gate.Passed || state.Status != StatusAccepted {
		t.Fatalf("pass state = %#v gate = %#v", state, gate)
	}
	state, err = state.Commit()
	if err != nil {
		t.Fatalf("Commit() error = %v", err)
	}
	if state.Status != StatusCommitted {
		t.Fatalf("committed state = %#v", state)
	}
}

func TestValidateDraftArtifactsReportsMissingAndUnconfirmedReferences(t *testing.T) {
	issues := ValidateDraftArtifacts(contracts.ClaimsFile{}, contracts.CitationMap{}, contracts.ConfirmedReferences{})
	if !hasIssue(issues, CodeMissingClaims) || !hasIssue(issues, CodeMissingCitationMap) {
		t.Fatalf("missing artifact issues = %#v", issues)
	}

	claims := contracts.ClaimsFile{
		ChapterID:    "ch01",
		DraftVersion: 1,
		Claims: []contracts.Claim{{
			ID:            "ch01_claim_001",
			Text:          "Confirmed refs matter.",
			Importance:    "high",
			ReferenceKeys: []string{"missingKey"},
		}},
	}
	citationMap := contracts.CitationMap{
		ChapterID:    "ch01",
		DraftVersion: 1,
		Mappings: []contracts.CitationMapping{{
			ParagraphID:   "ch01_p001",
			ClaimIDs:      []string{"unknown_claim"},
			ReferenceKeys: []string{"missingKey"},
		}},
	}
	issues = ValidateDraftArtifacts(claims, citationMap, contracts.ConfirmedReferences{Items: []contracts.ConfirmedReference{{Key: "confirmedKey"}}})
	if !hasIssue(issues, CodeUnconfirmedRef) || !hasIssue(issues, CodeUnknownClaim) {
		t.Fatalf("validation issues = %#v", issues)
	}
}

func sampleDraftBundle() DraftBundle {
	return DraftBundle{
		ChapterID:     "ch01",
		Version:       1,
		DraftMarkdown: "# Chapter 1\nBody.",
		Claims: contracts.ClaimsFile{
			ChapterID:    "ch01",
			DraftVersion: 1,
			Claims: []contracts.Claim{{
				ID:            "ch01_claim_001",
				Text:          "Confirmed refs matter.",
				Importance:    "high",
				ReferenceKeys: []string{"smith2024Rag"},
			}},
		},
		CitationMap: contracts.CitationMap{
			ChapterID:    "ch01",
			DraftVersion: 1,
			Mappings: []contracts.CitationMapping{{
				ParagraphID:   "ch01_p001",
				ClaimIDs:      []string{"ch01_claim_001"},
				ReferenceKeys: []string{"smith2024Rag"},
			}},
		},
		WriterNotes: "No gaps.",
	}
}

func passingReview() contracts.Review {
	return contracts.Review{
		ChapterID:    "ch01",
		DraftVersion: 1,
		Scores: contracts.ReviewScores{
			Overall:             85,
			CitationConsistency: 95,
			StructureLogic:      82,
			Coverage:            83,
			Readability:         84,
		},
		Passed:            true,
		UnsupportedClaims: []string{},
		RequiredFixes:     []string{},
		OptionalFixes:     []string{},
	}
}

func hasIssue(issues []ValidationIssue, code string) bool {
	for _, issue := range issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}
