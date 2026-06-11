package export

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/quality"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/store"
)

type defaultQualityReportRenderer struct{}

func (defaultQualityReportRenderer) RenderQualityReport(input ExportInput, generatedAt time.Time) (string, error) {
	return RenderQualityReport(input, generatedAt), nil
}

// QualityReportRendererFunc adapts a function into a quality report renderer.
type QualityReportRendererFunc func(ExportInput, time.Time) (string, error)

func (f QualityReportRendererFunc) RenderQualityReport(input ExportInput, generatedAt time.Time) (string, error) {
	return f(input, generatedAt)
}

func writeQualityReportIfAvailable(s store.Store, input ExportInput, generatedAt time.Time, renderer QualityReportRenderer, result *Result) *Issue {
	if !input.Quality.Available {
		if input.Quality.LoadError != "" {
			return &Issue{Code: CodeQualityReportFailed, Message: "quality artifacts could not be loaded: " + input.Quality.LoadError}
		}
		if len(input.Quality.MissingArtifacts) > 0 {
			return &Issue{Code: CodeQualityArtifactsMissing, Message: "quality artifacts missing; export continued in compatibility mode: " + strings.Join(input.Quality.MissingArtifacts, ", ")}
		}
		return nil
	}
	if renderer == nil {
		renderer = defaultQualityReportRenderer{}
	}
	report, err := renderer.RenderQualityReport(input, generatedAt)
	if err != nil {
		removeStaleQualityReport(s)
		return &Issue{Code: CodeQualityReportFailed, Message: err.Error()}
	}
	if err := writeText(s, qualityReportPath, "quality_report", report, result); err != nil {
		removeStaleQualityReport(s)
		return &Issue{Code: CodeQualityReportFailed, Message: err.Error()}
	}
	return nil
}

func removeStaleQualityReport(s store.Store) {
	_ = os.Remove(s.Path(filepath.FromSlash(qualityReportPath)))
}

// RenderQualityReport renders final/quality-report.md from already-loaded quality
// artifacts. Missing artifacts are handled by report.md compatibility warnings and
// do not call this renderer.
func RenderQualityReport(input ExportInput, generatedAt time.Time) string {
	var b strings.Builder
	q := input.Quality
	outcome := q.GateOutcome
	if outcome.Conclusion == "" {
		outcome = quality.EvaluateQualityGate(quality.GateInput{
			Mode:          q.Mode,
			Graph:         q.ClaimGraph,
			ConfirmedKeys: confirmedKeySet(input.ConfirmedReferences),
			EvidenceByID:  evidenceMap(q.EvidenceTable),
			RewriteRounds: rewriteRoundsByChapter(input.Chapters),
		})
	}
	mode := q.Mode
	if mode == "" {
		mode = outcome.Mode
	}
	if mode == "" {
		mode = quality.ModeEnhanced
	}

	b.WriteString("# Quality Report\n\n")
	fmt.Fprintf(&b, "- Generated at: `%s`\n", generatedAt.UTC().Format(time.RFC3339))
	fmt.Fprintf(&b, "- Quality mode: `%s`\n", mode)
	fmt.Fprintf(&b, "- Overall quality status: `%s`\n", outcome.Conclusion)
	fmt.Fprintf(&b, "- Claims checked: %d\n", len(q.ClaimGraph.Claims))
	fmt.Fprintf(&b, "- Verification verdicts: %d\n\n", len(q.VerificationResult.Verdicts))

	renderHardGateSummary(&b, outcome)
	renderEvidenceDepthDistribution(&b, q.EvidenceTable)
	renderSupportSummary(&b, q.ClaimGraph)
	renderUnsupportedAndOverstated(&b, q.ClaimGraph)
	renderNeedsHumanReview(&b, input, q.ClaimGraph, outcome, mode)
	renderRewriteSummary(&b, input, q.ClaimGraph, outcome)
	renderNextSteps(&b, q.ClaimGraph, outcome)
	return b.String()
}

func renderHardGateSummary(b *strings.Builder, outcome quality.GateOutcome) {
	b.WriteString("## Hard Gate Summary\n\n")
	if len(outcome.Blockers) == 0 {
		b.WriteString("- Passed: no hard blockers detected.\n")
	} else {
		fmt.Fprintf(b, "- Failed: %d blocker(s) detected.\n\n", len(outcome.Blockers))
		renderGateIssueTable(b, outcome.Blockers)
	}
	if len(outcome.Findings) > 0 {
		b.WriteString("\n### Risk Findings\n\n")
		renderGateIssueTable(b, outcome.Findings)
	}
	b.WriteString("\n")
}

func renderGateIssueTable(b *strings.Builder, issues []quality.GateIssue) {
	b.WriteString("| Severity | Code | Chapter | Claim | Message |\n")
	b.WriteString("| --- | --- | --- | --- | --- |\n")
	for _, issue := range issues {
		fmt.Fprintf(b, "| %s | `%s` | %s | %s | %s |\n",
			markdownCell(issue.Severity), issue.Code, markdownCell(issue.ChapterID), markdownCell(issue.ClaimID), markdownCell(issue.Message))
	}
}

func renderEvidenceDepthDistribution(b *strings.Builder, table quality.EvidenceTable) {
	b.WriteString("## Evidence Depth Distribution\n\n")
	counts := evidenceDepthCounts(table)
	b.WriteString("| Depth | Count |\n")
	b.WriteString("| --- | ---: |\n")
	for _, depth := range []string{quality.DepthMetadataOnly, quality.DepthAbstract, quality.DepthSnippet, quality.DepthFulltextExcerpt} {
		fmt.Fprintf(b, "| `%s` | %d |\n", depth, counts[depth])
	}
	b.WriteString("\n")
}

func renderSupportSummary(b *strings.Builder, graph quality.ClaimGraph) {
	b.WriteString("## Claim Support Summary\n\n")
	counts := map[string]int{}
	for _, node := range graph.Claims {
		support := node.Support
		if support == "" {
			support = "unverified"
		}
		counts[support]++
	}
	keys := []string{"supported", "partially_supported", "unsupported", "overstated", "skipped", "unverified"}
	b.WriteString("| Support | Count |\n")
	b.WriteString("| --- | ---: |\n")
	for _, key := range keys {
		fmt.Fprintf(b, "| `%s` | %d |\n", key, counts[key])
	}
	b.WriteString("\n")
}

func renderUnsupportedAndOverstated(b *strings.Builder, graph quality.ClaimGraph) {
	b.WriteString("## Unsupported / Overstated Claims\n\n")
	claims := filterClaims(graph.Claims, func(node quality.ClaimNode) bool {
		return node.Support == quality.SupportUnsupported || node.Support == quality.SupportOverstated
	})
	if len(claims) == 0 {
		b.WriteString("- None\n\n")
		return
	}
	b.WriteString("| Chapter | Claim | Support | Text | Verifier note |\n")
	b.WriteString("| --- | --- | --- | --- | --- |\n")
	for _, node := range claims {
		fmt.Fprintf(b, "| %s | `%s` | `%s` | %s | %s |\n",
			markdownCell(node.ChapterID), node.ID, node.Support, markdownCell(node.Text), markdownCell(node.VerifierNote))
	}
	b.WriteString("\n")
}

func renderNeedsHumanReview(b *strings.Builder, input ExportInput, graph quality.ClaimGraph, outcome quality.GateOutcome, mode string) {
	b.WriteString("## Needs Human Review\n\n")
	topPriority, otherIssues := humanReviewIssues(outcome.Findings)
	if mode == quality.ModeStrict && len(topPriority) > 0 {
		b.WriteString("### Strict-mode top priority\n\n")
		renderGateIssueTable(b, topPriority)
		b.WriteString("\n")
	}

	chapters := needsHumanReviewChapters(input, graph, append(topPriority, otherIssues...))
	if len(chapters) == 0 && !(mode == quality.ModeStrict && len(topPriority) > 0) {
		b.WriteString("- None\n\n")
		return
	}
	if len(chapters) > 0 {
		b.WriteString("### Chapters\n\n")
		for _, chapterID := range chapters {
			fmt.Fprintf(b, "- `%s`\n", chapterID)
		}
		b.WriteString("\n")
	}
	if len(otherIssues) > 0 {
		b.WriteString("### Human-review findings\n\n")
		renderGateIssueTable(b, otherIssues)
		b.WriteString("\n")
	}
}

func renderRewriteSummary(b *strings.Builder, input ExportInput, graph quality.ClaimGraph, outcome quality.GateOutcome) {
	b.WriteString("## Rewrite Summary\n\n")
	needsRewriteByChapter := map[string]int{}
	for _, node := range graph.Claims {
		if node.NeedsRewrite {
			needsRewriteByChapter[node.ChapterID]++
		}
	}
	humanReview := map[string]bool{}
	for _, issue := range outcome.Findings {
		if issue.Severity == quality.SeverityNeedsHumanReview {
			humanReview[issue.ChapterID] = true
		}
	}
	b.WriteString("| Chapter | Rounds | Required instructions | Optional instructions | Claims needing rewrite | Convergence |\n")
	b.WriteString("| --- | ---: | ---: | ---: | ---: | --- |\n")
	for _, chapter := range input.Chapters {
		required, optional := rewriteInstructionCounts(chapter)
		convergence := "converged"
		if humanReview[chapter.ID] || chapterNeedsHumanReview(chapter) {
			convergence = "needs_human_review"
		} else if needsRewriteByChapter[chapter.ID] > 0 || required > 0 {
			convergence = "needs_revision"
		}
		fmt.Fprintf(b, "| %s | %d | %d | %d | %d | `%s` |\n",
			markdownCell(chapter.ID), rewriteRoundsForChapter(chapter), required, optional, needsRewriteByChapter[chapter.ID], convergence)
	}
	b.WriteString("\n")
}

func renderNextSteps(b *strings.Builder, graph quality.ClaimGraph, outcome quality.GateOutcome) {
	b.WriteString("## Suggested Next Human Edits\n\n")
	steps := []string{"Read `final/paper.md` once for narrative flow and terminology consistency before external use."}
	if len(outcome.Blockers) > 0 {
		steps = append(steps, "Resolve hard blockers first: ensure every claim cites confirmed references and binds existing evidence IDs.")
	}
	if len(filterClaims(graph.Claims, func(node quality.ClaimNode) bool { return node.Support == quality.SupportUnsupported })) > 0 {
		steps = append(steps, "Rewrite or remove unsupported claims, then rerun verification/export.")
	}
	if len(filterClaims(graph.Claims, func(node quality.ClaimNode) bool { return node.Support == quality.SupportOverstated })) > 0 {
		steps = append(steps, "Soften overstated claims so wording matches the cited evidence depth.")
	}
	if hasFinding(outcome.Findings, quality.CodeGateShallowEvidenceStrongClaim) || hasFinding(outcome.Findings, quality.CodeGateMetadataOnlySoleSupport) {
		steps = append(steps, "Upgrade shallow evidence for key claims or narrow those claims to match abstract/metadata-level support.")
	}
	if hasHumanReviewFinding(outcome.Findings) {
		steps = append(steps, "Manually inspect chapters marked `needs_human_review`; the automated rewrite loop should not keep cycling.")
	}
	for _, step := range steps {
		fmt.Fprintf(b, "- %s\n", step)
	}
}

func evidenceMap(table quality.EvidenceTable) map[string]quality.Evidence {
	items := make(map[string]quality.Evidence, len(table.Items))
	for _, item := range table.Items {
		items[item.ID] = item
	}
	return items
}

func evidenceDepthCounts(table quality.EvidenceTable) map[string]int {
	counts := map[string]int{}
	for _, item := range table.Items {
		counts[item.Depth]++
	}
	return counts
}

func filterClaims(claims []quality.ClaimNode, keep func(quality.ClaimNode) bool) []quality.ClaimNode {
	out := make([]quality.ClaimNode, 0, len(claims))
	for _, node := range claims {
		if keep(node) {
			out = append(out, node)
		}
	}
	return out
}

func humanReviewIssues(findings []quality.GateIssue) ([]quality.GateIssue, []quality.GateIssue) {
	var topPriority []quality.GateIssue
	var other []quality.GateIssue
	for _, issue := range findings {
		if issue.Severity != quality.SeverityNeedsHumanReview {
			continue
		}
		if issue.TopPriority {
			topPriority = append(topPriority, issue)
		} else {
			other = append(other, issue)
		}
	}
	return topPriority, other
}

func needsHumanReviewChapters(input ExportInput, graph quality.ClaimGraph, issues []quality.GateIssue) []string {
	chapters := map[string]bool{}
	for _, chapter := range input.Chapters {
		if chapterNeedsHumanReview(chapter) {
			chapters[chapter.ID] = true
		}
	}
	for _, node := range graph.Claims {
		if node.NeedsHumanReview {
			chapters[node.ChapterID] = true
		}
	}
	for _, issue := range issues {
		if issue.ChapterID != "" {
			chapters[issue.ChapterID] = true
		}
	}
	return sortedKeys(chapters)
}

func rewriteInstructionCounts(chapter ChapterInput) (required, optional int) {
	for _, instruction := range chapter.Review.RewriteInstructions {
		switch instruction.Severity {
		case "required":
			required++
		case "optional":
			optional++
		}
	}
	return required, optional
}

func rewriteRoundsForChapter(chapter ChapterInput) int {
	if chapter.Version <= 1 {
		return 0
	}
	return chapter.Version - 1
}

func hasFinding(findings []quality.GateIssue, code string) bool {
	for _, finding := range findings {
		if finding.Code == code {
			return true
		}
	}
	return false
}

func hasHumanReviewFinding(findings []quality.GateIssue) bool {
	for _, finding := range findings {
		if finding.Severity == quality.SeverityNeedsHumanReview {
			return true
		}
	}
	return false
}

func sortedKeys(values map[string]bool) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		if key != "" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}

func markdownCell(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "-"
	}
	value = strings.ReplaceAll(value, "|", "\\|")
	value = strings.ReplaceAll(value, "\n", " ")
	return value
}
