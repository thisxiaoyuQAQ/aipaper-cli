package export

import (
	"time"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/checkpoint"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/quality"
)

const (
	CodeDocxFailed              = "EXPORT_DOCX_FAILED"
	CodeNoAcceptedChapters      = "EXPORT_NO_ACCEPTED_CHAPTERS"
	CodeUnconfirmedReference    = "EXPORT_UNCONFIRMED_REFERENCE"
	CodeReferenceFormat         = "EXPORT_REFERENCE_FORMAT_WARNING"
	CodeQualityReportFailed     = "EXPORT_QUALITY_REPORT_FAILED"
	CodeQualityArtifactsMissing = "EXPORT_QUALITY_ARTIFACTS_MISSING"
)

type ExportInput struct {
	Title               string
	Language            string
	CitationStyle       string
	ArticleTemplate     string
	Chapters            []ChapterInput
	ConfirmedReferences contracts.ConfirmedReferences
	CostEstimate        map[string]any
	Quality             QualityInput
}

type QualityInput struct {
	Available          bool
	MissingArtifacts   []string
	LoadError          string
	Mode               string
	EvidenceTable      quality.EvidenceTable
	SectionQualityPlan quality.SectionQualityPlan
	ClaimGraph         quality.ClaimGraph
	VerificationResult quality.VerificationResult
	GateOutcome        quality.GateOutcome
}

type ChapterInput struct {
	ID               string
	Title            string
	Version          int
	Status           string
	AcceptedMarkdown string
	Claims           contracts.ClaimsFile
	CitationMap      contracts.CitationMap
	Review           contracts.Review
}

type Options struct {
	Now                   time.Time
	DocxExporter          DocxExporter
	QualityReportRenderer QualityReportRenderer
}

type Result struct {
	Version     string
	Outputs     []checkpoint.OutputArtifact
	Issues      []Issue
	DocxWritten bool
	Metadata    map[string]any
}

type Issue struct {
	Code         string `json:"code"`
	Message      string `json:"message"`
	ChapterID    string `json:"chapter_id,omitempty"`
	ClaimID      string `json:"claim_id,omitempty"`
	ReferenceKey string `json:"reference_key,omitempty"`
}

type Error struct {
	Code    string
	Message string
}

func (e Error) Error() string {
	return e.Code + ": " + e.Message
}

type CitationTrace struct {
	Version     string              `json:"version"`
	GeneratedAt time.Time           `json:"generated_at"`
	Items       []CitationTraceItem `json:"items"`
}

type CitationTraceItem struct {
	ChapterID        string `json:"chapter_id"`
	ParagraphID      string `json:"paragraph_id"`
	ClaimID          string `json:"claim_id"`
	ReferenceKey     string `json:"reference_key"`
	SourceType       string `json:"source_type"`
	EditorVerified   bool   `json:"editor_verified"`
	NeedsHumanReview bool   `json:"needs_human_review"`
}

type DocxExporter interface {
	Export(markdown string) ([]byte, error)
}

type QualityReportRenderer interface {
	RenderQualityReport(input ExportInput, generatedAt time.Time) (string, error)
}

type DocxExporterFunc func(markdown string) ([]byte, error)

func (f DocxExporterFunc) Export(markdown string) ([]byte, error) {
	return f(markdown)
}
