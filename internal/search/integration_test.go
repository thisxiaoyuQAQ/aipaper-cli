package search

import (
	"testing"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
)

// TestChineseHistoricalTopicEndToEnd validates the complete flow for Chinese historical topics
func TestChineseHistoricalTopicEndToEnd(t *testing.T) {
	req := contracts.Requirements{
		Topic:             "明朝君主哪个更厉害",
		ResearchQuestions: []string{"朱元璋和朱棣的政绩比较"},
		Language:          "zh-CN",
	}

	queries := GenerateQueries(req, 10)

	// Should generate at least 3 queries
	if len(queries) < 3 {
		t.Errorf("Expected at least 3 queries, got %d", len(queries))
	}

	// Check that no query contains English academic suffixes
	for _, q := range queries {
		if containsEnglishSuffix(q.Text) {
			t.Errorf("Chinese query should not contain English suffix: %q", q.Text)
		}
	}

	// Check expected patterns
	hasResearchQuery := false
	hasEvaluationQuery := false
	for _, q := range queries {
		if contains(q.Text, "研究") {
			hasResearchQuery = true
		}
		if contains(q.Text, "评价") {
			hasEvaluationQuery = true
		}
	}

	if !hasResearchQuery {
		t.Error("Expected at least one query with '研究'")
	}
	if !hasEvaluationQuery {
		t.Error("Expected at least one query with '评价'")
	}

	t.Logf("Generated queries for Chinese historical topic:")
	for i, q := range queries {
		t.Logf("  %d. %q (score: %.2f, reason: %s)", i+1, q.Text, q.Score, q.Reason)
	}
}

// TestChineseGeneralTopicEndToEnd validates the flow for Chinese general academic topics
func TestChineseGeneralTopicEndToEnd(t *testing.T) {
	req := contracts.Requirements{
		Topic:    "深度学习在医学影像中的应用",
		Language: "zh-CN",
	}

	queries := GenerateQueries(req, 10)

	// Should generate at least 2 queries
	if len(queries) < 2 {
		t.Errorf("Expected at least 2 queries, got %d", len(queries))
	}

	// Check that no query contains English academic suffixes
	for _, q := range queries {
		if containsEnglishSuffix(q.Text) {
			t.Errorf("Chinese query should not contain English suffix: %q", q.Text)
		}
	}

	// Check expected patterns
	hasReviewQuery := false
	hasStatusQuery := false
	for _, q := range queries {
		if contains(q.Text, "综述") {
			hasReviewQuery = true
		}
		if contains(q.Text, "研究现状") {
			hasStatusQuery = true
		}
	}

	if !hasReviewQuery {
		t.Error("Expected at least one query with '综述'")
	}
	if !hasStatusQuery {
		t.Error("Expected at least one query with '研究现状'")
	}

	t.Logf("Generated queries for Chinese general topic:")
	for i, q := range queries {
		t.Logf("  %d. %q (score: %.2f, reason: %s)", i+1, q.Text, q.Score, q.Reason)
	}
}

// TestEnglishTopicPreservesExistingBehavior validates backward compatibility for English topics
func TestEnglishTopicPreservesExistingBehavior(t *testing.T) {
	req := contracts.Requirements{
		Topic:             "Retrieval augmented generation",
		ResearchQuestions: []string{"How does RAG improve accuracy"},
		Language:          "en",
	}

	queries := GenerateQueries(req, 10)

	// Should generate at least 2 queries
	if len(queries) < 2 {
		t.Errorf("Expected at least 2 queries, got %d", len(queries))
	}

	// Check expected patterns for English
	hasSystematicReview := false
	hasEvidenceLiterature := false
	for _, q := range queries {
		if contains(q.Text, "systematic review") {
			hasSystematicReview = true
		}
		if contains(q.Text, "evidence literature review") {
			hasEvidenceLiterature = true
		}
	}

	if !hasSystematicReview {
		t.Error("Expected at least one query with 'systematic review'")
	}
	if !hasEvidenceLiterature {
		t.Error("Expected at least one query with 'evidence literature review'")
	}

	t.Logf("Generated queries for English topic:")
	for i, q := range queries {
		t.Logf("  %d. %q (score: %.2f, reason: %s)", i+1, q.Text, q.Score, q.Reason)
	}
}

// TestBackwardCompatibilityQueryFromRequirements ensures old API still works
func TestBackwardCompatibilityQueryFromRequirements(t *testing.T) {
	req := contracts.Requirements{
		Topic:             "明朝君主哪个更厉害",
		ResearchQuestions: []string{"朱元璋和朱棣的政绩比较"},
		Language:          "zh-CN",
	}

	query := QueryFromRequirements(req, 10)

	// Should return a valid query
	if query.Text == "" {
		t.Error("Expected non-empty query text")
	}

	// Should use Chinese-optimized query (no English suffix)
	if containsEnglishSuffix(query.Text) {
		t.Errorf("Backward compatible query should also be optimized: %q", query.Text)
	}

	t.Logf("Backward compatible query: %q", query.Text)
}

// TestBackwardCompatibilityExpansionQueries ensures old expansion API still works
func TestBackwardCompatibilityExpansionQueries(t *testing.T) {
	req := contracts.Requirements{
		Topic:              "深度学习应用",
		ChapterPreferences: []string{"算法", "应用场景"},
		Language:           "zh-CN",
	}

	baseQuery := Query{
		Text:     "深度学习应用 综述",
		Language: "zh-CN",
		Limit:    10,
	}

	expansionQueries := ExpansionQueriesFromRequirements(req, baseQuery, 3)

	// Should return expansion queries
	if len(expansionQueries) == 0 {
		t.Error("Expected at least one expansion query")
	}

	// Should not duplicate base query
	for _, q := range expansionQueries {
		if q.Text == baseQuery.Text {
			t.Errorf("Expansion query duplicates base query: %q", q.Text)
		}
	}

	t.Logf("Backward compatible expansion queries:")
	for i, q := range expansionQueries {
		t.Logf("  %d. %q", i+1, q.Text)
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
		len(s) > len(substr)+1 && containsMiddle(s, substr)))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
