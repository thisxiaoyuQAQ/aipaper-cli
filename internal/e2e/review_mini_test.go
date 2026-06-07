package e2e

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/app"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/artifacts"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/checkpoint"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/config"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
	finalexport "github.com/thisxiaoyuQAQ/aipaper-cli/internal/export"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/materials"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/references"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/search"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/store"
)

type mockProvider struct{}

func (mockProvider) Name() string { return "mock_search" }

func (mockProvider) Search(context.Context, search.Query) ([]contracts.ReferenceCandidate, error) {
	return []contracts.ReferenceCandidate{{
		Title:           "Human Confirmation for AI Literature Reviews",
		Authors:         []string{"Lee, Kai"},
		Year:            2025,
		Source:          "mock_search",
		DOI:             "10.1000/human-confirmation",
		Abstract:        "Human confirmation reduces unsupported citations in AI assisted reviews.",
		RelevanceScore:  0.91,
		RelevanceReason: "Covers citation confirmation and review quality gates.",
		Status:          "pending",
	}}, nil
}

func TestE2EReviewMini(t *testing.T) {
	workDir := t.TempDir()
	if _, err := app.Bootstrap(workDir, config.Config{Provider: "mock", Model: "review-mini"}); err != nil {
		t.Fatalf("Bootstrap() error = %v", err)
	}
	s := store.New(workDir)
	req := reviewMiniRequirements()
	if _, err := store.WriteJSON(s.RequirementsPath(), req, store.Overwrite); err != nil {
		t.Fatalf("write requirements: %v", err)
	}

	materialResult, err := materials.ProcessDir(req.MaterialDir, s)
	if err != nil {
		t.Fatalf("ProcessDir() error = %v", err)
	}
	if len(materialResult.Manifest.Items) != 3 || len(materialResult.Candidates) != 1 {
		t.Fatalf("material result = %#v", materialResult)
	}

	searchResult, err := search.Run(context.Background(), s, search.Options{
		Requirements: req,
		Providers:    []search.Provider{mockProvider{}},
		Limit:        2,
	})
	if err != nil {
		t.Fatalf("search.Run() error = %v", err)
	}
	if len(searchResult.Candidates.Items) != 1 {
		t.Fatalf("search candidates = %#v", searchResult.Candidates.Items)
	}

	combinedCandidates := combineCandidates(materialResult.Candidates, searchResult.Candidates.Items)
	confirmed := confirmAllReferences(t, s, combinedCandidates)
	if len(confirmed.Items) != 2 {
		t.Fatalf("confirmed = %#v", confirmed)
	}
	referenceKey := confirmed.Items[0].Key
	writeOutline(t, s)

	state, err := artifacts.NewChapterState("ch01")
	if err != nil {
		t.Fatalf("NewChapterState() error = %v", err)
	}
	state, err = state.AfterDraft(1)
	if err != nil {
		t.Fatalf("AfterDraft(v1) error = %v", err)
	}
	v1Draft := mockDraftBundle("ch01", 1, referenceKey, "The first draft needs more citation discipline.")
	v1Write, err := artifacts.WriteDraftBundle(s, v1Draft)
	if err != nil {
		t.Fatalf("WriteDraftBundle(v1) error = %v", err)
	}
	lowReview := mockReview("ch01", 1, false, 62, 88)
	v1Review, err := artifacts.WriteReview(s, lowReview, "Needs rewrite for quality and citation consistency.")
	if err != nil {
		t.Fatalf("WriteReview(v1) error = %v", err)
	}
	state, gate, err := state.AfterReview(lowReview)
	if err != nil {
		t.Fatalf("AfterReview(v1) error = %v", err)
	}
	if gate.Status != artifacts.StatusRevisionRequired {
		t.Fatalf("gate = %#v", gate)
	}
	recordCheckpoint(t, s, 5, "review_chapter", append(v1Write.Outputs, v1Review.Outputs...), "rewrite_chapter", contracts.Progress{
		Phase:          "review_chapter",
		Status:         "running",
		CurrentChapter: "ch01",
		PendingChapters: []string{
			"ch01",
		},
		UpdatedAt: fixedTime(),
	})

	midRecover, err := app.Recover(workDir)
	if err != nil {
		t.Fatalf("Recover(mid) error = %v", err)
	}
	if !midRecover.OK || midRecover.ResumedFrom != 5 || !strings.Contains(midRecover.RecoveryPrompt, "next_expected=rewrite_chapter") {
		t.Fatalf("mid recover = %#v", midRecover)
	}

	state, err = state.NextDraft()
	if err != nil {
		t.Fatalf("NextDraft() error = %v", err)
	}
	state, err = state.AfterDraft(2)
	if err != nil {
		t.Fatalf("AfterDraft(v2) error = %v", err)
	}
	v2Draft := mockDraftBundle("ch01", 2, referenceKey, "The revised version grounds the review in confirmed references.")
	if _, err := artifacts.WriteDraftBundle(s, v2Draft); err != nil {
		t.Fatalf("WriteDraftBundle(v2) error = %v", err)
	}
	passReview := mockReview("ch01", 2, true, 86, 94)
	if _, err := artifacts.WriteReview(s, passReview, "Accepted after rewrite."); err != nil {
		t.Fatalf("WriteReview(v2) error = %v", err)
	}
	state, gate, err = state.AfterReview(passReview)
	if err != nil {
		t.Fatalf("AfterReview(v2) error = %v", err)
	}
	if !gate.Passed || state.Status != artifacts.StatusAccepted {
		t.Fatalf("accepted state = %#v gate = %#v", state, gate)
	}
	accepted, err := artifacts.CommitAccepted(s, "ch01", 2, passReview)
	if err != nil {
		t.Fatalf("CommitAccepted() error = %v", err)
	}
	state, err = state.Commit()
	if err != nil {
		t.Fatalf("Commit state error = %v", err)
	}
	if state.Status != artifacts.StatusCommitted {
		t.Fatalf("state = %#v", state)
	}
	if accepted.Path != "drafts/ch01/accepted.md" {
		t.Fatalf("accepted = %#v", accepted)
	}

	exportInput, err := finalexport.LoadInput(s)
	if err != nil {
		t.Fatalf("LoadInput() error = %v", err)
	}
	exportResult, err := finalexport.ExportFinal(s, exportInput, finalexport.Options{Now: fixedTime()})
	if err != nil {
		t.Fatalf("ExportFinal() error = %v", err)
	}
	if !exportResult.DocxWritten || len(exportResult.Outputs) != 5 {
		t.Fatalf("export result = %#v", exportResult)
	}
	recordCheckpoint(t, s, 9, "export_docx", exportResult.Outputs, "complete", contracts.Progress{
		Phase:             "export_docx",
		Status:            "completed",
		CurrentChapter:    "",
		CompletedChapters: []string{"ch01"},
		PendingChapters:   []string{},
		UpdatedAt:         fixedTime(),
	})

	validation := checkpoint.ValidateLatest(s)
	if !validation.OK || validation.Step != 9 || validation.Next != "complete" {
		t.Fatalf("latest validation = %#v", validation)
	}
	assertFinalArtifacts(t, s, confirmed)
}

func reviewMiniRequirements() contracts.Requirements {
	return contracts.Requirements{
		Topic:             "Retrieval augmented generation for literature review writing",
		ResearchQuestions: []string{"How should confirmed references constrain generated reviews?"},
		Scope:             "mini review",
		Language:          "en",
		CitationStyle:     "APA",
		TargetWords:       800,
		MaterialDir:       "../../fixtures/review-mini/materials",
		AllowOnlineSearch: true,
		SearchProviders:   []string{"mock_search"},
	}
}

func combineCandidates(materialCandidates, searchCandidates []contracts.ReferenceCandidate) contracts.ReferenceCandidates {
	all := append([]contracts.ReferenceCandidate{}, materialCandidates...)
	all = append(all, searchCandidates...)
	return contracts.ReferenceCandidates{Items: references.AssignCandidateIDs(references.DedupeCandidates(all), 1)}
}

func confirmAllReferences(t *testing.T, s store.Store, candidates contracts.ReferenceCandidates) contracts.ConfirmedReferences {
	t.Helper()
	if _, err := store.WriteJSON(s.Path("references", "candidates.json"), candidates, store.Overwrite); err != nil {
		t.Fatalf("write combined candidates: %v", err)
	}
	var ids []string
	for _, candidate := range candidates.Items {
		ids = append(ids, candidate.ID)
	}
	result, err := references.ConfirmCandidates(s, candidates, references.ConfirmationDecision{
		ConfirmedIDs: ids,
		ConfirmedAt:  fixedTime(),
	})
	if err != nil {
		t.Fatalf("ConfirmCandidates() error = %v", err)
	}
	return result.Confirmed
}

func writeOutline(t *testing.T, s store.Store) {
	t.Helper()
	outline := map[string]any{
		"title_suggestion": "RAG Review Mini",
		"chapters": []map[string]any{{
			"chapter_id": "ch01",
			"title":      "Confirmed Evidence",
		}},
	}
	if _, err := store.WriteJSON(s.Path("outline", "outline.json"), outline, store.Overwrite); err != nil {
		t.Fatalf("write outline.json: %v", err)
	}
	if _, err := store.WriteFile(s.Path("outline", "outline.md"), []byte("# RAG Review Mini\n"), store.Overwrite); err != nil {
		t.Fatalf("write outline.md: %v", err)
	}
}

func mockDraftBundle(chapterID string, version int, referenceKey, body string) artifacts.DraftBundle {
	claimID := chapterID + "_claim_001"
	return artifacts.DraftBundle{
		ChapterID:     chapterID,
		Version:       version,
		DraftMarkdown: "## Confirmed Evidence\n\n" + body,
		Claims: contracts.ClaimsFile{
			ChapterID:    chapterID,
			DraftVersion: version,
			Claims: []contracts.Claim{{
				ID:            claimID,
				Text:          "Confirmed references constrain generated literature reviews.",
				Importance:    "high",
				ReferenceKeys: []string{referenceKey},
				Confidence:    0.86,
			}},
		},
		CitationMap: contracts.CitationMap{
			ChapterID:    chapterID,
			DraftVersion: version,
			Mappings: []contracts.CitationMapping{{
				ParagraphID:   chapterID + "_p001",
				ClaimIDs:      []string{claimID},
				ReferenceKeys: []string{referenceKey},
			}},
		},
		WriterNotes: "mock writer output",
	}
}

func mockReview(chapterID string, version int, passed bool, overall, citationConsistency int) contracts.Review {
	return contracts.Review{
		ChapterID:    chapterID,
		DraftVersion: version,
		Scores: contracts.ReviewScores{
			Overall:             overall,
			CitationConsistency: citationConsistency,
			StructureLogic:      overall,
			Coverage:            overall,
			Readability:         overall,
		},
		Passed:            passed,
		UnsupportedClaims: []string{},
		RequiredFixes:     []string{},
		OptionalFixes:     []string{},
	}
}

func recordCheckpoint(t *testing.T, s store.Store, step int, phase string, outputs []checkpoint.OutputArtifact, next string, progress contracts.Progress) {
	t.Helper()
	cp := checkpoint.Checkpoint{
		Step:         step,
		Phase:        phase,
		CreatedAt:    fixedTime(),
		Outputs:      outputs,
		NextExpected: next,
	}
	if err := checkpoint.Record(s, cp, progress); err != nil {
		t.Fatalf("Record(step=%d) error = %v", step, err)
	}
}

func assertFinalArtifacts(t *testing.T, s store.Store, confirmed contracts.ConfirmedReferences) {
	t.Helper()
	paper := readStoreText(t, s, "final/paper.md")
	if !strings.Contains(paper, "RAG Review Mini") || !strings.Contains(paper, "revised version") {
		t.Fatalf("paper.md = %s", paper)
	}
	report := readStoreText(t, s, "final/report.md")
	if !strings.Contains(report, "Quality Summary") || !strings.Contains(report, "| ch01 | 86 | 94 | true | 0 |") {
		t.Fatalf("report.md = %s", report)
	}

	var trace finalexport.CitationTrace
	if err := store.ReadJSON(s.Path("final", "citation-trace.json"), &trace); err != nil {
		t.Fatalf("read citation trace: %v", err)
	}
	if len(trace.Items) != 1 {
		t.Fatalf("trace = %#v", trace)
	}
	confirmedKeys := map[string]bool{}
	for _, ref := range confirmed.Items {
		confirmedKeys[ref.Key] = true
	}
	if !confirmedKeys[trace.Items[0].ReferenceKey] {
		t.Fatalf("trace contains unconfirmed key: %#v", trace.Items[0])
	}
	if trace.Items[0].NeedsHumanReview {
		t.Fatalf("trace item unexpectedly needs review: %#v", trace.Items[0])
	}
	if readStoreText(t, s, "final/references.md") == "" {
		t.Fatalf("references.md is empty")
	}
	if data := readStoreText(t, s, "references/confirmed.bib"); !strings.Contains(data, "@article") {
		t.Fatalf("confirmed.bib = %s", data)
	}
}

func readStoreText(t *testing.T, s store.Store, relPath string) string {
	t.Helper()
	data, err := os.ReadFile(s.Path(relPath))
	if err != nil {
		t.Fatalf("read %s: %v", relPath, err)
	}
	return string(data)
}

func fixedTime() time.Time {
	return time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)
}
