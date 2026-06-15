package export

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
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
		result.Metadata["quality_conclusion"] = buildQualityConclusion(input.Quality, input.Language)
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
	numbering := buildReferenceNumbering(input)
	var b strings.Builder
	title := strings.TrimSpace(input.Title)
	if title == "" {
		title = "Untitled Paper"
	}
	fmt.Fprintf(&b, "# %s\n\n", title)
	for i, chapter := range input.Chapters {
		body := renderChapterMarkdown(chapter, numbering)
		if strings.TrimSpace(body) == "" {
			continue
		}
		if i > 0 {
			b.WriteString("\n")
		}
		if !startsWithMarkdownHeading(body) {
			chapterTitle := strings.TrimSpace(chapter.Title)
			if chapterTitle == "" {
				chapterTitle = chapter.ID
			}
			fmt.Fprintf(&b, "## %s\n\n", chapterTitle)
		}
		b.WriteString(strings.TrimSpace(body))
		b.WriteString("\n\n")
	}
	refs, _ := renderReferencesMarkdown(input)
	refs = strings.TrimSpace(stripReferencesHeader(refs))
	if refs != "" {
		if normalizeExportCitationStyle(input.CitationStyle) == "gbt7714" {
			b.WriteString("## 参考文献\n\n")
		} else {
			b.WriteString("## References\n\n")
		}
		b.WriteString(refs)
		b.WriteString("\n")
	}
	return b.String()
}

func renderChapterMarkdown(chapter ChapterInput, numbering map[string]int) string {
	body := strings.TrimSpace(chapter.AcceptedMarkdown)
	if body == "" {
		return ""
	}
	body = replaceCitationKeys(body, numbering)
	missing := citationLabelsForChapter(chapter, numbering)
	if len(missing) == 0 {
		return body
	}
	return appendCitationsToParagraphs(body, missing)
}

type paragraphCitationLabel struct {
	ParagraphID string
	Label       string
	Numbers     []int
}

func renderReferencesMarkdown(input ExportInput) (string, []Issue) {
	var b strings.Builder
	var issues []Issue
	style := normalizeExportCitationStyle(input.CitationStyle)
	if style == "gbt7714" {
		b.WriteString("# 参考文献\n\n")
	} else {
		fmt.Fprintf(&b, "# References\n\nCitation style: %s\n\n", style)
	}
	refs := orderedReferences(input)
	for i, ref := range refs {
		line, ok := formatReference(style, i+1, ref)
		if !ok {
			issues = append(issues, Issue{Code: CodeReferenceFormat, Message: "reference has incomplete metadata", ReferenceKey: ref.Key})
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	return b.String(), issues
}

func formatReference(style string, number int, ref contracts.ConfirmedReference) (string, bool) {
	if style == "gbt7714" {
		return formatGBT7714Reference(number, ref)
	}
	return formatLegacyReference(ref)
}

func formatGBT7714Reference(number int, ref contracts.ConfirmedReference) (string, bool) {
	authors := formatGBTAuthors(ref.Authors)
	title := strings.TrimSpace(ref.Title)
	complete := authors != "" && title != ""
	if authors == "" {
		authors = "佚名"
	}
	if title == "" {
		title = "未命名文献"
	}
	venue := strings.TrimSpace(ref.Venue)
	year := "n.d."
	if ref.Year != 0 {
		year = fmt.Sprintf("%d", ref.Year)
	}
	line := fmt.Sprintf("[%d] %s. %s", number, authors, title)
	if venue != "" {
		line += fmt.Sprintf("[J]. %s, %s", venue, year)
	} else {
		line += fmt.Sprintf("[J/OL]. %s", year)
	}
	var suffix []string
	if strings.TrimSpace(ref.DOI) != "" {
		suffix = append(suffix, "DOI: "+strings.TrimSpace(ref.DOI))
	}
	if strings.TrimSpace(ref.URL) != "" {
		suffix = append(suffix, strings.TrimSpace(ref.URL))
	}
	if len(suffix) > 0 {
		line += ". " + strings.Join(suffix, ". ")
	}
	if !strings.HasSuffix(line, ".") && !strings.HasSuffix(line, "。") {
		line += "."
	}
	return line, complete
}

func formatLegacyReference(ref contracts.ConfirmedReference) (string, bool) {
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

func normalizeExportCitationStyle(style string) string {
	style = strings.ToLower(strings.TrimSpace(style))
	switch style {
	case "apa":
		return "apa"
	case "gb/t 7714", "gbt-7714", "gbt7714", "":
		return "gbt7714"
	default:
		return style
	}
}

func orderedReferences(input ExportInput) []contracts.ConfirmedReference {
	refsByKey := referenceMap(input.ConfirmedReferences)
	seen := map[string]bool{}
	var ordered []contracts.ConfirmedReference
	for _, chapter := range input.Chapters {
		for _, mapping := range chapter.CitationMap.Mappings {
			for _, key := range mapping.ReferenceKeys {
				if seen[key] {
					continue
				}
				if ref, ok := refsByKey[key]; ok {
					ordered = append(ordered, ref)
					seen[key] = true
				}
			}
		}
	}
	remaining := append([]contracts.ConfirmedReference(nil), input.ConfirmedReferences.Items...)
	sort.SliceStable(remaining, func(i, j int) bool { return remaining[i].Key < remaining[j].Key })
	for _, ref := range remaining {
		if !seen[ref.Key] {
			ordered = append(ordered, ref)
			seen[ref.Key] = true
		}
	}
	return ordered
}

func buildReferenceNumbering(input ExportInput) map[string]int {
	numbering := map[string]int{}
	for i, ref := range orderedReferences(input) {
		numbering[ref.Key] = i + 1
	}
	return numbering
}

func citationLabelsForChapter(chapter ChapterInput, numbering map[string]int) []paragraphCitationLabel {
	var labels []paragraphCitationLabel
	seen := map[string]bool{}
	for _, mapping := range chapter.CitationMap.Mappings {
		var nums []int
		for _, key := range mapping.ReferenceKeys {
			if n, ok := numbering[key]; ok {
				nums = append(nums, n)
			}
		}
		nums = uniqueSortedNumbers(nums)
		if len(nums) == 0 {
			continue
		}
		label := formatCitationNumbers(nums)
		seenKey := mapping.ParagraphID + "\x00" + label
		if !seen[seenKey] {
			labels = append(labels, paragraphCitationLabel{ParagraphID: mapping.ParagraphID, Label: label, Numbers: nums})
			seen[seenKey] = true
		}
	}
	return labels
}

func uniqueSortedNumbers(nums []int) []int {
	if len(nums) == 0 {
		return nil
	}
	sort.Ints(nums)
	unique := nums[:0]
	last := 0
	for i, n := range nums {
		if i == 0 || n != last {
			unique = append(unique, n)
			last = n
		}
	}
	return unique
}

func formatCitationNumbers(nums []int) string {
	nums = uniqueSortedNumbers(nums)
	if len(nums) == 0 {
		return ""
	}
	var parts []string
	for i := 0; i < len(nums); i++ {
		start := nums[i]
		end := start
		for i+1 < len(nums) && nums[i+1] == end+1 {
			i++
			end = nums[i]
		}
		if start == end {
			parts = append(parts, fmt.Sprintf("%d", start))
		} else {
			parts = append(parts, fmt.Sprintf("%d-%d", start, end))
		}
	}
	return "[" + strings.Join(parts, ",") + "]"
}

func replaceCitationKeys(markdown string, numbering map[string]int) string {
	out := markdown
	keys := make([]string, 0, len(numbering))
	for key := range numbering {
		keys = append(keys, key)
	}
	sort.SliceStable(keys, func(i, j int) bool { return len(keys[i]) > len(keys[j]) })
	for _, key := range keys {
		label := fmt.Sprintf("[%d]", numbering[key])
		out = strings.ReplaceAll(out, "["+key+"]", label)
		out = strings.ReplaceAll(out, "（"+key+"）", label)
		out = strings.ReplaceAll(out, "("+key+")", label)
	}
	return out
}

func appendCitationsToParagraphs(markdown string, labels []paragraphCitationLabel) string {
	markdown = normalizeMarkdownNewlines(markdown)
	parts := strings.Split(markdown, "\n\n")
	bodyIndices := paragraphBodyIndices(parts)
	if len(bodyIndices) == 0 {
		return strings.Join(parts, "\n\n")
	}
	nextFallback := 0
	for _, item := range labels {
		partIndex := -1
		if paragraphIndex, ok := paragraphOrdinal(item.ParagraphID); ok && paragraphIndex < len(bodyIndices) {
			partIndex = bodyIndices[paragraphIndex]
		} else {
			for nextFallback < len(bodyIndices) && paragraphHasCitationNumbers(parts[bodyIndices[nextFallback]], item) {
				nextFallback++
			}
			if nextFallback < len(bodyIndices) {
				partIndex = bodyIndices[nextFallback]
				nextFallback++
			}
		}
		if partIndex < 0 {
			partIndex = bodyIndices[len(bodyIndices)-1]
		}
		if paragraphHasCitationNumbers(parts[partIndex], item) {
			continue
		}
		parts[partIndex] = strings.TrimRight(parts[partIndex], " \t\n") + item.Label
	}
	return strings.Join(parts, "\n\n")
}

func normalizeMarkdownNewlines(markdown string) string {
	markdown = strings.ReplaceAll(markdown, "\r\n", "\n")
	return strings.ReplaceAll(markdown, "\r", "\n")
}

func paragraphBodyIndices(parts []string) []int {
	var indices []int
	for i, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" || startsWithMarkdownHeading(trimmed) {
			continue
		}
		indices = append(indices, i)
	}
	return indices
}

func paragraphOrdinal(paragraphID string) (int, bool) {
	paragraphID = strings.TrimSpace(paragraphID)
	if paragraphID == "" {
		return 0, false
	}
	idx := strings.LastIndex(paragraphID, "_p")
	if idx >= 0 {
		return parseParagraphOrdinal(paragraphID[idx+2:])
	}
	if strings.HasPrefix(paragraphID, "p") {
		return parseParagraphOrdinal(paragraphID[1:])
	}
	return 0, false
}

func parseParagraphOrdinal(value string) (int, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}
	n, err := strconv.Atoi(value)
	if err != nil || n <= 0 {
		return 0, false
	}
	return n - 1, true
}

func paragraphHasCitationNumbers(part string, item paragraphCitationLabel) bool {
	if strings.Contains(part, item.Label) {
		return true
	}
	for _, n := range item.Numbers {
		if !strings.Contains(part, fmt.Sprintf("[%d]", n)) {
			return false
		}
	}
	return len(item.Numbers) > 0
}

func startsWithMarkdownHeading(markdown string) bool {
	trimmed := strings.TrimSpace(markdown)
	if trimmed == "" || trimmed[0] != '#' {
		return false
	}
	count := 0
	for count < len(trimmed) && trimmed[count] == '#' {
		count++
	}
	return count <= 6 && (count == len(trimmed) || trimmed[count] == ' ' || trimmed[count] == '\t')
}

func stripReferencesHeader(markdown string) string {
	lines := strings.Split(markdown, "\n")
	for len(lines) > 0 {
		line := strings.TrimSpace(lines[0])
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "Citation style:") {
			lines = lines[1:]
			continue
		}
		break
	}
	return strings.Join(lines, "\n")
}

func formatGBTAuthors(authors []string) string {
	var cleaned []string
	for _, author := range authors {
		author = strings.TrimSpace(author)
		if author != "" {
			cleaned = append(cleaned, author)
		}
	}
	if len(cleaned) == 0 {
		return ""
	}
	if len(cleaned) > 3 {
		return strings.Join(cleaned[:3], ", ") + ", 等"
	}
	return strings.Join(cleaned, ", ")
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

func buildQualityConclusion(quality QualityInput, language string) string {
	english := strings.EqualFold(strings.TrimSpace(language), "en")
	if !quality.Available {
		if english {
			return "Quality artifacts not available (compatibility mode)"
		}
		return "质量门控：质量工件不可用（兼容模式）"
	}

	conclusion := quality.GateOutcome.Conclusion
	blockerCount := len(quality.GateOutcome.Blockers)
	findingCount := len(quality.GateOutcome.Findings)

	if english {
		switch conclusion {
		case "pass":
			if findingCount == 0 {
				return "Quality gate: all chapters passed"
			}
			return fmt.Sprintf("Quality gate: passed (%d suggestion(s))", findingCount)
		case "pass_with_warnings":
			return fmt.Sprintf("Quality gate: passed with %d warning(s)", findingCount)
		case "needs_revision":
			return fmt.Sprintf("Quality gate: %d chapter(s) need revision", chaptersAtSeverity(quality.GateOutcome.Findings, "needs_revision"))
		case "needs_human_review":
			return fmt.Sprintf("Quality gate: %d chapter(s) need human review", chaptersAtSeverity(quality.GateOutcome.Findings, "needs_human_review"))
		case "blocked":
			return fmt.Sprintf("Quality gate: blocked (%d hard blocker(s))", blockerCount)
		default:
			return fmt.Sprintf("Quality gate: %s", conclusion)
		}
	}

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
