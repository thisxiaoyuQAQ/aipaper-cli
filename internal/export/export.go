package export

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/artifacts"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/checkpoint"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
	qualitypkg "github.com/thisxiaoyuQAQ/aipaper-cli/internal/quality"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/store"
)

const (
	paperPath         = "final/paper.md"
	docxPath          = "final/paper.docx"
	referencesPath    = "final/references.md"
	citationTracePath = "final/citation-trace.json"
	qualityReportPath = "final/quality-report.md"
	reportPath        = "final/report.md"
)

func ExportFinal(s store.Store, input ExportInput, opts Options) (Result, error) {
	if len(input.Chapters) == 0 {
		return Result{}, Error{Code: CodeNoAcceptedChapters, Message: "at least one accepted chapter is required"}
	}
	if err := store.EnsureLayout(s); err != nil {
		return Result{}, err
	}
	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	now = now.UTC()
	version := "export-" + now.Format("20060102T150405Z")
	result := Result{Version: version}

	referenceByKey := referenceMap(input.ConfirmedReferences)
	var issues []Issue
	for _, chapter := range input.Chapters {
		for _, issue := range artifacts.ValidateDraftArtifacts(chapter.Claims, chapter.CitationMap, input.ConfirmedReferences) {
			if issue.Code == artifacts.CodeUnconfirmedRef {
				return Result{}, Error{Code: CodeUnconfirmedReference, Message: issue.Message}
			}
			issues = append(issues, Issue{Code: issue.Code, Message: issue.Message, ChapterID: chapter.ID})
		}
	}

	paperMarkdown := renderPaperMarkdown(input)
	referencesMarkdown, referenceIssues := renderReferencesMarkdown(input)
	issues = append(issues, referenceIssues...)

	trace, err := buildCitationTrace(version, now, input.Chapters, referenceByKey)
	if err != nil {
		return Result{}, err
	}

	docxBytes, docxErr := exportDocx(paperMarkdown, opts.DocxExporter)
	if docxErr != nil {
		issues = append(issues, Issue{Code: CodeDocxFailed, Message: docxErr.Error()})
		if err := os.Remove(s.Path(filepath.FromSlash(docxPath))); err != nil && !os.IsNotExist(err) {
			return Result{}, err
		}
	} else {
		result.DocxWritten = true
	}
	result.Issues = issues

	if err := writeText(s, paperPath, "paper_markdown", paperMarkdown, &result); err != nil {
		return Result{}, err
	}
	if err := writeText(s, referencesPath, "references_markdown", referencesMarkdown, &result); err != nil {
		return Result{}, err
	}
	if err := writeJSON(s, citationTracePath, "citation_trace", trace, &result); err != nil {
		return Result{}, err
	}
	if result.DocxWritten {
		if err := writeBytes(s, docxPath, "paper_docx", docxBytes, &result); err != nil {
			return Result{}, err
		}
	}
	if issue := writeQualityReportIfAvailable(s, input, now, opts.QualityReportRenderer, &result); issue != nil {
		issues = append(issues, *issue)
	}
	result.Issues = issues

	// Populate quality conclusion in metadata
	if input.Quality.Available {
		if result.Metadata == nil {
			result.Metadata = make(map[string]any)
		}
		result.Metadata["quality_conclusion"] = buildQualityConclusion(input.Quality)
	}

	report := renderReport(input, result, now)
	if err := writeText(s, reportPath, "export_report", report, &result); err != nil {
		return Result{}, err
	}
	return result, nil
}

func exportDocx(markdown string, exporter DocxExporter) ([]byte, error) {
	if exporter == nil {
		exporter = SimpleDocxExporter{}
	}
	return exporter.Export(markdown)
}

func buildCitationTrace(version string, generatedAt time.Time, chapters []ChapterInput, refs map[string]contracts.ConfirmedReference) (CitationTrace, error) {
	trace := CitationTrace{Version: version, GeneratedAt: generatedAt, Items: []CitationTraceItem{}}
	for _, chapter := range chapters {
		unsupported := stringSet(chapter.Review.UnsupportedClaims)
		for _, mapping := range chapter.CitationMap.Mappings {
			for _, claimID := range mapping.ClaimIDs {
				for _, key := range mapping.ReferenceKeys {
					ref, ok := refs[key]
					if !ok {
						return CitationTrace{}, Error{Code: CodeUnconfirmedReference, Message: fmt.Sprintf("citation trace references unconfirmed key %q", key)}
					}
					editorVerified := !unsupported[claimID]
					trace.Items = append(trace.Items, CitationTraceItem{
						ChapterID:        chapter.ID,
						ParagraphID:      mapping.ParagraphID,
						ClaimID:          claimID,
						ReferenceKey:     key,
						SourceType:       referenceSourceType(ref),
						EditorVerified:   editorVerified,
						NeedsHumanReview: chapterNeedsHumanReview(chapter) || !editorVerified,
					})
				}
			}
		}
	}
	return trace, nil
}

func renderPaperMarkdown(input ExportInput) string {
	var b strings.Builder
	title := strings.TrimSpace(input.Title)
	if title == "" {
		title = "Untitled Paper"
	}
	fmt.Fprintf(&b, "# %s\n\n", title)
	for i, chapter := range input.Chapters {
		body := strings.TrimSpace(chapter.AcceptedMarkdown)
		if body == "" {
			continue
		}
		if i > 0 {
			b.WriteString("\n\n")
		}
		if !strings.HasPrefix(body, "#") {
			chapterTitle := strings.TrimSpace(chapter.Title)
			if chapterTitle == "" {
				chapterTitle = chapter.ID
			}
			fmt.Fprintf(&b, "## %s\n\n", chapterTitle)
		}
		b.WriteString(body)
		b.WriteString("\n")
	}
	return b.String()
}

func renderReferencesMarkdown(input ExportInput) (string, []Issue) {
	var b strings.Builder
	var issues []Issue
	style := strings.TrimSpace(input.CitationStyle)
	if style == "" {
		style = "default"
	}
	fmt.Fprintf(&b, "# References\n\nCitation style: %s\n\n", style)
	refs := append([]contracts.ConfirmedReference(nil), input.ConfirmedReferences.Items...)
	sort.SliceStable(refs, func(i, j int) bool {
		return refs[i].Key < refs[j].Key
	})
	for _, ref := range refs {
		line, ok := formatReference(ref)
		if !ok {
			issues = append(issues, Issue{Code: CodeReferenceFormat, Message: "reference has incomplete metadata", ReferenceKey: ref.Key})
		}
		b.WriteString("- ")
		b.WriteString(line)
		b.WriteString("\n")
	}
	return b.String(), issues
}

func formatReference(ref contracts.ConfirmedReference) (string, bool) {
	key := strings.TrimSpace(ref.Key)
	if key == "" {
		key = "unknown"
	}
	authors := strings.Join(ref.Authors, ", ")
	if authors == "" {
		authors = "Unknown author"
	}
	title := strings.TrimSpace(ref.Title)
	complete := title != "" && len(ref.Authors) > 0
	if title == "" {
		title = "Untitled reference"
	}
	year := "n.d."
	if ref.Year != 0 {
		year = fmt.Sprintf("%d", ref.Year)
	}
	var suffix []string
	if strings.TrimSpace(ref.DOI) != "" {
		suffix = append(suffix, "DOI: "+strings.TrimSpace(ref.DOI))
	}
	if strings.TrimSpace(ref.URL) != "" {
		suffix = append(suffix, "URL: "+strings.TrimSpace(ref.URL))
	}
	line := fmt.Sprintf("[%s] %s (%s). %s.", key, authors, year, title)
	if len(suffix) > 0 {
		line += " " + strings.Join(suffix, " ")
	}
	return line, complete
}

func renderReport(input ExportInput, result Result, generatedAt time.Time) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Export Report\n\n")
	fmt.Fprintf(&b, "- Export version: `%s`\n", result.Version)
	fmt.Fprintf(&b, "- Generated at: `%s`\n", generatedAt.Format(time.RFC3339))
	fmt.Fprintf(&b, "- Overwrite strategy: final artifacts are overwritten on each export; chapter artifacts are not modified.\n")
	fmt.Fprintf(&b, "- Markdown: `%s`\n", paperPath)
	if result.DocxWritten {
		fmt.Fprintf(&b, "- Docx: `%s`\n", docxPath)
	} else {
		fmt.Fprintf(&b, "- Docx: failed with `%s`\n", CodeDocxFailed)
	}
	fmt.Fprintf(&b, "- References: `%s`\n", referencesPath)
	fmt.Fprintf(&b, "- Citation trace: `%s`\n", citationTracePath)
	if input.Quality.Available && !hasIssue(result.Issues, CodeQualityReportFailed) {
		fmt.Fprintf(&b, "- Quality report: `%s`\n", qualityReportPath)
	} else if input.Quality.LoadError != "" || len(input.Quality.MissingArtifacts) > 0 {
		fmt.Fprintf(&b, "- Quality report: not generated (compatibility mode)\n")
	}
	b.WriteString("\n")

	renderReportQualitySummary(&b, input, result)

	renderReportQualitySummary(&b, input, result)

	b.WriteString("## Chapter Review Summary\n\n")
	b.WriteString("| Chapter | Overall | Citation consistency | Passed | Unsupported claims |\n")
	b.WriteString("| --- | ---: | ---: | --- | ---: |\n")
	for _, chapter := range input.Chapters {
		fmt.Fprintf(&b, "| %s | %d | %d | %t | %d |\n",
			chapter.ID,
			chapter.Review.Scores.Overall,
			chapter.Review.Scores.CitationConsistency,
			chapter.Review.Passed,
			len(chapter.Review.UnsupportedClaims),
		)
	}

	b.WriteString("\n## Needs Human Review\n\n")
	needsReview := false
	for _, chapter := range input.Chapters {
		if chapterNeedsHumanReview(chapter) {
			needsReview = true
			fmt.Fprintf(&b, "- `%s`\n", chapter.ID)
		}
	}
	if !needsReview {
		b.WriteString("- None\n")
	}

	b.WriteString("\n## Issues\n\n")
	if len(result.Issues) == 0 {
		b.WriteString("- None\n")
	} else {
		for _, issue := range result.Issues {
			fmt.Fprintf(&b, "- `%s`: %s", issue.Code, issue.Message)
			if issue.ChapterID != "" {
				fmt.Fprintf(&b, " (chapter `%s`)", issue.ChapterID)
			}
			if issue.ReferenceKey != "" {
				fmt.Fprintf(&b, " (reference `%s`)", issue.ReferenceKey)
			}
			b.WriteString("\n")
		}
	}

	if len(input.CostEstimate) > 0 {
		b.WriteString("\n## Cost Estimate\n\n")
		data, err := json.MarshalIndent(input.CostEstimate, "", "  ")
		if err == nil {
			b.WriteString("```json\n")
			b.Write(data)
			b.WriteString("\n```\n")
		}
	}
	return b.String()
}

func renderReportQualitySummary(b *strings.Builder, input ExportInput, result Result) {
	b.WriteString("## Quality Summary\n\n")
	if !input.Quality.Available {
		if len(input.Quality.MissingArtifacts) > 0 {
			b.WriteString("Quality artifacts missing (compatibility mode):\n")
			for _, artifact := range input.Quality.MissingArtifacts {
				fmt.Fprintf(b, "- `%s`\n", artifact)
			}
		} else if input.Quality.LoadError != "" {
			fmt.Fprintf(b, "Quality artifacts could not be loaded: %s\n", input.Quality.LoadError)
		} else {
			b.WriteString("Quality mode: not enabled for this run.\n")
		}
		b.WriteString("\n")
		return
	}
	fmt.Fprintf(b, "- Mode: `%s`\n", input.Quality.Mode)
	fmt.Fprintf(b, "- Gate conclusion: `%s`\n", input.Quality.GateOutcome.Conclusion)
	if len(input.Quality.GateOutcome.Blockers) > 0 {
		fmt.Fprintf(b, "- Hard blockers: %d\n", len(input.Quality.GateOutcome.Blockers))
	}
	if len(input.Quality.GateOutcome.Findings) > 0 {
		fmt.Fprintf(b, "- Risk findings: %d\n", len(input.Quality.GateOutcome.Findings))
	}
	if hasIssue(result.Issues, CodeQualityReportFailed) {
		b.WriteString("- Full quality report: generation failed (see Issues)\n")
	} else {
		fmt.Fprintf(b, "- Full quality report: `%s`\n", qualityReportPath)
	}
	b.WriteString("\n")
}

func buildQualityConclusion(quality QualityInput) string {
	if !quality.Available {
		return "Quality artifacts not available (compatibility mode)"
	}

	conclusion := quality.GateOutcome.Conclusion
	blockerCount := len(quality.GateOutcome.Blockers)
	findingCount := len(quality.GateOutcome.Findings)

	switch conclusion {
	case "pass":
		if findingCount == 0 {
			return "质量门控：全部章节通过"
		}
		return fmt.Sprintf("质量门控：通过（%d 条建议）", findingCount)
	case "pass_with_warnings":
		return fmt.Sprintf("质量门控：通过但有 %d 条警告", findingCount)
	case "needs_revision":
		return fmt.Sprintf("质量门控：%d 章需要修订", chaptersAtSeverity(quality.GateOutcome.Findings, "needs_revision"))
	case "needs_human_review":
		return fmt.Sprintf("质量门控：%d 章需要人工复核", chaptersAtSeverity(quality.GateOutcome.Findings, "needs_human_review"))
	case "blocked":
		return fmt.Sprintf("质量门控：已阻断（%d 条硬门槛问题）", blockerCount)
	default:
		return fmt.Sprintf("质量门控：%s", conclusion)
	}
}

// chaptersAtSeverity counts the distinct chapters carrying findings of the
// given severity, so the conclusion line reports chapters rather than raw
// finding counts. Findings without a chapter id (defensive) count as one each.
func chaptersAtSeverity(findings []qualitypkg.GateIssue, severity string) int {
	chapters := map[string]bool{}
	orphans := 0
	for _, finding := range findings {
		if finding.Severity != severity {
			continue
		}
		if finding.ChapterID == "" {
			orphans++
			continue
		}
		chapters[finding.ChapterID] = true
	}
	return len(chapters) + orphans
}

func referenceMap(confirmed contracts.ConfirmedReferences) map[string]contracts.ConfirmedReference {
	refs := map[string]contracts.ConfirmedReference{}
	for _, ref := range confirmed.Items {
		refs[ref.Key] = ref
	}
	return refs
}

func referenceSourceType(ref contracts.ConfirmedReference) string {
	for _, id := range ref.SourceMaterialIDs {
		lower := strings.ToLower(id)
		if strings.Contains(lower, "bib") {
			return "bibtex"
		}
	}
	if len(ref.SourceMaterialIDs) > 0 {
		return "user_material"
	}
	return "academic_search"
}

func stringSet(items []string) map[string]bool {
	set := map[string]bool{}
	for _, item := range items {
		set[item] = true
	}
	return set
}

func chapterNeedsHumanReview(chapter ChapterInput) bool {
	return chapter.Status == artifacts.StatusNeedsHumanReview || !chapter.Review.Passed || len(chapter.Review.UnsupportedClaims) > 0
}

func writeText(s store.Store, relPath, kind, text string, result *Result) error {
	return writeBytes(s, relPath, kind, []byte(ensureTrailingNewline(text)), result)
}

func writeJSON(s store.Store, relPath, kind string, value any, result *Result) error {
	res, err := store.WriteJSON(s.Path(filepath.FromSlash(relPath)), value, store.Overwrite)
	if err != nil {
		return err
	}
	result.addOutput(kind, relPath, res.SHA256)
	return nil
}

func writeBytes(s store.Store, relPath, kind string, data []byte, result *Result) error {
	res, err := store.WriteFile(s.Path(filepath.FromSlash(relPath)), data, store.Overwrite)
	if err != nil {
		return err
	}
	result.addOutput(kind, relPath, res.SHA256)
	return nil
}

func (r *Result) addOutput(kind, relPath, sha string) {
	r.Outputs = append(r.Outputs, checkpoint.OutputArtifact{Kind: kind, Path: relPath, SHA256: sha})
}

func ensureTrailingNewline(s string) string {
	if s == "" || s[len(s)-1] == '\n' {
		return s
	}
	return s + "\n"
}

func hasIssue(issues []Issue, code string) bool {
	for _, issue := range issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}
