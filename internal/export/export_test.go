package export

import (
	"archive/zip"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/artifacts"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/store"
)

func TestExportFinalWritesMarkdownReferencesTraceReportAndDocx(t *testing.T) {
	s := store.NewAt(filepath.Join(t.TempDir(), "store"))
	input := sampleExportInput()
	now := time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)

	result, err := ExportFinal(s, input, Options{Now: now})
	if err != nil {
		t.Fatalf("ExportFinal() error = %v", err)
	}
	if !result.DocxWritten {
		t.Fatalf("DocxWritten = false, issues = %#v", result.Issues)
	}
	if len(result.Outputs) != 5 {
		t.Fatalf("outputs = %#v", result.Outputs)
	}

	paper := readFile(t, s.Path("final", "paper.md"))
	if !strings.Contains(paper, "# Generated AI Review") || !strings.Contains(paper, "Body with citation.") {
		t.Fatalf("paper.md = %s", paper)
	}
	references := readFile(t, s.Path("final", "references.md"))
	if !strings.Contains(references, "[smith2024Rag] Smith, Jane (2024). RAG for Reviews.") {
		t.Fatalf("references.md = %s", references)
	}
	report := readFile(t, s.Path("final", "report.md"))
	if !strings.Contains(report, "Export version: `export-20260607T120000Z`") || !strings.Contains(report, "| ch01 | 86 | 94 | true | 0 |") {
		t.Fatalf("report.md = %s", report)
	}
	docx, err := zip.OpenReader(s.Path("final", "paper.docx"))
	if err != nil {
		t.Fatalf("paper.docx is not a readable zip: %v", err)
	}
	if err := docx.Close(); err != nil {
		t.Fatalf("close paper.docx: %v", err)
	}

	var trace CitationTrace
	if err := store.ReadJSON(s.Path("final", "citation-trace.json"), &trace); err != nil {
		t.Fatalf("read trace: %v", err)
	}
	if len(trace.Items) != 1 {
		t.Fatalf("trace = %#v", trace)
	}
	item := trace.Items[0]
	if item.ReferenceKey != "smith2024Rag" || item.SourceType != "academic_search" || !item.EditorVerified || item.NeedsHumanReview {
		t.Fatalf("trace item = %#v", item)
	}
}

func TestExportFinalDocxFailureStillWritesReport(t *testing.T) {
	s := store.NewAt(filepath.Join(t.TempDir(), "store"))
	if _, err := ExportFinal(s, sampleExportInput(), Options{}); err != nil {
		t.Fatalf("initial ExportFinal() error = %v", err)
	}
	if _, err := os.Stat(s.Path("final", "paper.docx")); err != nil {
		t.Fatalf("initial paper.docx missing: %v", err)
	}
	failing := DocxExporterFunc(func(string) ([]byte, error) {
		return nil, errors.New("pandoc unavailable")
	})

	result, err := ExportFinal(s, sampleExportInput(), Options{DocxExporter: failing})
	if err != nil {
		t.Fatalf("ExportFinal() error = %v", err)
	}
	if result.DocxWritten || !hasIssue(result.Issues, CodeDocxFailed) {
		t.Fatalf("result = %#v", result)
	}
	if _, err := os.Stat(s.Path("final", "paper.md")); err != nil {
		t.Fatalf("paper.md missing: %v", err)
	}
	if _, err := os.Stat(s.Path("final", "paper.docx")); !os.IsNotExist(err) {
		t.Fatalf("paper.docx should not exist, err = %v", err)
	}
	report := readFile(t, s.Path("final", "report.md"))
	if !strings.Contains(report, "Docx: failed with `EXPORT_DOCX_FAILED`") || !strings.Contains(report, "pandoc unavailable") {
		t.Fatalf("report.md = %s", report)
	}
}

func TestExportFinalRejectsUnconfirmedCitationTraceReference(t *testing.T) {
	input := sampleExportInput()
	input.Chapters[0].CitationMap.Mappings[0].ReferenceKeys = []string{"missingRef"}

	_, err := ExportFinal(store.NewAt(filepath.Join(t.TempDir(), "store")), input, Options{})
	var exportErr Error
	if !errors.As(err, &exportErr) || exportErr.Code != CodeUnconfirmedReference {
		t.Fatalf("error = %v", err)
	}
}

func TestLoadInputReadsAcceptedArtifactsFromStore(t *testing.T) {
	s := store.NewAt(filepath.Join(t.TempDir(), "store"))
	input := sampleExportInput()
	if _, err := store.WriteJSON(s.RequirementsPath(), contracts.Requirements{Topic: "Fallback Title", CitationStyle: "APA"}, store.Overwrite); err != nil {
		t.Fatalf("write requirements: %v", err)
	}
	if _, err := store.WriteJSON(s.RunPath(), contracts.Run{CostEstimate: map[string]any{"tokens": 123}}, store.Overwrite); err != nil {
		t.Fatalf("write run: %v", err)
	}
	outline := map[string]any{
		"title_suggestion": "Outline Title",
		"chapters": []map[string]any{{
			"chapter_id": "ch01",
			"title":      "Intro",
		}},
		"extra_field_allowed": true,
	}
	if _, err := store.WriteJSON(s.Path("outline", "outline.json"), outline, store.Overwrite); err != nil {
		t.Fatalf("write outline: %v", err)
	}
	if _, err := store.WriteJSON(s.Path("references", "confirmed.json"), input.ConfirmedReferences, store.Overwrite); err != nil {
		t.Fatalf("write confirmed: %v", err)
	}
	bundle := artifacts.DraftBundle{
		ChapterID:     "ch01",
		Version:       1,
		DraftMarkdown: input.Chapters[0].AcceptedMarkdown,
		Claims:        input.Chapters[0].Claims,
		CitationMap:   input.Chapters[0].CitationMap,
	}
	if _, err := artifacts.WriteDraftBundle(s, bundle); err != nil {
		t.Fatalf("WriteDraftBundle() error = %v", err)
	}
	if _, err := artifacts.WriteReview(s, input.Chapters[0].Review, "ok"); err != nil {
		t.Fatalf("WriteReview() error = %v", err)
	}
	if _, err := artifacts.CommitAccepted(s, "ch01", 1, input.Chapters[0].Review); err != nil {
		t.Fatalf("CommitAccepted() error = %v", err)
	}

	loaded, err := LoadInput(s)
	if err != nil {
		t.Fatalf("LoadInput() error = %v", err)
	}
	if loaded.Title != "Outline Title" || loaded.CitationStyle != "APA" {
		t.Fatalf("loaded metadata = %#v", loaded)
	}
	if len(loaded.Chapters) != 1 || loaded.Chapters[0].Title != "Intro" || loaded.Chapters[0].Version != 1 {
		t.Fatalf("loaded chapters = %#v", loaded.Chapters)
	}
	if loaded.CostEstimate["tokens"].(float64) != 123 {
		t.Fatalf("loaded cost = %#v", loaded.CostEstimate)
	}
}

func sampleExportInput() ExportInput {
	return ExportInput{
		Title:         "Generated AI Review",
		CitationStyle: "APA",
		CostEstimate:  map[string]any{"usd": 0.12},
		ConfirmedReferences: contracts.ConfirmedReferences{Items: []contracts.ConfirmedReference{{
			Key:         "smith2024Rag",
			Title:       "RAG for Reviews",
			Authors:     []string{"Smith, Jane"},
			Year:        2024,
			DOI:         "10.1000/rag",
			ConfirmedAt: time.Date(2026, 6, 7, 10, 0, 0, 0, time.UTC),
		}}},
		Chapters: []ChapterInput{{
			ID:               "ch01",
			Title:            "Intro",
			Version:          1,
			Status:           artifacts.StatusCommitted,
			AcceptedMarkdown: "## Intro\n\nBody with citation.",
			Claims: contracts.ClaimsFile{
				ChapterID:    "ch01",
				DraftVersion: 1,
				Claims: []contracts.Claim{{
					ID:            "ch01_claim_001",
					Text:          "RAG helps review writing.",
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
			Review: contracts.Review{
				ChapterID:    "ch01",
				DraftVersion: 1,
				Scores: contracts.ReviewScores{
					Overall:             86,
					CitationConsistency: 94,
					StructureLogic:      85,
					Coverage:            84,
					Readability:         88,
				},
				Passed:            true,
				UnsupportedClaims: []string{},
			},
		}},
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", path, err)
	}
	return string(data)
}
