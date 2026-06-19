package export

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/quality"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/store"
)

func TestRenderQualityReportWithFullArtifacts(t *testing.T) {
	input := sampleQualityInput()
	report := RenderQualityReport(input, fixedTime())
	if !strings.Contains(report, "# Quality Report") {
		t.Errorf("missing header")
	}
	if !strings.Contains(report, "Quality mode: `enhanced`") {
		t.Errorf("missing mode, got: %s", report)
	}
	if !strings.Contains(report, "Paper Quality policy: `"+quality.PaperQualityPolicyVersion+"`") {
		t.Errorf("missing policy version, got: %s", report)
	}
	if !strings.Contains(report, "Evidence depth meaning") || !strings.Contains(report, "metadata_only") {
		t.Errorf("missing evidence depth explanation, got: %s", report)
	}
	if !strings.Contains(report, "Overall quality status: `pass_with_warnings`") {
		t.Errorf("missing conclusion, got: %s", report)
	}
	if !strings.Contains(report, "## Hard Gate Summary") {
		t.Errorf("missing hard gate section")
	}
	if !strings.Contains(report, "## Evidence Depth Distribution") {
		t.Errorf("missing evidence depth section")
	}
	if !strings.Contains(report, "## Claim Support Summary") {
		t.Errorf("missing support summary")
	}
}

func TestExportWithQualityArtifactsWritesQualityReport(t *testing.T) {
	s := store.NewAt(filepath.Join(t.TempDir(), "store"))
	input := sampleExportInput()
	input.Quality = sampleQualityInput().Quality
	result, err := ExportFinal(s, input, Options{Now: fixedTime()})
	if err != nil {
		t.Fatalf("ExportFinal() error = %v", err)
	}
	if !result.DocxWritten {
		t.Fatalf("DocxWritten = false")
	}
	if len(result.Outputs) != 6 {
		t.Fatalf("outputs = %d, want 6 (paper/docx/refs/trace/quality-report/report)", len(result.Outputs))
	}
	qualityReport := readFile(t, s.Path("final", "quality-report.md"))
	if !strings.Contains(qualityReport, "# Quality Report") || !strings.Contains(qualityReport, "enhanced") {
		t.Fatalf("quality-report.md = %s", qualityReport)
	}
	report := readFile(t, s.Path("final", "report.md"))
	if !strings.Contains(report, "## Quality Summary") || !strings.Contains(report, "Mode: `enhanced`") {
		t.Fatalf("report.md missing quality summary: %s", report)
	}
	if !strings.Contains(report, "Quality report: `final/quality-report.md`") {
		t.Fatalf("report.md missing quality report link: %s", report)
	}
}

func TestExportWithoutQualityArtifactsShowsCompatibilityMode(t *testing.T) {
	s := store.NewAt(filepath.Join(t.TempDir(), "store"))
	input := sampleExportInput()
	input.Quality.Available = false
	input.Quality.MissingArtifacts = []string{"quality/evidence-table.json", "quality/claim-graph.json"}
	result, err := ExportFinal(s, input, Options{Now: fixedTime()})
	if err != nil {
		t.Fatalf("ExportFinal() error = %v", err)
	}
	if len(result.Outputs) != 5 {
		t.Fatalf("outputs = %d, want 5 (no quality-report)", len(result.Outputs))
	}
	if !hasIssue(result.Issues, CodeQualityArtifactsMissing) {
		t.Fatalf("expected quality artifacts missing issue, got: %#v", result.Issues)
	}
	report := readFile(t, s.Path("final", "report.md"))
	if !strings.Contains(report, "compatibility mode") {
		t.Fatalf("report.md missing compatibility mode: %s", report)
	}
}

func TestQualityReportRendererFailureDoesNotBlockExport(t *testing.T) {
	s := store.NewAt(filepath.Join(t.TempDir(), "store"))
	input := sampleExportInput()
	input.Quality = sampleQualityInput().Quality
	failingRenderer := QualityReportRendererFunc(func(ExportInput, time.Time) (string, error) {
		return "", errors.New("renderer unavailable")
	})
	result, err := ExportFinal(s, input, Options{Now: fixedTime(), QualityReportRenderer: failingRenderer})
	if err != nil {
		t.Fatalf("ExportFinal() error = %v", err)
	}
	if len(result.Outputs) != 5 {
		t.Fatalf("outputs = %d, want 5 (quality-report not written)", len(result.Outputs))
	}
	if !hasIssue(result.Issues, CodeQualityReportFailed) {
		t.Fatalf("expected quality report failed issue, got: %#v", result.Issues)
	}
	report := readFile(t, s.Path("final", "report.md"))
	if !strings.Contains(report, "generation failed") {
		t.Fatalf("report.md missing failure note: %s", report)
	}
}

func sampleQualityInput() ExportInput {
	input := sampleExportInput()
	table := quality.EvidenceTable{
		GeneratedAt: fixedTime(),
		Items: []quality.Evidence{{
			ID:           "ev_001",
			ReferenceKey: "smith2024Rag",
			Depth:        quality.DepthAbstract,
			Topics:       []string{"RAG"},
			KeyFindings:  []string{"Improves review quality"},
			Confidence:   quality.ConfidenceHigh,
		}},
	}
	graph := quality.ClaimGraph{
		UpdatedAt: fixedTime(),
		Claims: []quality.ClaimNode{{
			ID:            "claim_001",
			Text:          "RAG helps review writing.",
			ChapterID:     "ch01",
			ReferenceKeys: []string{"smith2024Rag"},
			EvidenceIDs:   []string{"ev_001"},
			Support:       quality.SupportSupported,
			RiskLevel:     quality.RiskLow,
		}},
	}
	verification := quality.VerificationResult{
		UpdatedAt: fixedTime(),
		Verdicts: []quality.ClaimVerdict{{
			ClaimID: "claim_001",
			Support: quality.SupportSupported,
		}},
	}
	outcome := quality.GateOutcome{
		Conclusion: quality.GatePassWithWarnings,
		Mode:       quality.ModeEnhanced,
		CheckedAt:  fixedTime(),
		Findings: []quality.GateIssue{{
			Code:     quality.CodeGateDuplicateClaim,
			ClaimID:  "claim_001",
			Message:  "claim repeats earlier claims",
			Severity: quality.SeverityWarning,
		}},
	}
	input.Quality = QualityInput{
		Available:          true,
		Mode:               quality.ModeEnhanced,
		EvidenceTable:      table,
		ClaimGraph:         graph,
		VerificationResult: verification,
		GateOutcome:        outcome,
	}
	return input
}

func fixedTime() time.Time {
	return time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)
}
