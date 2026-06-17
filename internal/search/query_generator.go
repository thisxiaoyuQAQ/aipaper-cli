package search

import (
	"strings"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
)

// QueryWithMetadata contains a query along with its generation metadata
type QueryWithMetadata struct {
	Query
	Strategy QueryStrategy
	Score    float64
	Reason   string
}

// GenerateQueries generates multiple queries based on the requirements.
// Returns queries ordered by score (highest first).
func GenerateQueries(req contracts.Requirements, limit int) []QueryWithMetadata {
	if limit <= 0 {
		limit = 10
	}

	strategy := SelectStrategy(req)

	var queries []QueryWithMetadata

	switch strategy.TopicType {
	case TopicTypeComparison:
		queries = buildComparisonQueries(req, strategy, limit)
	case TopicTypeGeneral:
		queries = buildGeneralQueries(req, strategy, limit)
	default:
		queries = buildDefaultQueries(req, strategy, limit)
	}

	// Score and filter queries
	scoredQueries := scoreAndFilterQueries(queries, req.Topic, strategy.Language)

	return scoredQueries
}

// GenerateExpansionQueries generates expansion queries based on the base query
func GenerateExpansionQueries(req contracts.Requirements, base QueryWithMetadata, needed int) []QueryWithMetadata {
	if needed <= 0 {
		return nil
	}

	strategy := base.Strategy
	limit := base.Limit
	if limit <= 0 {
		limit = 10
	}

	var queries []QueryWithMetadata

	switch strategy.TopicType {
	case TopicTypeComparison:
		queries = buildComparisonExpansionQueries(req, strategy, limit, needed)
	case TopicTypeGeneral:
		queries = buildGeneralExpansionQueries(req, strategy, limit, needed)
	default:
		queries = buildDefaultExpansionQueries(req, strategy, limit, needed)
	}

	// Score and filter
	scoredQueries := scoreAndFilterQueries(queries, req.Topic, strategy.Language)

	// Deduplicate against base query
	baseText := strings.ToLower(strings.TrimSpace(base.Text))
	var result []QueryWithMetadata
	for _, q := range scoredQueries {
		qText := strings.ToLower(strings.TrimSpace(q.Text))
		if qText != baseText {
			result = append(result, q)
			if len(result) >= needed {
				break
			}
		}
	}

	return result
}

// scoreAndFilterQueries scores all queries and filters out low-quality ones
func scoreAndFilterQueries(queries []QueryWithMetadata, topic string, language string) []QueryWithMetadata {
	var scored []QueryWithMetadata

	for _, q := range queries {
		score := ScoreQuery(q.Text, topic, language)
		q.Score = score

		// Filter out very low quality queries (score < 0.3)
		if score >= 0.3 {
			scored = append(scored, q)
		}
	}

	// Sort by score descending (simple bubble sort for small lists)
	for i := 0; i < len(scored)-1; i++ {
		for j := i + 1; j < len(scored); j++ {
			if scored[j].Score > scored[i].Score {
				scored[i], scored[j] = scored[j], scored[i]
			}
		}
	}

	return scored
}

// ScoreQuery evaluates the quality of a query string
func ScoreQuery(query string, topic string, language string) float64 {
	score := 1.0

	queryRunes := []rune(strings.TrimSpace(query))
	topicRunes := []rune(strings.TrimSpace(topic))

	// Penalty: too short
	if len(queryRunes) < 3 {
		score *= 0.5
	}

	// Penalty: too long (overly specific)
	if len(queryRunes) > 50 {
		score *= 0.8
	}

	// Chinese query: penalize English academic suffixes
	if language == "zh-CN" && containsEnglishSuffix(query) {
		score *= 0.3
	}

	// Reward: contains topic core keywords
	if len(topicRunes) > 0 && containsTopicKeywords(query, topic) {
		score *= 1.2
	}

	// Cap score at 1.0
	if score > 1.0 {
		score = 1.0
	}

	return score
}

// containsEnglishSuffix checks if the query contains English academic suffixes
func containsEnglishSuffix(query string) bool {
	lowerQuery := strings.ToLower(query)
	suffixes := []string{"evidence", "literature", "review", "systematic", "empirical", "study"}

	for _, suffix := range suffixes {
		if strings.Contains(lowerQuery, suffix) {
			return true
		}
	}

	return false
}

// containsTopicKeywords checks if query contains core keywords from topic
func containsTopicKeywords(query string, topic string) bool {
	queryLower := strings.ToLower(query)
	topicLower := strings.ToLower(topic)

	// Simple heuristic: check if at least 30% of topic characters appear in query
	topicRunes := []rune(topicLower)
	matchCount := 0

	for _, r := range topicRunes {
		if strings.ContainsRune(queryLower, r) {
			matchCount++
		}
	}

	ratio := float64(matchCount) / float64(len(topicRunes))
	return ratio >= 0.3
}
