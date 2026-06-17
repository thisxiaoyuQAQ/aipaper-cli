package search

import (
	"testing"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
)

func TestGenerateQueries(t *testing.T) {
	tests := []struct {
		name          string
		req           contracts.Requirements
		limit         int
		expectMinimum int
		expectChinese bool
		expectEnglish bool
	}{
		{
			name: "Chinese historical comparison",
			req: contracts.Requirements{
				Topic:             "明朝君主哪个更厉害",
				ResearchQuestions: []string{"朱元璋和朱棣的政绩比较"},
				Language:          "zh-CN",
			},
			limit:         10,
			expectMinimum: 3,
			expectChinese: true,
			expectEnglish: false,
		},
		{
			name: "Chinese general academic",
			req: contracts.Requirements{
				Topic:    "深度学习在医学影像中的应用",
				Language: "zh-CN",
			},
			limit:         10,
			expectMinimum: 2,
			expectChinese: true,
			expectEnglish: false,
		},
		{
			name: "English academic topic",
			req: contracts.Requirements{
				Topic:             "Retrieval augmented generation",
				ResearchQuestions: []string{"How does RAG improve accuracy"},
				Language:          "en",
			},
			limit:         10,
			expectMinimum: 2,
			expectChinese: false,
			expectEnglish: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			queries := GenerateQueries(tt.req, tt.limit)

			if len(queries) < tt.expectMinimum {
				t.Errorf("Expected at least %d queries, got %d", tt.expectMinimum, len(queries))
			}

			// Check that queries are scored
			for i, q := range queries {
				if q.Score <= 0 {
					t.Errorf("Query %d has invalid score: %f", i, q.Score)
				}
				if q.Text == "" {
					t.Errorf("Query %d has empty text", i)
				}

				// Check language-specific expectations
				if tt.expectChinese && containsEnglishSuffix(q.Text) {
					t.Errorf("Chinese query should not contain English suffix: %q", q.Text)
				}
			}

			// Queries should be sorted by score (descending)
			for i := 0; i < len(queries)-1; i++ {
				if queries[i].Score < queries[i+1].Score {
					t.Errorf("Queries not sorted by score: queries[%d].Score=%f < queries[%d].Score=%f",
						i, queries[i].Score, i+1, queries[i+1].Score)
				}
			}
		})
	}
}

func TestGenerateExpansionQueries(t *testing.T) {
	tests := []struct {
		name   string
		req    contracts.Requirements
		base   QueryWithMetadata
		needed int
		expect int
	}{
		{
			name: "Chinese expansion with preferences",
			req: contracts.Requirements{
				Topic:              "深度学习应用",
				ChapterPreferences: []string{"算法", "应用场景"},
				Language:           "zh-CN",
			},
			base: QueryWithMetadata{
				Query: Query{
					Text:     "深度学习应用 综述",
					Language: "zh-CN",
					Limit:    10,
				},
				Strategy: QueryStrategy{Language: "zh-CN", TopicType: TopicTypeGeneral},
			},
			needed: 3,
			expect: 3,
		},
		{
			name: "Chinese comparison expansion",
			req: contracts.Requirements{
				Topic:    "明朝君主哪个更厉害",
				Language: "zh-CN",
			},
			base: QueryWithMetadata{
				Query: Query{
					Text:     "明朝君主 研究",
					Language: "zh-CN",
					Limit:    10,
				},
				Strategy: QueryStrategy{Language: "zh-CN", TopicType: TopicTypeComparison},
			},
			needed: 2,
			expect: 2,
		},
		{
			name: "English expansion",
			req: contracts.Requirements{
				Topic:              "RAG systems",
				ChapterPreferences: []string{"architecture", "performance"},
				Language:           "en",
			},
			base: QueryWithMetadata{
				Query: Query{
					Text:     "RAG systems",
					Language: "en",
					Limit:    10,
				},
				Strategy: QueryStrategy{Language: "en", TopicType: TopicTypeGeneral},
			},
			needed: 2,
			expect: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			queries := GenerateExpansionQueries(tt.req, tt.base, tt.needed)

			if len(queries) > tt.expect {
				t.Errorf("Expected at most %d queries, got %d", tt.expect, len(queries))
			}

			// Should not duplicate base query
			for _, q := range queries {
				if q.Text == tt.base.Text {
					t.Errorf("Expansion query duplicates base query: %q", q.Text)
				}
			}
		})
	}
}

func TestScoreQuery(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		topic    string
		language string
		minScore float64
		maxScore float64
	}{
		{
			name:     "Good Chinese query",
			query:    "明朝君主 研究",
			topic:    "明朝君主哪个更厉害",
			language: "zh-CN",
			minScore: 0.8,
			maxScore: 1.0,
		},
		{
			name:     "Chinese query with English suffix - should be penalized",
			query:    "明朝君主 evidence literature review",
			topic:    "明朝君主哪个更厉害",
			language: "zh-CN",
			minScore: 0.0,
			maxScore: 0.5,
		},
		{
			name:     "Too short query",
			query:    "明",
			topic:    "明朝君主",
			language: "zh-CN",
			minScore: 0.0,
			maxScore: 0.6,
		},
		{
			name:     "English query with suffix - should be OK",
			query:    "RAG evidence literature review",
			topic:    "Retrieval augmented generation",
			language: "en",
			minScore: 0.5,
			maxScore: 1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := ScoreQuery(tt.query, tt.topic, tt.language)

			if score < tt.minScore || score > tt.maxScore {
				t.Errorf("ScoreQuery(%q, %q, %q) = %f, want between %f and %f",
					tt.query, tt.topic, tt.language, score, tt.minScore, tt.maxScore)
			}
		})
	}
}

func TestContainsEnglishSuffix(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		expected bool
	}{
		{"Has evidence", "topic evidence", true},
		{"Has literature", "topic literature", true},
		{"Has review", "topic review", true},
		{"No suffix", "主题 研究", false},
		{"Mixed but has suffix", "主题 literature", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := containsEnglishSuffix(tt.query)
			if result != tt.expected {
				t.Errorf("containsEnglishSuffix(%q) = %v, want %v", tt.query, result, tt.expected)
			}
		})
	}
}

func TestRemoveComparisonWords(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		expected string
	}{
		{
			name:     "Remove 哪个",
			text:     "明朝君主哪个更厉害",
			expected: "明朝君主更厉害",
		},
		{
			name:     "Remove vs",
			text:     "A vs B comparison",
			expected: "A B comparison",
		},
		{
			name:     "No comparison words",
			text:     "深度学习应用",
			expected: "深度学习应用",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := removeComparisonWords(tt.text)
			if result != tt.expected {
				t.Errorf("removeComparisonWords(%q) = %q, want %q", tt.text, result, tt.expected)
			}
		})
	}
}
